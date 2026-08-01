package model

import (
	"fmt"
	"time"

	"blog-system/config"
	"blog-system/utils"

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

	// 自动迁移：按模型建表/改表
	// 说明：ArticleTag 中间表不需要单独登记 —— GORM 发现 Article.Tags 上的
	//       many2many:article_tags 会自动创建中间表，这里是"12 张表"的来源
	if err := DB.AutoMigrate(
		&User{},
		&Column{},
		&Category{},
		&Tag{},
		&Article{},
		&Comment{},
		&ArticleView{},
		&ArticleLike{},
		&Image{},
		&Link{},
		&Setting{},
	); err != nil {
		return fmt.Errorf("自动迁移失败:%w", err)
	}

	initTestData()

	return nil
}

// mustHash 种子数据专用：生成密码哈希
// 为什么不直接用 utils.HashPassword？它返回 (string, error)，种子数据失败应直接暴露，
// 所以包一层：出错就 panic（启动即报错，而不是带病运行）
func mustHash(plain string) string {
	h, err := utils.HashPassword(plain)
	if err != nil {
		panic("密码哈希生成失败: " + err.Error())
	}
	return h
}

// initTestData 种子数据（幂等：已有用户就跳过，不会重复插入）
// 内容对照数据库设计文档 §5：三角色 + 分类 + 标签 + 专栏 + 文章（含 1 篇待审核）
func initTestData() {
	var count int64
	DB.Model(&User{}).Count(&count)
	if count > 0 {
		return
	}

	// 1. 三角色用户（角色常量见 const.go）
	admin := User{Username: "admin", Email: "admin@example.com", Password: mustHash("123456"), Nickname: "站长", Role: RoleAdmin, Status: UserStatusActive}
	editor := User{Username: "editor", Email: "editor@example.com", Password: mustHash("123456"), Nickname: "编辑小陈", Role: RoleEditor, Status: UserStatusActive}
	user1 := User{Username: "user1", Email: "user1@example.com", Password: mustHash("123456"), Nickname: "普通小李", Role: RoleUser, Status: UserStatusActive}
	DB.Create(&admin)
	DB.Create(&editor)
	DB.Create(&user1)

	// 2. 分类
	categories := []Category{
		{Name: "Go语言", Description: "Go语言相关的技术文章", Sort: 1},
		{Name: "数据库", Description: "MySQL、Redis等数据库文章", Sort: 2},
		{Name: "前端", Description: "HTML、CSS、JavaScript文章", Sort: 3},
		{Name: "随笔", Description: "生活随笔与思考", Sort: 4},
	}
	for i := range categories {
		DB.Create(&categories[i])
	}

	// 3. 标签
	tags := []Tag{
		{Name: "gin"},
		{Name: "gorm"},
		{Name: "mysql"},
		{Name: "css"},
	}
	for i := range tags {
		DB.Create(&tags[i])
	}

	// 4. 专栏（作者 admin 开一个，演示"作者个人空间"）
	column := Column{Name: "Go从入门到进阶", Description: "我的Go学习之路", AuthorID: admin.ID, Status: ColumnStatusActive}
	DB.Create(&column)

	// 5. 文章：3 篇已发布 + 1 篇待审核（演示审核流）
	published := []Article{
		{Title: "Go语言入门指南", Summary: "Go语言的基本语法和特性介绍", Content: "<p>这是Go语言入门文章的内容...</p>", Status: ArticleStatusPublished, AuthorID: admin.ID, CategoryID: categories[0].ID, ColumnID: &column.ID, ViewCount: 128, LikeCount: 12},
		{Title: "GORM入门教程", Summary: "GORM的基本使用", Content: "<p>这是GORM教程的内容...</p>", Status: ArticleStatusPublished, AuthorID: admin.ID, CategoryID: categories[0].ID, ViewCount: 96, LikeCount: 8},
		{Title: "MySQL索引优化", Summary: "MySQL索引的原理和优化技巧", Content: "<p>这是MySQL优化文章的内容...</p>", Status: ArticleStatusPublished, AuthorID: editor.ID, CategoryID: categories[1].ID, ViewCount: 200, LikeCount: 30},
	}
	for i := range published {
		DB.Create(&published[i])
	}

	// 待审核文章：user1 投稿，等编辑审核（Status=ArticleStatusPending）
	pending := Article{
		Title: "Redis缓存雪崩与穿透", Summary: "user1的投稿，等待审核", Content: "<p>这是待审核的投稿内容...</p>",
		Status: ArticleStatusPending, AuthorID: user1.ID, CategoryID: categories[1].ID,
	}
	DB.Create(&pending)

	// 6. 给第一篇文章挂标签（多对多：Association + Append，Day10 知识）
	DB.Model(&published[0]).Association("Tags").Append(&tags[0], &tags[1])

	// 7. 站点配置（settings KV 表：站点标题 / 描述）
	DB.Create(&Setting{K: "site_title", V: "Lumi 博客"})
	DB.Create(&Setting{K: "site_description", V: "Go 学习与技术分享"})

	fmt.Println("测试数据已创建（三角色 admin/editor/user1，4分类，4标签，1专栏，4篇文章含1篇待审核）")
}
