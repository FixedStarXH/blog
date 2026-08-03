package model

// Image 图片图库（发布文章时上传的图片）
//
// 需求：发布文章可上传图片（Day04 文件上传知识）
// 文件真实存在 uploads/ 目录，数据库只记"元信息"（路径/大小/扩展名）
type Image struct {
	BaseModel

	Name       string `gorm:"not null;size:255;comment:原始文件名" json:"name"`
	URL        string `gorm:"not null;size:255;comment:存储路径" json:"url"`      // 如 /uploads/2026/08/xxxx.jpg
	Size       int64  `gorm:"not null;comment:大小字节" json:"size"`              // int64 防止超大文件溢出
	Ext        string `gorm:"not null;size:10;comment:扩展名" json:"ext"`        // 如 jpg/png（白名单校验）
	UploaderID uint   `gorm:"not null;index;comment:上传者ID" json:"uploaderId"` // 谁传的，便于按人管理

	Uploader User `gorm:"foreignKey:UploaderID" json:"uploader,omitempty"` // 关联上传者
}
