package model

// Column 专栏模型（系统特色：作者的"个人空间"）
//
// 一个作者可开多个专栏（AuthorID），一个专栏下可挂多篇文章（Article.ColumnID 可空）
// 归属规则：创建/编辑归作者自己（接口 /api/my/columns）；
//          管理员后台可关闭/删除全部专栏
type Column struct {
	BaseModel

	Name        string `gorm:"not null;size:100;comment:专栏名" json:"name"` // 必填
	Description string `gorm:"size:255;comment:专栏简介" json:"description"`  // 可空
	Cover       string `gorm:"size:255;comment:封面图URL" json:"cover"`      // 可空
	AuthorID    uint   `gorm:"not null;index;comment:作者ID" json:"authorId"` // 归属作者 FK
	Status      int    `gorm:"default:1;comment:1正常0关闭" json:"status"`    // 默认 ColumnStatusActive=1

	Author User `gorm:"foreignKey:AuthorID" json:"author"` // 关联作者
}
