package model

import "gorm.io/gorm"

type Category struct {
	gorm.Model
	Name        string `gorm:"unique;not null;size:50;comment:分类名称"`
	Description string `gorm:"size:200;comment:分类描述"`
}
