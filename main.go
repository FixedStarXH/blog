package main

import (
	"fmt"
	"log"

	"blog-system/config"
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

	scheduler.StartPublishScheduler(model.DB)

	r := gin.Default()

	router.Init(r)

	addr := fmt.Sprintf(":%d", config.AppConfig.Server.Port)
	fmt.Printf("服务启动在http://localhost%s\n", addr)
	r.Run(addr)
}
