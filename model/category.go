package model

// Category 分类模型
//
// 文章必须属于一个分类（Article.CategoryID not null）
// Sort 控制后台分类列表的展示顺序，越小越靠前
type Category struct {
	BaseModel

	Name        string `gorm:"unique;not null;size:50;comment:分类名" json:"name"` // 唯一
	Description string `gorm:"size:200;comment:分类描述" json:"description"`        // 可空
	Sort        int    `gorm:"default:0;comment:排序小在前" json:"sort"`             // 排序值

	// 反向关联：该分类下的文章（分类列表接口统计 articleCount 用）
	// json:"-" 不输出文章列表，避免和 Article.Category 互相嵌套导致 JSON 循环
	ArticleCount int64     `gorm:"-" json:"articleCount"`
	Articles     []Article `gorm:"foreignKey:CategoryID" json:"-"`
}
