package model

import "gorm.io/gorm"

type User struct {
	gorm.Model
	Username string `gorm:"unique;not null;size:50;comment:用户名"`
	Email    string `gorm:"unique;not null;size:100;comment:邮箱"`
	Password string `gorm:"not null;size:255;comment:密码(加密存储)"`
	Nickname string `gorm:"size:50;comment:昵称"`
	Avatar   string `gorm:"size:255;comment:头像URL"`
	Role     int    `gorm:"default:0;comment:角色(0=普通用户 1=管理员)"`
}
