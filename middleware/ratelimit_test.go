package middleware

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTokenBucketBurst(t *testing.T) {
	// 桶容量=10：一上来允许 10 个突发请求全过
	b := newTokenBucket(5, 10)
	for i := 0; i < 10; i++ {
		if !b.allow() {
			t.Fatalf("第 %d 个请求应放行（burst=10）", i+1)
		}
	}
	// 第 11 个：桶空且没到补充时间 → 拒绝
	if b.allow() {
		t.Fatal("桶空后应立即拒绝")
	}
}

func TestTokenBucketRefill(t *testing.T) {
	// rps=1000（每秒补 1000 个）、burst=2
	b := newTokenBucket(1000, 2)
	// 用光 2 个令牌
	if !b.allow() || !b.allow() {
		t.Fatal("前两个请求应放行")
	}
	if b.allow() {
		t.Fatal("第 3 个应被拒（桶空）")
	}
	// 睡 3 毫秒：1000/秒 * 0.003 秒 = 补 3 个令牌
	time.Sleep(3 * time.Millisecond)
	if !b.allow() {
		t.Fatal("补充后应恢复放行")
	}
}

func TestTokenBucketNoOverflow(t *testing.T) {
	// 长期不请求，令牌不能超过桶容量（不能攒无限多）
	b := newTokenBucket(1000, 2)
	time.Sleep(50 * time.Millisecond) // 按速率应补 50 个，但桶只有 2
	// 最多只能连续放行 2 个
	if !b.allow() || !b.allow() {
		t.Fatal("应放行 2 个")
	}
	if b.allow() {
		t.Fatal("桶容量封顶：最多 2 个")
	}
}

func TestTokenBucketConcurrent(t *testing.T) {
	// 并发安全：50 个 goroutine 同时抢令牌，最终放行数 = 桶容量 5
	b := newTokenBucket(1000, 5)
	var pass int32
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if b.allow() {
				// 注意：不能写 pass++（并发写是数据竞争），必须用原子累加
				atomic.AddInt32(&pass, 1)
			}
		}()
	}
	wg.Wait() // 等 50 个 goroutine 全部结束
	if pass != 5 {
		t.Fatalf("并发下应恰好放行 5 个，实际 %d", pass)
	}
}
