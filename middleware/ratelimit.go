package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// tokenBucket 手写令牌桶（教学版，零依赖；生产可用 golang.org/x/time/rate 替代）
type tokenBucket struct {
	mu       sync.Mutex    // 并发安全：多个请求同时进来要加锁
	tokens   float64       // 当前桶里的令牌数
	max      float64       // 桶容量（burst）
	refill   float64       // 每秒补充的令牌数（rps）
	lastTime time.Time     // 上次取令牌的时间（用于结算补充量）
}

func newTokenBucket(rps, burst int) *tokenBucket {
	return &tokenBucket{
		tokens:   float64(burst), // 初始装满，允许一上来就有突发
		max:      float64(burst),
		refill:   float64(rps),
		lastTime: time.Now(),
	}
}

// allow 取一个令牌：有 → true（放行）；没有 → false（拒绝）
func (b *tokenBucket) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	// ① 结算：距离上次请求过去了多久，就补充多少令牌
	b.tokens += now.Sub(b.lastTime).Seconds() * b.refill
	b.lastTime = now
	// ② 桶满了就丢弃多余的（令牌不能攒到无限多）
	if b.tokens > b.max {
		b.tokens = b.max
	}
	// ③ 取令牌
	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// RateLimitByIP 按 IP 维度限流：每个客户端 IP 一个独立的令牌桶
// 相比全局单桶 RateLimit：单个 IP 刷爆自己的配额只会让自己被限，
// 不会把所有人的配额耗尽（避免一个攻击者拖垮全站）
//
// 注意：IP 取自 gin 的 ClientIP()（优先信任 X-Forwarded-For，适合 Nginx 反代部署）。
// 直连部署时客户端可伪造该头绕过 —— 个人博客可接受；如要更严格请改用 c.RemoteIP()
func RateLimitByIP(rps, burst int) gin.HandlerFunc {
	var mu sync.Mutex
	buckets := make(map[string]*tokenBucket)

	// 定期清理长时间不活跃的桶，防止 map 无限增长（内存泄漏）
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			now := time.Now()
			mu.Lock()
			for ip, b := range buckets {
				b.mu.Lock()
				idle := now.Sub(b.lastTime)
				b.mu.Unlock()
				if idle > 30*time.Minute {
					delete(buckets, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		ip := c.ClientIP()
		// 取桶：map 并发读写不安全，先加锁取/建，取到桶后再用桶自己的锁放行
		mu.Lock()
		b, ok := buckets[ip]
		if !ok {
			b = newTokenBucket(rps, burst)
			buckets[ip] = b
		}
		mu.Unlock()

		if !b.allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"code":    429,
				"message": "请求过于频繁，请稍后再试",
				"data":    nil,
			})
			return
		}
		c.Next()
	}
}
