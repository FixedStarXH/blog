package model

// Setting 站点配置（KV 键值对）
//
// 存什么：站点标题、描述、社交链接、每日一言的名言池、累计访问量…
// 为什么不用 BaseModel？
//   - 这是 KV 结构，k 就是主键，不需要自增 id
//   - 配置需要"覆盖写"，不需要软删除、不需要记录创建时间
//   所以它是最简单的"裸表"，GORM 一样能建表
type Setting struct {
	K string `gorm:"primaryKey;size:100;comment:配置键" json:"k"` // 如 site_title
	V string `gorm:"type:text;comment:配置值" json:"v"`           // 值
}
