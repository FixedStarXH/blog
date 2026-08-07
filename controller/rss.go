package controller

import (
	"blog-system/model"
	"blog-system/service"
	"html"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// RSSController RSS 订阅源（RSS 2.0）
type RSSController struct {
	articleSvc *service.ArticleService
	settingSvc *service.SettingService
}

func NewRSSController(articleSvc *service.ArticleService, settingSvc *service.SettingService) *RSSController {
	return &RSSController{articleSvc: articleSvc, settingSvc: settingSvc}
}

// Feed GET /api/rss.xml → 返回 RSS 2.0 XML
func (c *RSSController) Feed(ctx *gin.Context) {
	// 站点信息（标题/描述/作者等）
	siteTitle := "LUMI 技术博客"
	siteDesc := "Go + Gin 博客系统，技术分享与学习笔记"
	if settings, err := c.settingSvc.GetSiteSettings(); err == nil {
		if t, ok := settings["site_title"]; ok && t != "" {
			siteTitle = t
		}
		if d, ok := settings["site_description"]; ok && d != "" {
			siteDesc = d
		}
	}

	// 取最新 20 篇已发布文章
	articles, _, err := c.articleSvc.GetPublishedArticles("", 0, "", 0, "latest", 1, 20)
	if err != nil {
		ctx.String(http.StatusOK, "<?xml version=\"1.0\" encoding=\"UTF-8\"?><rss version=\"2.0\"><channel><title>%s</title></channel></rss>", html.EscapeString(siteTitle))
		return
	}

	// 站点根 URL（根据请求推断）
	scheme := "http"
	if ctx.Request.TLS != nil || ctx.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	baseURL := scheme + "://" + ctx.Request.Host

	// 拼接 RSS 2.0 XML
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom">` + "\n")
	b.WriteString("  <channel>\n")
	b.WriteString("    <title>" + html.EscapeString(siteTitle) + "</title>\n")
	b.WriteString("    <link>" + html.EscapeString(baseURL) + "</link>\n")
	b.WriteString("    <description>" + html.EscapeString(siteDesc) + "</description>\n")
	b.WriteString("    <language>zh-CN</language>\n")
	b.WriteString("    <lastBuildDate>" + time.Now().Format(time.RFC1123Z) + "</lastBuildDate>\n")
	b.WriteString("    <atom:link href=\"" + html.EscapeString(baseURL) + "/api/rss.xml\" rel=\"self\" type=\"application/rss+xml\"/>\n")

	for _, a := range articles {
		link := baseURL + "/article.html?id=" + uintToStr(a.ID)
		summary := a.Summary
		if summary == "" {
			// 摘要为空时取正文前 200 字纯文本
			summary = stripHTML(a.Content, 200)
		}
		b.WriteString("    <item>\n")
		b.WriteString("      <title>" + html.EscapeString(a.Title) + "</title>\n")
		b.WriteString("      <link>" + html.EscapeString(link) + "</link>\n")
		b.WriteString("      <description>" + html.EscapeString(summary) + "</description>\n")
		b.WriteString("      <pubDate>" + a.CreatedAt.Format(time.RFC1123Z) + "</pubDate>\n")
		b.WriteString("      <guid>" + html.EscapeString(link) + "</guid>\n")
		b.WriteString("      <category>" + html.EscapeString(a.Category.Name) + "</category>\n")
		b.WriteString("    </item>\n")
	}

	b.WriteString("  </channel>\n</rss>")

	ctx.Header("Content-Type", "application/rss+xml; charset=utf-8")
	ctx.String(http.StatusOK, b.String())
}

// stripHTML 去掉 HTML 标签，截取前 n 个字符作为摘要
func stripHTML(s string, n int) string {
	// 粗暴去标签：删掉 <...> 之间的内容
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
			continue
		}
		if !inTag {
			b.WriteRune(r)
		}
	}
	text := strings.TrimSpace(b.String())
	if len([]rune(text)) > n {
		runes := []rune(text)
		return string(runes[:n]) + "..."
	}
	return text
}

func uintToStr(n uint) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// 确保引用 model 包（避免未使用导入）
var _ = model.ArticleStatusPublished
