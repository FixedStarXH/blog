package model

// AuditLog 后台操作审计日志（记录管理员/编辑的关键写操作）
//
// 设计要点：
//   - 谁（OperatorID/Username/Role）+ 做了什么（Action）+ 对谁做的（TargetType/TargetID）
//   - Detail 存请求体摘要（截断），方便追溯"改成了什么"
//   - 写操作全量记录（含失败），登录尝试单独记录（防暴力破解留痕）
//   - 只允许追加 + 管理员可清空，不允许修改/删除单条（审计记录不可篡改）
type AuditLog struct {
	BaseModel

	OperatorID uint   `gorm:"index;comment:操作人ID" json:"operatorId"`
	Username   string `gorm:"size:50;index;comment:操作人账号" json:"username"`
	Role       int    `gorm:"comment:操作人角色" json:"role"`
	Action     string `gorm:"size:100;index;comment:操作动作(中文描述)" json:"action"`
	Method     string `gorm:"size:10;comment:HTTP方法" json:"method"`
	Path       string `gorm:"size:200;comment:请求路径" json:"path"`
	TargetType string `gorm:"size:50;comment:目标类型(article/comment/user...)" json:"targetType"`
	TargetID   uint   `gorm:"index;comment:目标ID" json:"targetId"`
	Detail     string `gorm:"size:500;comment:详情(请求体摘要)" json:"detail"`
	IP         string `gorm:"size:45;comment:来源IP" json:"ip"`
	Status     int    `gorm:"comment:结果(0失败 1成功)" json:"status"`
}
