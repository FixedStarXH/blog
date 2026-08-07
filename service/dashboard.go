package service

import (
	"blog-system/model"

	"gorm.io/gorm"
)

// DashboardService 仪表盘统计
// 为什么直接持有 db 而不是某个 DAO？
// 统计是"跨实体聚合"（文章数 + 浏览量 + 评论数 + 热门文章 + 最近评论），
// 不属于任何一个具体实体的 DAO 职责，所以这里直接对 db 做查询。
type DashboardService struct {
	db *gorm.DB
}

func NewDashboardService(db *gorm.DB) *DashboardService {
	return &DashboardService{db: db}
}

// Counts 四块数字卡片
type Counts struct {
	Articles        int64 `json:"articles"`        // 文章总数（含草稿/待审）
	Views           int64 `json:"views"`           // 全站总浏览量
	Comments        int64 `json:"comments"`        // 评论总数
	PendingComments int64 `json:"pendingComments"` // 待审核评论数
}

// DashboardData 仪表盘整包数据（前端契约见 admin/dashboard.html）
type DashboardData struct {
	Counts         Counts          `json:"counts"`
	HotArticles    []model.Article `json:"hotArticles"`    // 热门 TOP5
	RecentComments []model.Comment `json:"recentComments"` // 最近 10 条评论
}

func (s *DashboardService) GetDashboard() (*DashboardData, error) {
	d := &DashboardData{}

	// ① 四块统计（COUNT 直接进结构体指针，无需 Scan）
	s.db.Model(&model.Article{}).Count(&d.Counts.Articles)
	s.db.Model(&model.Comment{}).Count(&d.Counts.Comments)
	s.db.Model(&model.Comment{}).
		Where("status = ?", model.CommentStatusPending).
		Count(&d.Counts.PendingComments)
	// 总浏览量：SUM 可能为 NULL（没文章时），COALESCE 兜底成 0
	if err := s.db.Model(&model.Article{}).
		Select("COALESCE(SUM(view_count),0) AS views").
		Scan(&d.Counts.Views).Error; err != nil {
		return nil, err
	}

	// ② 热门文章 TOP5（已发布，按浏览量倒序，带分类名）
	if err := s.db.Model(&model.Article{}).
		Where("status = ?", model.ArticleStatusPublished).
		Order("view_count desc").
		Limit(5).
		Preload("Category").
		Find(&d.HotArticles).Error; err != nil {
		return nil, err
	}

	// ③ 最近 10 条评论（带文章标题，填充 ArticleTitle 给前端用）
	var comments []model.Comment
	if err := s.db.Preload("Article").
		Order("created_at desc").
		Limit(10).
		Find(&comments).Error; err != nil {
		return nil, err
	}
	for i := range comments {
		comments[i].ArticleTitle = comments[i].Article.Title
	}
	d.RecentComments = comments

	return d, nil
}
