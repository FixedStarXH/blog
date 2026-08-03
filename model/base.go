package model

import (
	"time"

	"gorm.io/gorm"
)

// BaseModel 是所有模型的公共基础结构（替代 gorm.Model）
//
// 为什么不用 gorm.Model？
//
//	gorm.Model 的字段【没有 json tag】，JSON 序列化后输出的是
//	ID / CreatedAt / UpdatedAt / DeletedAt 这种大写开头，
//	而前端 JS 读取的是小驼峰 id / createdAt，
//	结果就是前端拿不到数据（之前审核报告 P-A 的问题）。
//
//	所以自定义一份"带 json tag"的版本，字段名和 gorm.Model 完全一样，
//	GORM 建表行为不受影响，只是输出 JSON 时变成小驼峰。
type BaseModel struct {
	ID        uint           `gorm:"primarykey" json:"id"` // 主键
	CreatedAt time.Time      `json:"createdAt"`            // 创建时间
	UpdatedAt time.Time      `json:"updatedAt"`            // 更新时间
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`       // 软删除标记；json:"-" 表示不输出给前端（外部不该知道删除状态）
}
