package model

// Tag 标签模型
//
// 文章和标签是【多对多】关系，通过中间表 article_tags 关联
// （Day10 笔记：many2many 需要中间表）
// 一个标签可挂多篇文章，一篇文章可有多个标签
type Tag struct {
	BaseModel

	Name string `gorm:"unique;not null;size:50;comment:标签名" json:"name"` // 标签名全局唯一

	// 反向关联：这个标签下的文章（按标签查文章时配合中间表过滤）
	// json:"-" 不在标签对象里输出文章列表，避免数据冗余
	ArticleCount int64     `gorm:"-" json:"articleCount"`
	Articles     []Article `gorm:"many2many:article_tags" json:"-"`
}
