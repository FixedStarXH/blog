package model

// Link 友情链接（站点设置里的友链管理）
type Link struct {
	BaseModel

	Name        string `gorm:"not null;size:50;comment:友链名" json:"name"`
	URL         string `gorm:"not null;size:255;comment:链接地址" json:"url"`
	Description string `gorm:"size:200;comment:描述" json:"description"`
	Sort        int    `gorm:"default:0;comment:排序" json:"sort"`
	Enabled     bool   `gorm:"default:true;comment:是否启用" json:"enabled"` // 关闭后前台不展示
}
