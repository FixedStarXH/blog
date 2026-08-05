package model

// AdjacentArticle 文章详情页的上一篇/下一篇信息
type AdjacentArticle struct {
	ID    uint   `json:"id"`    // 文章ID（前端跳转用）
	Title string `json:"title"` // 标题（前端显示用）
}

// ArticleNav 上一篇 + 下一篇 + 相关推荐（一次详情页请求全部返回）
type ArticleNav struct {
	Prev    *AdjacentArticle `json:"prev"`    // 上一篇（没有则为 null）
	Next    *AdjacentArticle `json:"next"`    // 下一篇（没有则为 null）
	Related []Article        `json:"related"` // 相关推荐（同分类热门）
}
