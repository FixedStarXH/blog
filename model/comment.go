package model

import "gorm.io/gorm"

type Comment struct {
	gorm.Model
	Content   string  `gorm:"type:text;not null;comment:评论内容"`
	Status    int     `gorm:"default:0;index;comment:状态(0=待审核 1=已通过 2=已驳回)"`
	ArticleID uint    `gorm:"not null;index;comment:所属文章ID"`
	UserID    uint    `gorm:"not null;index;comment:评论用户ID"`
	ParentID  *uint   `gorm:"index;comment:父评论ID(回复评论)"`
	Article   Article `gorm:"foreignKey:ArticleID"` // 关联文章
	User      User    `gorm:"foreignKey:UserID"`    // 关联用户
}
