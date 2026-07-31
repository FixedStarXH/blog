package model

import (
	"fmt"
	"time"

	"blog-system/config"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func InitDB() error {
	var err error

	DB, err = gorm.Open(mysql.Open(config.AppConfig.MySQL.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return fmt.Errorf("连接数据库失败:%w", err)
	}
	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("获取sql.DB失败:%w", err)
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if err := DB.AutoMigrate(
		&User{},
		&Category{},
		&Article{},
		&Comment{},
	); err != nil {
		return fmt.Errorf("自动迁移失败:%w", err)
	}

	initTestData()

	return nil

}

func initTestData() {
	var count int64
	DB.Model(&User{}).Count(&count)
	if count > 0 {
		return
	}
	// 1. 创建测试用户
	admin := User{
		Username: "admin",
		Email:    "admin@example.com",
		Password: "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy", // 明文: 123456
		Nickname: "管理员",
		Role:     1,
	}
	DB.Create(&admin)

	// 2. 创建测试分类
	categories := []Category{
		{Name: "Go语言", Description: "Go语言相关的技术文章"},
		{Name: "数据库", Description: "MySQL、Redis等数据库文章"},
		{Name: "前端", Description: "HTML、CSS、JavaScript文章"},
	}
	for i := range categories {
		DB.Create(&categories[i])
	}

	// 3. 创建测试文章
	articles := []Article{
		{Title: "Go语言入门指南", Summary: "Go语言的基本语法和特性介绍", Content: "这是Go语言入门文章的内容...", Status: 1, AuthorID: admin.ID, CategoryID: categories[0].ID},
		{Title: "GORM入门教程", Summary: "GORM的基本使用", Content: "这是GORM教程的内容...", Status: 1, AuthorID: admin.ID, CategoryID: categories[0].ID},
		{Title: "MySQL索引优化", Summary: "MySQL索引的原理和优化技巧", Content: "这是MySQL优化文章的内容...", Status: 1, AuthorID: admin.ID, CategoryID: categories[1].ID},
	}
	for i := range articles {
		DB.Create(&articles[i])
	}

	fmt.Println("测试数据已创建（用户: admin/123456，3篇文章，3个分类）")
}
