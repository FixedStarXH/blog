package model

import "time"

// ArticleView 文章浏览记录（浏览量防刷的"明细表"）
//
// 设计原理（数据库设计文档 2.8）：
//   文章表有一个冗余的 ViewCount 总数，这里是每一次浏览的明细。
//   防刷：同一 IP 对同一篇文章，时间窗内只记一条明细。
//   查询：先查"这个 IP 今天有没有看过这篇文章"，有就不加数。
//   加数时：明细表和 ViewCount 必须在【同一个事务】里更新，保证一致。
type ArticleView struct {
	BaseModel

	ArticleID uint      `gorm:"not null;index;comment:文章ID" json:"articleId"` // 被浏览的文章
	IP        string    `gorm:"size:45;index;comment:访问者IP" json:"-"`        // IPv6 最长 45 字符；不输出给前端
	ViewedAt  time.Time `gorm:"index;comment:浏览时间" json:"viewedAt"`          // 配合 IP 做时间窗去重
}
