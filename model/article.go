package model

import "gorm.io/gorm"

type Article struct {
	gorm.Model

	Title      string   `gorm:"not null;size:200;index;comment:文章标题"`
	Content    string   `gorm:"type:longtext;not null;comment:文章内容(支持HTML)"`
	Summary    string   `gorm:"size:500;comment:文章摘要"`
	CoverImage string   `gorm:"size:255;comment:封面图片URL"`
	ViewCount  int      `gorm:"default:0;comment:阅读量"`
	Status     int      `gorm:"default:0;index;comment:状态(0=草稿 1=已发布)"`
	AuthorID   uint     `gorm:"not null;index;comment:作者ID"`
	CategoryID uint     `gorm:"not null;index;comment:分类ID"`
	Author     User     `gorm:"foreignKey:AuthorID"`   // 关联作者
	Category   Category `gorm:"foreignKey:CategoryID"` // 关联分类
}
