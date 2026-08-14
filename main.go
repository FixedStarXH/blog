package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"blog-system/cache"
	"blog-system/config"
	"blog-system/middleware"
	"blog-system/model"
	"blog-system/router"
	"blog-system/scheduler"

	"github.com/gin-gonic/gin"
)

func main() {
	if err := config.Init(); err != nil {
		log.Fatalf("初始化配置失败:%v", err)
	}

	if err := model.InitDB(); err != nil {
		log.Fatalf("初始化数据库失败:%v", err)
	}
	fmt.Println("数据库连接成功，表已自动创建")

	// 初始化 Redis 缓存层（失败自动降级为"直查数据库"，不影响主流程）
	cache.Init(config.AppConfig.Redis.Addr(), config.AppConfig.Redis.Password, config.AppConfig.Redis.DB)

	// 初始化结构化日志（slog：控制台 + logs/app.log 双输出）
	if err := middleware.InitLogger(); err != nil {
		log.Fatalf("初始化日志失败:%v", err)
	}

	scheduler.StartPublishScheduler(model.DB)
	scheduler.StartViewFlushScheduler(model.DB)

	// 生产模式：release 关闭 gin 的 debug 控制台日志与调试开销（高并发性能关键项）
	gin.SetMode(gin.ReleaseMode)

	// gin.New() 不带默认 logger（我们用 slog），只留 Recovery 兜底防 panic
	r := gin.New()
	r.Use(gin.Recovery())

	// 安全：只信任回环 + Docker 内网网段的 X-Forwarded-For。
	// gin 默认信任所有代理，直连部署时攻击者可伪造 XFF 绕过"按 IP 限流/浏览量去重/防爆破"，
	// 现在 RemoteIP 不在可信列表 → ClientIP() 回退为真实客户端 IP（限流按真实 IP 生效）。
	// Docker 场景：nginx 容器在 172.16.0.0/12 内，反代下 XFF 仍被信任。
	if err := r.SetTrustedProxies([]string{"127.0.0.1", "::1", "172.16.0.0/12", "10.0.0.0/8"}); err != nil {
		log.Fatalf("设置可信代理失败:%v", err)
	}

	router.Init(r)

	addr := fmt.Sprintf(":%d", config.AppConfig.Server.Port)

	// 优雅退出：把 ListenAndServe 放到 goroutine，主协程等信号。
	// 收到 Ctrl+C(SIGINT) 或 docker stop(SIGTERM) 后：
	//   ① 停止接收新连接  ② 等待正在处理的请求最多 10 秒  ③ 退出
	// 否则强制 kill 会导致正在写的请求被中断、数据不一致。
	srv := &http.Server{Addr: addr, Handler: r}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("服务启动失败:%v", err)
		}
	}()
	fmt.Printf("服务启动在http://localhost%s\n", addr)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	fmt.Println("收到退出信号，正在优雅关闭…")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("优雅关闭失败:%v", err)
	}
	fmt.Println("服务已安全退出")
}
