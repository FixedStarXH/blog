package scheduler

import (
	"log"
	"time"

	"blog-system/dao"

	"gorm.io/gorm"
)

// StartPublishScheduler 启动定时发布任务
// 原理：goroutine 后台跑，time.Ticker 每 30 秒"响"一次，响就扫描到期文章
func StartPublishScheduler(db *gorm.DB) {
	articleDAO := dao.NewArticleDAO()

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C { // ticker.C 是个 channel，每 30 秒收到一次"信号"
			n, err := articleDAO.PublishScheduled(db)
			if err != nil {
				log.Printf("[scheduler] 定时发布扫描失败: %v", err)
				continue
			}
			if n > 0 {
				log.Printf("[scheduler] 已定时发布 %d 篇文章", n)
			}
		}
	}()

	log.Println("[scheduler] 定时发布任务已启动（每 30 秒扫描一次）")
}
