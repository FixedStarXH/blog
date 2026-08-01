package model

// ArticleLike 文章点赞记录（去重）
//
// 设计原理（数据库设计文档 2.8）：
//   (article_id, ip) 组成【联合唯一索引】——数据库层面保证：
//   同一 IP 对同一篇文章最多一条点赞记录，想重复赞也会被 MySQL 拒绝。
//   UserID 可空：游客用 IP 去重，登录用户可绑定 UserID。
type ArticleLike struct {
	BaseModel

	ArticleID uint  `gorm:"not null;uniqueIndex:uk_like_article_ip;comment:文章ID" json:"articleId"`
	IP        string `gorm:"size:45;uniqueIndex:uk_like_article_ip;comment:点赞者IP" json:"-"`
	UserID    *uint `gorm:"index;comment:用户ID" json:"userId"`
}
