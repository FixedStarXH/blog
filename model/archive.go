package model

// Archive 时间归档（前台归档页）
// 前端契约：[{year: "2026", count: 3, articles: [{id, title, createdAt}]}]
type Archive struct {
	Year     string    `json:"year"`     // 年份，如 "2026"
	Count    int       `json:"count"`    // 该年文章数
	Articles []Article `json:"articles"` // 该年文章列表
}
