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

// RateLimit 生成限流中间件
// rps: 每秒补充令牌数；burst: 桶容量（瞬时突发上限）
// 令牌用完 → 429 Too Many Requests
func RateLimit(rps, burst int) gin.HandlerFunc {
	limiter := newTokenBucket(rps, burst)
	return func(c *gin.Context) {
		if !limiter.allow() {
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
