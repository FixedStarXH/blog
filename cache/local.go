// 本地内存缓存层（L1）：多级缓存的第一跳
//
// 架构：L1 本地内存（本文件）→ L2 Redis（cache.go）→ DB（singleflight 防击穿）
//   - 为什么需要 L1？Redis 也要走网络（~0.1ms 级），本地内存是纯进程内读（~微秒级），
//     热点文章在单实例部署下命中 L1 完全不过网络，详情接口吞吐大幅提升。
//   - 为什么用 freecache？零 GC 压力（Entry 不产生堆对象）、内置 LRU 淘汰 + 过期 + 线程安全，
//     比自己手写 map+mutex+定时器更稳，代码量也少。
//   - 一致性怎么保证？写操作（InvalidateArticleRelated）同时删 L1 和 L2，单实例部署下立即生效；
//     多实例部署时需把"删 L1"升级为 Redis Pub/Sub 广播（本教学版以单实例为准）。
package cache

import (
	"encoding/json"
	"log"
	"time"

	"github.com/coocood/freecache"
)

// localCacheSize 本地缓存容量：32MB，文章详情（含全文）可容纳数千篇，个人博客足够
const localCacheSize = 32 * 1024 * 1024

var local *freecache.Cache

func init() {
	// NewCache 会预分配全部内存，不会失败（OOM 由系统兜底，个人博客规模可控）
	local = freecache.NewCache(localCacheSize)
	log.Println("[cache] 本地内存缓存层(L1)已启用 32MB")
}

// GetLocal 读本地缓存到 target；命中返回 true（未命中/反序列化失败返回 false）
// 与 cache.Get 语义一致，只是存储介质不同，业务代码无需关心差异
func GetLocal(key string, target any) bool {
	if local == nil {
		return false
	}
	data, err := local.Get([]byte(key))
	if err != nil {
		return false // freecache.ErrNotFound 或其它错误：都当未命中
	}
	if err := json.Unmarshal(data, target); err != nil {
		return false
	}
	return true
}

// SetLocal 写本地缓存。ttl<=0 表示不过期（freecache 过期精度为秒）
func SetLocal(key string, value any, ttl time.Duration) {
	if local == nil {
		return
	}
	data, err := json.Marshal(value)
	if err != nil {
		return
	}
	sec := int(ttl.Seconds())
	if err := local.Set([]byte(key), data, sec); err != nil {
		// 仅当 entry 超过整个缓存容量时才会失败（LRU 淘汰会腾空间，正常不会触发）
		log.Printf("[cache] 本地缓存写入失败 key=%s: %v", key, err)
	}
}

// DelLocal 删除本地缓存（写操作后调用，与 Redis 的 Del 配套保证 L1/L2 同步失效）
func DelLocal(keys ...string) {
	if local == nil {
		return
	}
	for _, k := range keys {
		local.Del([]byte(k))
	}
}
