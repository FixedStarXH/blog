package service

import (
	"blog-system/dao"
	"blog-system/model"
	"math/rand/v2"
	"strings"
	"time"

	"gorm.io/gorm"
)

type SettingService struct {
	dao *dao.SettingDAO
	db  *gorm.DB
}

func NewSettingService(dao *dao.SettingDAO, db *gorm.DB) *SettingService {
	return &SettingService{dao: dao, db: db}
}

// GetSiteSettings 获取站点全部配置（KV map，老接口 /api/settings 用）
func (s *SettingService) GetSiteSettings() (map[string]string, error) {
	return s.dao.FindAllAsMap(s.db)
}

// GetDailyQuote 随机返回一条名言（老接口 /api/quote 用，纯字符串）
func (s *SettingService) GetDailyQuote() (string, error) {
	quotes, err := s.dao.GetQuotes(s.db)
	if err != nil {
		return "", err
	}
	// 名言池为空时兜底返回一句默认名言：
	// rand.IntN(0) 会 panic（除零），管理员清空名言池时不能把公开接口打崩
	if len(quotes) == 0 {
		return "把每一个今天都当作新的开始。", nil
	}
	// rand.IntN(len) 返回 0..len-1 的随机整数，正好当切片下标
	return quotes[rand.IntN(len(quotes))], nil
}

// ------------------------------------------------------------
// 后台设置（admin/settings.html 契约）
// ------------------------------------------------------------

// QuoteItem 一条名言（前端编辑框里是"内容 | 作者"的一行）
type QuoteItem struct {
	Content string `json:"content"`
	Author  string `json:"author"`
}

// GetAdminSettings 后台设置全量（含结构化的名言池）
// 返回的就是 KV map + quotes 数组，前端直接按键名取值
func (s *SettingService) GetAdminSettings() (map[string]interface{}, error) {
	kv, err := s.dao.FindAllAsMap(s.db)
	if err != nil {
		return nil, err
	}
	result := make(map[string]interface{}, len(kv)+1)
	for k, v := range kv {
		result[k] = v
	}
	result["quotes"] = parseQuotes(kv["daily_quotes"])
	return result, nil
}

// SiteSettingsInput 后台保存设置的结构化输入（struct 即白名单：不在结构里的键直接丢弃）
type SiteSettingsInput struct {
	SiteTitle       string
	SiteSubtitle    string
	SiteDescription string
	SiteLogo        string
	SiteBeian       string
	SocialGithub    string
	SocialEmail     string
	Quotes          []QuoteItem
}

// UpdateSettings 保存站点设置：struct 字段本身就是白名单，防止往 KV 表乱写键
func (s *SettingService) UpdateSettings(input *SiteSettingsInput) error {
	kv := map[string]string{
		"site_title":       input.SiteTitle,
		"site_subtitle":    input.SiteSubtitle,
		"site_description": input.SiteDescription,
		"site_logo":        input.SiteLogo,
		"site_beian":       input.SiteBeian,
		"social_github":    input.SocialGithub,
		"social_email":     input.SocialEmail,
	}
	// 名言池：数组 → 每行 "内容 | 作者"（与前端编辑框格式一致，可来回倒）
	var lines []string
	for _, q := range input.Quotes {
		author := strings.TrimSpace(q.Author)
		if author == "" {
			author = "佚名"
		}
		lines = append(lines, strings.TrimSpace(q.Content)+" | "+author)
	}
	kv["daily_quotes"] = strings.Join(lines, "\n")
	return s.dao.Upsert(s.db, kv)
}

// parseQuotes 把 "内容 | 作者" 每行拆成结构化对象（兼容老数据：没有 | 的行作者默认佚名）
func parseQuotes(raw string) []QuoteItem {
	var list []QuoteItem
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// SplitN 只拆第一个 |，内容里出现 | 也不怕
		parts := strings.SplitN(line, "|", 2)
		content := strings.TrimSpace(parts[0])
		author := "佚名"
		if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
			author = strings.TrimSpace(parts[1])
		}
		list = append(list, QuoteItem{Content: content, Author: author})
	}
	return list
}

// ------------------------------------------------------------
// 前台接口（home.js / common.js / about.html 契约）
// ------------------------------------------------------------

// SiteStats 首页统计数字
type SiteStats struct {
	ArticleCount int64 `json:"articleCount"` // 已发布文章数
	ViewCount    int64 `json:"viewCount"`    // 全站总浏览量
	FoundYear    int   `json:"foundYear"`    // 建站年份（最早文章年份）
}

// SiteInfo 前台站点信息 GET /api/site
// 前端契约（home.js）：{ title, github, email, stats: {...} }
type SiteInfo struct {
	Title  string    `json:"title"`
	Github string    `json:"github"`
	Email  string    `json:"email"`
	Stats  SiteStats `json:"stats"`
}

// GetSiteInfo 组装前台站点信息：设置 KV + 文章统计
func (s *SettingService) GetSiteInfo() (*SiteInfo, error) {
	kv, err := s.dao.FindAllAsMap(s.db)
	if err != nil {
		return nil, err
	}
	info := &SiteInfo{
		Title:  kv["site_title"],
		Github: kv["social_github"],
		Email:  kv["social_email"],
	}

	// 统计（直接查文章表）
	s.db.Model(&model.Article{}).
		Where("status = ?", model.ArticleStatusPublished).
		Count(&info.Stats.ArticleCount)
	// SUM 可能为 NULL（没文章时），COALESCE 兜底 0
	s.db.Model(&model.Article{}).
		Select("COALESCE(SUM(view_count),0) AS views").
		Scan(&info.Stats.ViewCount)
	// 建站年份 = 最早一篇文章的年份；没文章就取今年
	var earliest time.Time
	s.db.Model(&model.Article{}).
		Select("COALESCE(MIN(created_at), NOW())").
		Scan(&earliest)
	info.Stats.FoundYear = earliest.Year()

	return info, nil
}

// GetRandomQuote 前台每日一言（结构化）GET /api/quote/random
// 前端契约（common.js）：{ content, author }
func (s *SettingService) GetRandomQuote() (map[string]string, error) {
	quotes, err := s.dao.GetQuotes(s.db)
	if err != nil {
		return nil, err
	}
	if len(quotes) == 0 {
		return map[string]string{"content": "把每一个今天都当作新的开始。", "author": "佚名"}, nil
	}
	// 随机取一行，拆成 content/author
	line := quotes[rand.IntN(len(quotes))]
	parts := strings.SplitN(line, "|", 2)
	content := strings.TrimSpace(parts[0])
	author := "佚名"
	if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
		author = strings.TrimSpace(parts[1])
	}
	return map[string]string{"content": content, "author": author}, nil
}
