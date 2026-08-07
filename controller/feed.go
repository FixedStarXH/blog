package controller

import (
	"encoding/xml"
	"net/http"
	"time"

	"blog-system/service"

	"github.com/gin-gonic/gin"
)

// ---- RSS 2.0 结构（字段顺序就是 XML 输出顺序，不能乱）----

type rssItem struct {
	Title       string `xml:"title"`       // 文章标题
	Link        string `xml:"link"`        // 文章绝对 URL
	Description string `xml:"description"` // 摘要
	PubDate     string `xml:"pubDate"`     // 发布时间（RFC1123 格式）
}

type rssChannel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	Items       []rssItem `xml:"item"`
}

type rssDoc struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"` // 根元素的 version="2.0" 属性
	Channel rssChannel `xml:"channel"`
}

// ---- Sitemap 结构 ----

type urlEntry struct {
	Loc     string `xml:"loc"`     // 页面绝对 URL
	LastMod string `xml:"lastmod"` // 最后修改时间（YYYY-MM-DD）
}

type urlsetDoc struct {
	XMLName xml.Name   `xml:"urlset"`
	Xmlns   string     `xml:"xmlns,attr"` // 必须带 sitemaps 命名空间
	Urls    []urlEntry `xml:"url"`
}

type FeedController struct {
	service *service.ArticleService
}

func NewFeedController(service *service.ArticleService) *FeedController {
	return &FeedController{service: service}
}

// Rss 生成 RSS 2.0（最新 20 篇已发布文章）
// GET /feed.xml
func (c *FeedController) Rss(ctx *gin.Context) {
	articles, _, err := c.service.GetPublishedArticles("", 0, "", 0, "newest", 1, 20)
	if err != nil {
		ctx.String(http.StatusInternalServerError, "生成 RSS 失败")
		return
	}
	// 用请求的域名拼绝对 URL（部署后自动指向真实域名）
	base := "http://" + ctx.Request.Host

	doc := rssDoc{
		Version: "2.0",
		Channel: rssChannel{
			Title:       "Lumi 博客",
			Link:        base,
			Description: "Go 学习与技术分享",
			Items:       make([]rssItem, 0, len(articles)),
		},
	}
	for _, a := range articles {
		doc.Channel.Items = append(doc.Channel.Items, rssItem{
			Title:       a.Title,
			Link:        base + "/articles/" + itoa(a.ID), // 前端 SPA 的文章页 URL
			Description: firstLine(a.Content),
			PubDate:     a.CreatedAt.Format(time.RFC1123Z), // RSS 标准时间格式
		})
	}

	writeXML(ctx, doc)
}

// Sitemap 生成站点地图（已发布文章，供搜索引擎收录）
// GET /sitemap.xml
func (c *FeedController) Sitemap(ctx *gin.Context) {
	// 教学简化：取最新 50 篇；生产环境应循环分页拉全量
	articles, _, err := c.service.GetPublishedArticles("", 0, "", 0, "newest", 1, 50)
	if err != nil {
		ctx.String(http.StatusInternalServerError, "生成 Sitemap 失败")
		return
	}
	base := "http://" + ctx.Request.Host

	doc := urlsetDoc{
		Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
		Urls:  make([]urlEntry, 0, len(articles)),
	}
	// 首页 + 每篇文章
	doc.Urls = append(doc.Urls, urlEntry{Loc: base, LastMod: time.Now().Format("2006-01-02")})
	for _, a := range articles {
		doc.Urls = append(doc.Urls, urlEntry{
			Loc:     base + "/articles/" + itoa(a.ID),
			LastMod: a.UpdatedAt.Format("2006-01-02"),
		})
	}

	writeXML(ctx, doc)
}

// writeXML 统一的 XML 输出：MarshalIndent 美化 + 声明头 + 正确 Content-Type
func writeXML(ctx *gin.Context, v any) {
	data, err := xml.MarshalIndent(v, "", "  ")
	if err != nil {
		ctx.String(http.StatusInternalServerError, "XML 序列化失败")
		return
	}
	// xml.Header = <?xml version="1.0" encoding="UTF-8"?>
	ctx.Data(http.StatusOK, "application/xml; charset=utf-8", append([]byte(xml.Header), data...))
}

// itoa 简易整数转字符串（避免每个文件都 import strconv）
func itoa(n uint) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// firstLine 取正文前 200 字符当摘要（简化版；生产可用正则剥 HTML 标签）
func firstLine(s string) string {
	if len(s) > 200 {
		return s[:200]
	}
	return s
}
