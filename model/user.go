package model

// User 用户模型（多角色）
//
// 角色体系：1=普通用户(RoleUser) 2=编辑(RoleEditor) 3=管理员(RoleAdmin)
//
//	0=游客只是"未登录"的概念，不存数据库（见 const.go）
//
// 字段默认值都写常量名（RoleUser / UserStatusActive），不写裸数字
type User struct {
	BaseModel

	Username string `gorm:"unique;not null;size:50;comment:用户名" json:"username"` // 唯一，登录凭证之一
	Email    string `gorm:"unique;not null;size:100;comment:邮箱" json:"email"`    // 唯一，登录凭证之一
	Password string `gorm:"not null;size:255;comment:密码BCrypt哈希" json:"-"`       // json:"-" = 序列化时跳过，密码永不返回给前端！
	Nickname string `gorm:"size:50;comment:昵称" json:"nickname"`                  // 展示名，可空，默认显示用户名
	Avatar   string `gorm:"size:255;comment:头像URL" json:"avatar"`                // 头像图片地址
	Role     int    `gorm:"default:1;index;comment:角色1普通2编辑3管理员" json:"role"`    // 默认 RoleUser=1；加索引便于按角色筛选
	Status   int    `gorm:"default:1;index;comment:状态1正常0禁用" json:"status"`      // 默认 UserStatusActive=1；禁用=拉黑不能登录
	Bio      string `gorm:"size:255;comment:个人简介" json:"bio"`                    // 作者页展示，可空
}
