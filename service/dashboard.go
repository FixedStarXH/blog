package service

import (
	"time"

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

// ViewTrendPoint 单日浏览量（前端折线图一个点）
type ViewTrendPoint struct {
	Date  string `json:"date"`  // YYYY-MM-DD
	Views int64  `json:"views"` // 当日浏览明细条数
}

// ViewTrend 近 N 天浏览量趋势（按 article_views 明细表按天聚合）
//
// 为什么用明细表而不是文章的 ViewCount？
//
//	ViewCount 是"累计值"，无法拆出"某天新增多少"；
//	article_views 每行是一次真实浏览（带 ViewedAt），按天 GROUP BY 即可得到趋势。
//	当天无浏览的日期也要出现在结果里（前端折线图需要连续日期），Go 侧补 0。
//
// 为什么用 DATE_FORMAT 而不是 DATE()？
//
//	MySQL 驱动开启 parseTime 后，DATE() 返回的 date 类型会被转成 time.Time，
//	Scan 进 string 字段时变成 "2026-08-10T00:00:00+08:00" 这种格式，
//	和 Go 侧 key（"2026-08-10"）对不上。DATE_FORMAT 直接返回纯字符串。
//	注意：这是 MySQL 专有函数，本方法无单测依赖 sqlite，生产仅 MySQL，可接受。
func (s *DashboardService) ViewTrend(days int) ([]ViewTrendPoint, error) {
	if days <= 0 || days > 365 {
		days = 30
	}
	// 起点：今天往前推 days-1 天的 00:00
	today := time.Now()
	start := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.Local).
		AddDate(0, 0, -(days - 1))

	var rows []struct {
		Date  string
		Views int64
	}
	if err := s.db.Raw(
		"SELECT DATE_FORMAT(viewed_at, '%Y-%m-%d') AS date, COUNT(*) AS views FROM article_views "+
			"WHERE viewed_at >= ? AND deleted_at IS NULL GROUP BY DATE_FORMAT(viewed_at, '%Y-%m-%d') ORDER BY date",
		start,
	).Scan(&rows).Error; err != nil {
		return nil, err
	}

	// 缺失日期补 0（当天没浏览也要有"一个点"）
	byDate := make(map[string]int64, len(rows))
	for _, r := range rows {
		byDate[r.Date] = r.Views
	}
	out := make([]ViewTrendPoint, 0, days)
	for i := 0; i < days; i++ {
		key := start.AddDate(0, 0, i).Format("2006-01-02")
		out = append(out, ViewTrendPoint{Date: key, Views: byDate[key]})
	}
	return out, nil
}
