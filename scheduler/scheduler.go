package scheduler

import (
	"log"
	"time"

	"blog-system/cache"
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
				// 新文章发布 → 清空列表/热门/归档/计数缓存，让前台立刻可见
				cache.InvalidateArticleRelated(0)
			}
		}
	}()

	log.Println("[scheduler] 定时发布任务已启动（每 30 秒扫描一次）")
}

// StartViewFlushScheduler 浏览量刷库任务
// Redis 里攒的增量（view:delta:*）定时合并进 MySQL，避免每次都写库
func StartViewFlushScheduler(db *gorm.DB) {
	articleDAO := dao.NewArticleDAO()

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			cache.FlushViews(func(articleID uint, delta int64) error {
				return articleDAO.AddViewDelta(db, articleID, delta)
			})
		}
	}()

	log.Println("[scheduler] 浏览量刷库任务已启动（每 30 秒一次）")
}
