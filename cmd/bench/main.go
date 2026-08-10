// 文章详情接口并发压测工具
//
// 用法：
//
//	go run ./cmd/bench                 # 200 并发 GET /api/articles/1（每个并发发 1 次）
//	go run ./cmd/bench -n 500 -id 2    # 500 并发压 id=2 的文章
//	go run ./cmd/bench -n 200 -total 50 # 每个并发连续发 50 次（观察缓存命中后的表现）
//	go run ./cmd/bench -flush          # 压测前删除该文章详情缓存（模拟缓存穿透/击穿场景）
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

func main() {
	url := flag.String("url", "http://127.0.0.1:8080", "服务地址")
	concurrency := flag.Int("n", 200, "并发数")
	articleID := flag.Int("id", 1, "文章 ID")
	total := flag.Int("total", 1, "每个并发发送的请求次数")
	flush := flag.Bool("flush", false, "压测前删除该文章详情缓存（模拟缓存穿透）")
	xff := flag.Bool("xff", false, "模拟多 IP（每个并发用独立 X-Forwarded-For），绕过单 IP 限流测真实性能")
	flag.Parse()

	if *flush {
		rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379", DB: 0})
		key := fmt.Sprintf("article:%d", *articleID)
		rdb.Del(context.Background(), key)
		fmt.Printf("已删除缓存 %s（模拟缓存穿透/击穿场景）\n", key)
	}

	reqPerWorker := *total
	target := fmt.Sprintf("%s/api/articles/%d", *url, *articleID)
	fmt.Printf("压测目标: GET %s\n并发: %d  每个并发 %d 次 = 总请求 %d\n\n",
		target, *concurrency, reqPerWorker, *concurrency*reqPerWorker)

	var (
		success atomic.Int64
		failed  atomic.Int64
		start   = time.Now()
		wg      sync.WaitGroup
		mu      sync.Mutex
		latency []time.Duration
	)
	client := &http.Client{Timeout: 10 * time.Second}

	worker := func(workerIndex int) {
		defer wg.Done()
		for i := 0; i < reqPerWorker; i++ {
			begin := time.Now()

			var resp *http.Response
			var err error
			if *xff {
				// 模拟多 IP：每个并发用独立 X-Forwarded-For，绕过单 IP 限流测真实性能
				req, _ := http.NewRequest(http.MethodGet, target, nil)
				ip := fmt.Sprintf("10.%d.%d.%d", (workerIndex>>16)&0xFF, (workerIndex>>8)&0xFF, workerIndex&0xFF)
				req.Header.Set("X-Forwarded-For", ip)
				resp, err = client.Do(req)
			} else {
				resp, err = client.Get(target)
			}

			cost := time.Since(begin)
			mu.Lock()
			latency = append(latency, cost)
			mu.Unlock()
			if err == nil {
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				if resp.StatusCode == 200 {
					success.Add(1)
				} else {
					failed.Add(1)
				}
			} else {
				failed.Add(1)
			}
		}
	}

	for i := 0; i < *concurrency; i++ {
		wg.Add(1)
		go worker(i)
	}
	wg.Wait()
	elapsed := time.Since(start)

	var totalCost time.Duration
	for _, d := range latency {
		totalCost += d
	}
	avg := time.Duration(0)
	if len(latency) > 0 {
		avg = totalCost / time.Duration(len(latency))
	}
	requests := success.Load() + failed.Load()
	qps := float64(requests) / elapsed.Seconds()

	fmt.Printf("总请求: %d   成功: %d   失败: %d\n", requests, success.Load(), failed.Load())
	fmt.Printf("总耗时: %v\n平均延迟: %v\nQPS: %.0f req/s\n",
		elapsed.Round(time.Millisecond), avg.Round(time.Millisecond), qps)
}
