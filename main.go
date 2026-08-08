package main

import (
	"fmt"
	"log"

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

	// gin.New() 不带默认 logger（我们用 slog），只留 Recovery 兜底防 panic
	r := gin.New()
	r.Use(gin.Recovery())

	router.Init(r)

	addr := fmt.Sprintf(":%d", config.AppConfig.Server.Port)
	fmt.Printf("服务启动在http://localhost%s\n", addr)
	r.Run(addr)
}
