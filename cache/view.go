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
	"log"
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
func FlushViews(updateDB func(articleID uint, delta int64) error) {
	if !Enabled || Client == nil {
		return
	}
	// ① 取出所有待刷文章 ID（dirty 集合，AddView 时 SADD 加入）
	ids, err := Client.SMembers(Ctx, viewDirtySet).Result()
	if err != nil || len(ids) == 0 {
		return
	}

	// ② 逐篇结算：GET 读增量 → 写库 → DEL 清增量
	//    （用 GET+DEL 而非 GETDEL，兼容 Redis < 6.2；刷库成功后清空，失败保留 dirty 重试）
	for _, idStr := range ids {
		articleID, _ := strconv.ParseUint(idStr, 10, 64)
		if articleID == 0 {
			Client.SRem(Ctx, viewDirtySet, idStr)
			continue
		}
		delta, err := Client.Get(Ctx, viewDeltaKey+idStr).Int64()
		if err != nil || delta <= 0 {
			Client.SRem(Ctx, viewDirtySet, idStr)
			continue
		}
		// ③ 写回 MySQL（失败则保留 dirty，下次 30 秒后再试）
		if err := updateDB(uint(articleID), delta); err != nil {
			log.Printf("[cache] 浏览量刷库失败 article=%s: %v", idStr, err)
			continue
		}
		// ④ 刷库成功：清增量 + 移除脏标记
		Client.Del(Ctx, viewDeltaKey+idStr)
		Client.SRem(Ctx, viewDirtySet, idStr)
	}
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
