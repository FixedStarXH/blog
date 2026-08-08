// 文章浏览量热数据 —— Redis 增量计数 + 定时刷回 MySQL
//
// 为什么不用数据库自增？
//
//	每次浏览都 UPDATE view_count = view_count + 1 会给 MySQL 造成写压力；
//	高并发的"计数型"数据（浏览量/点赞数）经典做法是先进 Redis 累加，
//	定时批量刷回数据库，实现"读写分离、异步落库"。
//
// 数据模型（Redis 中四把钥匙）：
//
//	view:count:{id}   累计浏览量（= DB 值 + 未刷回的增量），实时给前端展示
//	view:delta:{id}   自上次刷库以来的新增量（刷库后清零）
//	view:dirty        待刷文章 ID 集合（SADD 标记 / SPOP 清空）
//	view:dedup:{id}:{date}:{ip}  当日该 IP 是否已浏览（防重复计数，等价原 SQL 查询）
package cache

import (
	"fmt"
	"log"
	"math/rand/v2"
	"strconv"
	"time"
)

// 视图计数相关的 key 前缀
const (
	viewCountKey = "view:count:" // view:count:{id}     累计浏览量
	viewDeltaKey = "view:delta:" // view:delta:{id}     待刷增量
	viewDedupKey = "view:dedup:" // view:dedup:{id}:{date}:{ip}  当日 IP 去重
	viewDirtySet = "view:dirty"  // 待刷文章 ID 集合
)

// GetViewCount 获取文章当前浏览量（读 Redis 缓存；未命中返回 -1 由调用方回源 DB）
func GetViewCount(articleID uint) int64 {
	if !Enabled || Client == nil {
		return -1
	}
	if v, err := Client.Get(Ctx, viewCountKey+uintToString(articleID)).Int64(); err == nil {
		return v
	}
	return -1
}

// EnsureViewCount 初始化累计计数：key 不存在时用 DB 当前值打底
// getDB 返回数据库里的真实浏览量；SetNX 保证只写一次，不覆盖已有计数
func EnsureViewCount(articleID uint, getDB func() int64) {
	if !Enabled || Client == nil {
		return
	}
	v := getDB()
	if v < 0 {
		return
	}
	Client.SetNX(Ctx, viewCountKey+uintToString(articleID), v, 0)
}

// AddView 浏览量 +1：返回 (当前累计值, 是否正常)
// 流程：IP 当日已访问 → 不重复计数；否则 INCR 计数 + 记录待刷增量
func AddView(articleID uint, ip string) (int64, bool) {
	if !Enabled || Client == nil {
		return 0, false
	}

	// ① 当日 IP 去重：SETNX 成功 = 首次访问；失败 = 已访问过，不重复计数
	ok, err := Client.SetNX(Ctx, dedupKey(articleID, ip), "1", remainingDay()).Result()
	if err != nil {
		return 0, false
	}
	if !ok {
		// 已访问过：直接返回当前计数（不 +1）
		if v, e := Client.Get(Ctx, viewCountKey+uintToString(articleID)).Int64(); e == nil {
			return v, true
		}
		return 0, true
	}

	// ② 首次访问：INCR 累计计数 + 增量，标记待刷（管道一次往返，减少 RTT）
	pipe := Client.TxPipeline()
	countCmd := pipe.Incr(Ctx, viewCountKey+uintToString(articleID))
	pipe.Incr(Ctx, viewDeltaKey+uintToString(articleID))
	pipe.SAdd(Ctx, viewDirtySet, uintToString(articleID))
	if _, err := pipe.Exec(Ctx); err != nil {
		return 0, false
	}
	if v, e := countCmd.Result(); e == nil {
		return v, true
	}
	return 0, false
}

// FlushViews 定时刷库：把 Redis 增量合并进 MySQL，并同步累计值
// 由 scheduler 每 30 秒调用；updateDB 是注入的 DAO 更新函数
//
// 并发安全设计（修复两个竞态）：
//  1. 【多实例重复累加】两个实例同时执行都会读到同一批 dirty + 同一个 delta，
//     各写一次库 → 浏览量翻倍。解决：Redis 分布式锁（SET NX EX），
//     同一时刻只有一个实例能刷库。
//  2. 【取增量与删增量之间的丢数】旧写法 GET → 写库 → DEL 三步非原子，
//     GET 之后新来的浏览 INCR 增量会被随后的 DEL 一并删除，浏览量永久丢失。
//     解决：Lua 脚本把"取增量 + 删增量"合成一个原子操作（兼容 Redis 2.6+）。
func FlushViews(updateDB func(articleID uint, delta int64) error) {
	if !Enabled || Client == nil {
		return
	}

	// ① 分布式锁：多实例部署时只有一个实例能刷库
	token, ok := acquireViewFlushLock()
	if !ok {
		return // 其他实例正在刷库，本轮直接跳过（下个周期再来）
	}
	defer releaseViewFlushLock(token)

	// ② 取出所有待刷文章 ID（dirty 集合，AddView 时 SADD 加入）
	ids, err := Client.SMembers(Ctx, viewDirtySet).Result()
	if err != nil || len(ids) == 0 {
		return
	}

	// ③ 逐篇结算：原子取出增量 → 写库 → 成功后再清 dirty
	for _, idStr := range ids {
		articleID, _ := strconv.ParseUint(idStr, 10, 64)
		if articleID == 0 {
			Client.SRem(Ctx, viewDirtySet, idStr)
			continue
		}
		delta, err := getAndDelDelta(idStr)
		if err != nil || delta <= 0 {
			Client.SRem(Ctx, viewDirtySet, idStr)
			continue
		}
		// ④ 写回 MySQL（失败则把增量回补，等下次 30 秒后再试）
		if err := updateDB(uint(articleID), delta); err != nil {
			log.Printf("[cache] 浏览量刷库失败 article=%s: %v", idStr, err)
			Client.IncrBy(Ctx, viewDeltaKey+idStr, delta) // 回补增量，防止丢失
			continue
		}
		// ⑤ 刷库成功：若期间又来了新浏览（delta key 重新存在），
		//    保留 dirty（AddView 已 SADD），等下一轮继续刷；否则清理 dirty
		if n, _ := Client.Exists(Ctx, viewDeltaKey+idStr).Result(); n == 0 {
			Client.SRem(Ctx, viewDirtySet, idStr)
		}
	}
}

// getAndDelDelta 原子取出并删除某篇文章的待刷增量（Lua 脚本实现）
// 等价 Redis 6.2+ 的 GETDEL，但 Lua 兼容所有版本；取不到返回 0
var getAndDelDeltaScript = `
local delta = redis.call('GET', KEYS[1])
if delta then
    redis.call('DEL', KEYS[1])
end
return delta`

func getAndDelDelta(idStr string) (int64, error) {
	v, err := Client.Eval(Ctx, getAndDelDeltaScript, []string{viewDeltaKey + idStr}).Result()
	if err != nil || v == nil {
		return 0, err
	}
	switch n := v.(type) {
	case int64:
		return n, nil
	case string:
		var i int64
		fmt.Sscanf(n, "%d", &i)
		return i, nil
	}
	return 0, nil
}

// ------------------------------------------------------------------
// 浏览量刷库的分布式锁（多实例防重）
// ------------------------------------------------------------------

const (
	viewFlushLockKey = "view:flush:lock" // 刷库锁的 key
	viewFlushLockTTL = 30 * time.Second  // 锁过期时间：刷库是毫秒级操作，30 秒足够且安全
	viewFlushLockLua = `
if redis.call('GET', KEYS[1]) == ARGV[1] then
    return redis.call('DEL', KEYS[1])
else
    return 0
end` // 释放锁必须"先比 token 再 DEL"
)

// acquireViewFlushLock 获取刷库锁：SET NX EX 原子性保证同一时刻只有一个实例拿到锁。
// 返回的 token 是持有者身份标识，释放时必须带着它（防止误删别人的锁）
func acquireViewFlushLock() (token string, ok bool) {
	token = fmt.Sprintf("%d-%d", time.Now().UnixNano(), rand.Int64())
	ok, err := Client.SetNX(Ctx, viewFlushLockKey, token, viewFlushLockTTL).Result()
	if err != nil {
		return "", false
	}
	return token, ok
}

// releaseViewFlushLock 释放锁：Lua 先比较 token 再删除。
// 为什么要比较？锁若已因超时过期、被别的实例重新获取，直接 DEL 会删掉别人的锁
// （经典误删问题），比较 token 能确保只删自己的锁
func releaseViewFlushLock(token string) {
	if token == "" {
		return
	}
	Client.Eval(Ctx, viewFlushLockLua, []string{viewFlushLockKey}, token)
}

// dedupKey 当日 IP 去重 key
func dedupKey(articleID uint, ip string) string {
	return viewDedupKey + uintToString(articleID) + ":" + time.Now().Format("20060102") + ":" + ip
}

// remainingDay 当天剩余时长（去重 key 的 TTL：到今晚 24 点自动过期）
func remainingDay() time.Duration {
	now := time.Now()
	midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	return midnight.Sub(now)
}
