package dao

import (
	"testing"

	"blog-system/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupCommentDB 评论测试前置：sqlite 内存库 + 文章/用户/评论三张表
// （Comment 的 Preload 需要 articles、users 表存在）
func setupCommentDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("初始化测试内存数据库失败: %v", err)
	}
	if err := db.AutoMigrate(&model.Article{}, &model.User{}, &model.Comment{}); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	if err := db.Create(&model.User{Username: "author", Email: "author@test.com", Password: "x"}).Error; err != nil {
		t.Fatalf("插入作者失败: %v", err)
	}
	if err := db.Create(&model.User{Username: "user1", Email: "user1@test.com", Password: "x"}).Error; err != nil {
		t.Fatalf("插入评论人失败: %v", err)
	}
	if err := db.Create(&model.Article{Title: "被评文章", Content: "正文", AuthorID: 1, CategoryID: 1}).Error; err != nil {
		t.Fatalf("插入文章失败: %v", err)
	}
	return db
}

// createPending 辅助：给文章 1 创建一条待审评论
func createPending(t *testing.T, db *gorm.DB, content string) model.Comment {
	t.Helper()
	c := model.Comment{Content: content, ArticleID: 1, Nickname: "游客"}
	if err := NewCommentDAO().Create(db, &c); err != nil {
		t.Fatalf("创建评论失败: %v", err)
	}
	return c
}

// TestCommentCreateIsPending 新评论默认待审（status=0），前台查不到
func TestCommentCreateIsPending(t *testing.T) {
	db := setupCommentDB(t)
	c := createPending(t, db, "写得真好")

	if c.Status != model.CommentStatusPending {
		t.Errorf("新评论应待审(0)，实际 %d", c.Status)
	}
	list, err := NewCommentDAO().FindApprovedByArticleID(db, 1)
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("待审评论不应出现在前台列表，实际 %d 条", len(list))
	}
}

// TestCommentApproveVisible 审核通过（1）后 → 前台可见
func TestCommentApproveVisible(t *testing.T) {
	db := setupCommentDB(t)
	c := createPending(t, db, "写得真好")

	if err := NewCommentDAO().UpdateStatus(db, c.ID, model.CommentStatusApproved); err != nil {
		t.Fatalf("审核通过失败: %v", err)
	}
	list, err := NewCommentDAO().FindApprovedByArticleID(db, 1)
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if len(list) != 1 || list[0].Content != "写得真好" {
		t.Errorf("通过后应可见 1 条，实际 %d 条: %+v", len(list), list)
	}
}

// TestCommentRejectHidden 驳回（2）后 → 前台不可见
func TestCommentRejectHidden(t *testing.T) {
	db := setupCommentDB(t)
	c := createPending(t, db, "垃圾广告")

	if err := NewCommentDAO().UpdateStatus(db, c.ID, model.CommentStatusRejected); err != nil {
		t.Fatalf("驳回失败: %v", err)
	}
	list, err := NewCommentDAO().FindApprovedByArticleID(db, 1)
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("驳回评论不应出现在前台，实际 %d 条", len(list))
	}
}

// TestCommentSoftDelete 删除（软删除）后 → 前台不可见
func TestCommentSoftDelete(t *testing.T) {
	db := setupCommentDB(t)
	c := createPending(t, db, "要删的评论")

	if err := NewCommentDAO().UpdateStatus(db, c.ID, model.CommentStatusApproved); err != nil {
		t.Fatalf("审核通过失败: %v", err)
	}
	if err := NewCommentDAO().Delete(db, c.ID); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	list, err := NewCommentDAO().FindApprovedByArticleID(db, 1)
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("软删除后前台不应可见，实际 %d 条", len(list))
	}
}

// TestCommentFindAllByStatus 后台列表按状态筛选：待审/通过/驳回互不串
func TestCommentFindAllByStatus(t *testing.T) {
	db := setupCommentDB(t)
	d := NewCommentDAO()

	approved := createPending(t, db, "已通过")
	rejected := createPending(t, db, "已驳回")
	createPending(t, db, "还待审") // 第 3 条：保持待审
	if err := d.UpdateStatus(db, approved.ID, model.CommentStatusApproved); err != nil {
		t.Fatalf("审核通过失败: %v", err)
	}
	if err := d.UpdateStatus(db, rejected.ID, model.CommentStatusRejected); err != nil {
		t.Fatalf("驳回失败: %v", err)
	}

	// 待审列表：只应有第 3 条（默认待审）
	pendings, total, err := d.FindAll(db, model.CommentStatusPending, 1, 10)
	if err != nil {
		t.Fatalf("查询待审列表失败: %v", err)
	}
	if total != 1 || len(pendings) != 1 {
		t.Errorf("待审列表应 1 条，实际 total=%d len=%d", total, len(pendings))
	}
	// 通过列表：1 条
	approvedList, total, _ := d.FindAll(db, model.CommentStatusApproved, 1, 10)
	if total != 1 || len(approvedList) != 1 || approvedList[0].Content != "已通过" {
		t.Errorf("通过列表应 1 条且为'已通过'，实际 total=%d", total)
	}
	// -1 = 全部：3 条
	all, total, _ := d.FindAll(db, -1, 1, 10)
	if total != 3 || len(all) != 3 {
		t.Errorf("全部列表应 3 条，实际 total=%d len=%d", total, len(all))
	}
}
