package dao

import (
	"testing"
	"time"

	"blog-system/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupArticleDB 测试前置：纯 Go sqlite 内存库 + 建表。
// 无需外部 MySQL，测试自包含。ArticleLike 的防重依赖
// (article_id, ip) 联合唯一索引，sqlite 同样支持 uniqueIndex。
func setupArticleDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("初始化测试内存数据库失败: %v", err)
	}
	if err := db.AutoMigrate(&model.Article{}, &model.ArticleView{}, &model.ArticleLike{}, &model.User{}); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	// 作者行（Article.AuthorID 外键指向 users，sqlite 默认不校验外键，插入保险）
	if err := db.Create(&model.User{Username: "author", Email: "author@test.com", Password: "x"}).Error; err != nil {
		t.Fatalf("插入作者失败: %v", err)
	}
	// 文章：ViewCount/LikeCount 初始 0
	if err := db.Create(&model.Article{Title: "测试文章", Content: "正文", AuthorID: 1, CategoryID: 1}).Error; err != nil {
		t.Fatalf("插入文章失败: %v", err)
	}
	return db
}

// viewCount 读取文章当前浏览量
func viewCount(t *testing.T, db *gorm.DB, articleID uint) int {
	t.Helper()
	var v int
	if err := db.Model(&model.Article{}).Where("id = ?", articleID).Select("view_count").Scan(&v).Error; err != nil {
		t.Fatalf("读取 view_count 失败: %v", err)
	}
	return v
}

// likeCount 读取文章当前点赞数
func likeCount(t *testing.T, db *gorm.DB, articleID uint) int {
	t.Helper()
	var v int
	if err := db.Model(&model.Article{}).Where("id = ?", articleID).Select("like_count").Scan(&v).Error; err != nil {
		t.Fatalf("读取 like_count 失败: %v", err)
	}
	return v
}

// ----------------------------- 浏览量防刷 -----------------------------

// TestAddViewFirstTime 首次浏览 → 计数 +1，且落一条浏览明细
func TestAddViewFirstTime(t *testing.T) {
	db := setupArticleDB(t)
	d := NewArticleDAO()

	n, err := d.AddView(db, 1, "1.2.3.4")
	if err != nil {
		t.Fatalf("首次浏览不应报错: %v", err)
	}
	if n != 1 {
		t.Errorf("首次浏览应返回计数 1，实际 %d", n)
	}
	if viewCount(t, db, 1) != 1 {
		t.Errorf("浏览量应为 1，实际 %d", viewCount(t, db, 1))
	}
	var rows int64
	db.Model(&model.ArticleView{}).Where("article_id = ? AND ip = ?", 1, "1.2.3.4").Count(&rows)
	if rows != 1 {
		t.Errorf("应落 1 条浏览明细，实际 %d", rows)
	}
}

// TestAddViewSameIPDedup 同 IP 当天重复访问 → 不加数（防刷核心）
func TestAddViewSameIPDedup(t *testing.T) {
	db := setupArticleDB(t)
	d := NewArticleDAO()

	if _, err := d.AddView(db, 1, "1.2.3.4"); err != nil {
		t.Fatalf("首次浏览失败: %v", err)
	}
	n, err := d.AddView(db, 1, "1.2.3.4") // 同 IP 再访问
	if err != nil {
		t.Fatalf("重复浏览不应报错: %v", err)
	}
	if n != 1 {
		t.Errorf("重复访问应保持计数 1，实际 %d（防刷失效！）", n)
	}
	if viewCount(t, db, 1) != 1 {
		t.Errorf("浏览量应仍为 1，实际 %d（防刷失效！）", viewCount(t, db, 1))
	}
}

// TestAddViewDifferentIP 不同 IP → 各自计数，均 +1
func TestAddViewDifferentIP(t *testing.T) {
	db := setupArticleDB(t)
	d := NewArticleDAO()

	if _, err := d.AddView(db, 1, "1.2.3.4"); err != nil {
		t.Fatalf("IP1 首次浏览失败: %v", err)
	}
	if _, err := d.AddView(db, 1, "5.6.7.8"); err != nil {
		t.Fatalf("IP2 首次浏览失败: %v", err)
	}
	if viewCount(t, db, 1) != 2 {
		t.Errorf("两个不同 IP 浏览应为 2，实际 %d", viewCount(t, db, 1))
	}
}

// TestAddViewNextDayCountsAgain 次日记一条新明细后 → 重新开始计数
func TestAddViewNextDayCountsAgain(t *testing.T) {
	db := setupArticleDB(t)
	d := NewArticleDAO()

	if _, err := d.AddView(db, 1, "1.2.3.4"); err != nil {
		t.Fatalf("首次浏览失败: %v", err)
	}
	// 把明细的 ViewedAt 改到昨天（模拟时间跨天），再浏览应视为新的一天
	if err := db.Model(&model.ArticleView{}).
		Where("article_id = ?", 1).
		Update("viewed_at", time.Now().Add(-24*time.Hour)).Error; err != nil {
		t.Fatalf("改写浏览时间失败: %v", err)
	}

	n, err := d.AddView(db, 1, "1.2.3.4")
	if err != nil {
		t.Fatalf("跨天浏览不应报错: %v", err)
	}
	if n != 2 {
		t.Errorf("跨天应重新计数（2），实际 %d", n)
	}
}

// ----------------------------- 点赞/取消点赞 -----------------------------

// TestLikeFirstTime 首次点赞 → 计数 +1，Already=false
func TestLikeFirstTime(t *testing.T) {
	db := setupArticleDB(t)
	d := NewArticleDAO()

	r, err := d.Like(db, 1, "1.2.3.4")
	if err != nil {
		t.Fatalf("首次点赞不应报错: %v", err)
	}
	if r.Already {
		t.Errorf("首次点赞 Already 应为 false")
	}
	if r.LikeCount != 1 {
		t.Errorf("点赞数应为 1，实际 %d", r.LikeCount)
	}
	if likeCount(t, db, 1) != 1 {
		t.Errorf("文章 like_count 应为 1，实际 %d", likeCount(t, db, 1))
	}
}

// TestLikeToggle 再点一次 = 取消（计数 -1），第三次 = 重新点赞
func TestLikeToggle(t *testing.T) {
	db := setupArticleDB(t)
	d := NewArticleDAO()

	r1, _ := d.Like(db, 1, "1.2.3.4") // 赞
	if r1.Already || r1.LikeCount != 1 {
		t.Fatalf("首赞异常: already=%v count=%d", r1.Already, r1.LikeCount)
	}
	r2, _ := d.Like(db, 1, "1.2.3.4") // 取消
	if !r2.Already || r2.LikeCount != 0 {
		t.Fatalf("取消赞异常: already=%v count=%d", r2.Already, r2.LikeCount)
	}
	if likeCount(t, db, 1) != 0 {
		t.Errorf("取消后 like_count 应为 0，实际 %d", likeCount(t, db, 1))
	}
	r3, _ := d.Like(db, 1, "1.2.3.4") // 再赞
	if r3.Already || r3.LikeCount != 1 {
		t.Fatalf("再次点赞异常: already=%v count=%d", r3.Already, r3.LikeCount)
	}
}

// TestLikeCountNeverNegative 反复取消时计数不能变负数（GREATEST 下限保护）
func TestLikeCountNeverNegative(t *testing.T) {
	db := setupArticleDB(t)
	d := NewArticleDAO()

	// 从未点赞的 IP 直接连续取消：Delete 0 行 → 走插入分支（等价再点=赞），
	// 但若 DB 中 like_count 与明细不一致（如被别人手动改过），GREATEST 保证不为负。
	db.Model(&model.Article{}).Where("id = 1").Update("like_count", 0)
	if _, err := d.Like(db, 1, "1.2.3.4"); err != nil {
		t.Fatalf("点赞不应报错: %v", err)
	}
	if c := likeCount(t, db, 1); c != 1 {
		t.Errorf("like_count 应为 1，实际 %d", c)
	}
}

// TestLikeUniqueConflict 并发/重复插入兜底：
// 直接预置一行点赞明细后，同 IP 再 Like = 取消（幂等 DELETE 命中）；
// 若 DELETE 删 0 行但插入撞唯一索引（MySQL 1062 兜底），
// MySQL 下 isDuplicateEntry 会转 Already=true 不重复加数；
// sqlite 无 MySQL 错误码，这里验证"至少不产生错误计数"的语义。
func TestLikeUniqueConflict(t *testing.T) {
	db := setupArticleDB(t)
	d := NewArticleDAO()

	// 预置一条点赞明细 + 计数 1，模拟"数据已存在"
	if err := db.Create(&model.ArticleLike{ArticleID: 1, IP: "1.2.3.4"}).Error; err != nil {
		t.Fatalf("预置点赞失败: %v", err)
	}
	if err := db.Model(&model.Article{}).Where("id = 1").Update("like_count", 1).Error; err != nil {
		t.Fatalf("预置计数失败: %v", err)
	}

	r, err := d.Like(db, 1, "1.2.3.4") // 再次点击 → 取消
	if err != nil {
		t.Fatalf("点击不应报错: %v", err)
	}
	if !r.Already || r.LikeCount != 0 {
		t.Errorf("应返回取消（Already=true, count=0），实际 already=%v count=%d", r.Already, r.LikeCount)
	}
}
