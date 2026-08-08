// 命令：go run ./cmd/csdn_import
//
// 功能：抓取指定 CSDN 博客的文章（列表页拿 ID/摘要/时间，详情页拿标题/正文 HTML），
//
//	通过后台接口以指定账号身份批量导入本站。
//
// 常用参数：
//
//	go run ./cmd/csdn_import -dry              # 试跑：只抓不导入，先看解析对不对
//	go run ./cmd/csdn_import                   # 试跑：只导入第一篇
//	go run ./cmd/csdn_import -all              # 全量导入
//
// 教学点：
//  1. 反爬三件套：浏览器 User-Agent + Referer + 请求间隔（time.Sleep）
//  2. 正则适合"提 ID / 提纯文本"，正文这种嵌套 HTML 必须用解析器（x/net/html）
//  3. 幂等：导入前先拉已有标题，标题重复就跳过（脚本可安全重复跑）
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// ==================== 配置（命令行参数，不写死） ====================
var (
	baseURL   = flag.String("base", "http://localhost:8080", "博客后端地址")
	csdnUser  = flag.String("user", "FixedstarXH", "CSDN 用户名（主页 URL 里的一段）")
	blogUser  = flag.String("login-user", "Lumi", "博客后台账号")
	blogPass  = flag.String("login-pass", "123456", "博客后台密码")
	importAll = flag.Bool("all", false, "true=全量导入；false=只导入第一篇（试跑）")
	dryRun    = flag.Bool("dry", false, "true=只抓不导入（预览解析结果）")
)

// 仿浏览器请求头：CSDN 对无 UA 的请求直接回验证页
const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// 请求间隔：CSDN 详情页有 Cloudflare 反爬，连续 10 个请求后开始 521
// 教学实测：1.5s 太激进，3s 起步、失败再等 5s 重试，才稳定
const (
	sleepBetween = 3 * time.Second
	retryTimes   = 3 // 单篇抓取失败重试次数
	retryWait    = 5 * time.Second
)

var (
	httpClient = &http.Client{Timeout: 30 * time.Second}

	// 列表页：每篇文章的 ID（详情页 URL 里的数字）
	reArticleID = regexp.MustCompile(`data-articleid="(\d+)"`)
	// 列表页：摘要（纯文本）
	reSummary = regexp.MustCompile(`(?s)<p class="content">(.*?)</p>`)
	// 列表页：发布时间
	reDate = regexp.MustCompile(`<span class="date">([^<]+)</span>`)
	// 详情页：标题
	reTitle = regexp.MustCompile(`(?s)<h1[^>]*class="title-article"[^>]*>(.*?)</h1>`)
)

// articleMeta 列表页一篇文章的信息（正文在详情页，标题也从详情页取，保证一致）
type articleMeta struct {
	ID      string // CSDN 文章ID
	Summary string // 摘要（纯文本）
	Date    string // 发布时间
}

// ==================== 抓取 CSDN ====================

func fetchHTML(url string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", "https://blog.csdn.net/")
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d 无法访问: %s", resp.StatusCode, url)
	}

	// 先读原始字节，再根据声明的 charset 转码
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// CSDN 部分页面声明 charset=gbk，Go 默认按 UTF-8 读会乱码
	// 判断优先级：HTTP Content-Type > HTML meta
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	bodyLower := strings.ToLower(string(b[:min(len(b), 2048)]))
	isGBK := strings.Contains(ct, "gbk") || strings.Contains(ct, "gb2312") || strings.Contains(ct, "gb18030") ||
		strings.Contains(bodyLower, "charset=gbk") || strings.Contains(bodyLower, "charset=gb2312")
	if isGBK {
		decoded, _, err := transform.Bytes(simplifiedchinese.GBK.NewDecoder(), b)
		if err != nil {
			return "", fmt.Errorf("GBK 解码失败: %w", err)
		}
		return string(decoded), nil
	}
	return string(b), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// fetchListPage 抓一页列表，解析出该页所有文章元信息
func fetchListPage(page int) ([]articleMeta, error) {
	url := fmt.Sprintf("https://blog.csdn.net/%s/article/list/%d", *csdnUser, page)
	pageHTML, err := fetchHTML(url)
	if err != nil {
		return nil, err
	}

	// 先拿全部文章ID，再把页面按 ID 切成段，每段内提取摘要/时间
	ids := reArticleID.FindAllStringSubmatch(pageHTML, -1)
	metas := make([]articleMeta, 0, len(ids))
	parts := strings.Split(pageHTML, "data-articleid=")
	for i := 1; i < len(parts) && i <= len(ids); i++ {
		m := articleMeta{ID: ids[i-1][1]}
		seg := parts[i] // 第 i 段 = 第 i 篇文章的 HTML
		if s := reSummary.FindStringSubmatch(seg); len(s) > 0 {
			m.Summary = cleanText(s[1])
		}
		if d := reDate.FindStringSubmatch(seg); len(d) > 0 {
			m.Date = strings.TrimSpace(d[1])
		}
		metas = append(metas, m)
	}
	return metas, nil
}

// fetchArticle 抓详情页，提取标题 + 正文 HTML（失败自动重试，绕过反爬的偶发拦截）
func fetchArticle(id string) (title, content string, err error) {
	url := fmt.Sprintf("https://blog.csdn.net/%s/article/details/%s", *csdnUser, id)
	for attempt := 1; attempt <= retryTimes; attempt++ {
		detailHTML, e := fetchHTML(url)
		if e != nil {
			err = e // 记录最后一次错误：重试耗尽后主流程要能判断"这篇没抓到"
			fmt.Printf("        (第 %d 次抓取失败: %v，等待 %v 后重试)\n", attempt, e, retryWait)
			time.Sleep(retryWait)
			continue
		}
		if t := reTitle.FindStringSubmatch(detailHTML); len(t) > 0 {
			title = cleanText(t[1])
		}
		content, err = extractContentViews(detailHTML)
		return title, content, err
	}
	return "", "", err
}

// extractContentViews 提取 <div id="content_views"> 的内部 HTML（正文）
// 为什么不用正则？正文里 div 层层嵌套，正则没法配对闭合标签；
// x/net/html 是 Go 官方解析库，构建出 DOM 树后取节点子元素，天然正确。
func extractContentViews(page string) (string, error) {
	doc, err := html.Parse(strings.NewReader(page))
	if err != nil {
		return "", err
	}
	var out strings.Builder
	var find func(*html.Node) bool
	find = func(n *html.Node) bool {
		if n.Type == html.ElementNode && n.Data == "div" {
			for _, a := range n.Attr {
				if a.Key == "id" && a.Val == "content_views" {
					// 把该 div 的所有子节点逐个序列化成 HTML
					for c := n.FirstChild; c != nil; c = c.NextSibling {
						html.Render(&out, c)
					}
					return true
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if find(c) {
				return true
			}
		}
		return false
	}
	find(doc)
	return out.String(), nil
}

// cleanText 去掉 HTML 标签并合并连续空白（标题/摘要用）
func cleanText(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			b.WriteRune(r)
		}
	}
	// Fields 按空白拆分再拼接 = 合并多余空格/换行
	return strings.Join(strings.Fields(b.String()), " ")
}

// ==================== 调用博客后端 ====================

// apiResp 后端统一响应格式 {code, message, data}
type apiResp struct {
	Code int             `json:"code"`
	Msg  string          `json:"message"`
	Data json.RawMessage `json:"data"`
}

func postJSON(url, token string, body any) (*apiResp, error) {
	buf, _ := json.Marshal(body)
	req, err := http.NewRequest("POST", url, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var r apiResp
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	return &r, nil
}

func getJSON(url, token string) (*apiResp, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var r apiResp
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	return &r, nil
}

// login 用博客账号登录拿 token
func login() (string, error) {
	r, err := postJSON(*baseURL+"/api/admin/login", "", map[string]string{
		"username": *blogUser,
		"password": *blogPass,
	})
	if err != nil {
		return "", err
	}
	if r.Code != 200 {
		return "", fmt.Errorf("登录失败(%d): %s", r.Code, r.Msg)
	}
	var data struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(r.Data, &data); err != nil {
		return "", err
	}
	return data.Token, nil
}

// existingTitles 拉取后台所有文章标题（幂等：重复运行跳过已导入的）
func existingTitles(token string) (map[string]bool, error) {
	set := map[string]bool{}
	for page := 1; ; page++ {
		url := fmt.Sprintf("%s/api/admin/articles?page=%d&pageSize=100", *baseURL, page)
		r, err := getJSON(url, token)
		if err != nil {
			return nil, err
		}
		if r.Code != 200 {
			return nil, fmt.Errorf("拉文章列表失败(%d): %s", r.Code, r.Msg)
		}
		var data struct {
			Total int `json:"total"`
			List  []struct {
				Title string `json:"title"`
			} `json:"list"`
		}
		if err := json.Unmarshal(r.Data, &data); err != nil {
			return nil, err
		}
		for _, a := range data.List {
			set[a.Title] = true
		}
		if page*100 >= int(data.Total) {
			break
		}
	}
	return set, nil
}

// importArticle 通过后台接口导入一篇文章（status=1 直接发布）
func importArticle(token string, m articleMeta, title, content string) error {
	payload := map[string]any{
		"title":      title,
		"content":    content,
		"summary":    m.Summary,
		"categoryId": 1, // 固定进分类 1（导入后可在后台自行调整）
		"status":     1, // 直接发布
		"isTop":      false,
		"password":   "",
		"publishAt":  nil,
		"tagIds":     []int{},
		"sourceUrl":  fmt.Sprintf("https://blog.csdn.net/%s/article/details/%s", *csdnUser, m.ID), // CSDN 原文地址（裂图兜底跳转用）
	}
	r, err := postJSON(*baseURL+"/api/admin/articles", token, payload)
	if err != nil {
		return err
	}
	if r.Code != 200 {
		return fmt.Errorf("导入失败(%d): %s", r.Code, r.Msg)
	}
	return nil
}

// ==================== 主流程 ====================

func main() {
	flag.Parse()
	fmt.Printf("==> CSDN 用户: %s\n", *csdnUser)

	// ① 登录博客后台
	token, err := login()
	if err != nil {
		fmt.Println("[失败]", err)
		return
	}
	fmt.Printf("==> 后台登录成功: %s\n", *blogUser)

	// ② 拉已有标题（幂等）
	exists, err := existingTitles(token)
	if err != nil {
		fmt.Println("[失败]", err)
		return
	}
	fmt.Printf("==> 本站已有文章 %d 篇（同标题将跳过）\n", len(exists))

	// ③ 翻列表页收集全部文章元信息
	var metas []articleMeta
	for page := 1; ; page++ {
		list, err := fetchListPage(page)
		if err != nil {
			if page == 1 {
				fmt.Println("[失败] 抓列表页:", err)
				return
			}
			break // 后续页抓不到 = 已到末页
		}
		if len(list) == 0 {
			break // 空页 = 翻完了
		}
		metas = append(metas, list...)
		fmt.Printf("    第 %d 页: 抓取 %d 篇（累计 %d）\n", page, len(list), len(metas))
		if !*importAll {
			break // 试跑模式：只翻第一页
		}
		time.Sleep(sleepBetween)
	}
	fmt.Printf("==> 共发现 %d 篇文章\n", len(metas))

	// ④ 逐篇抓详情 + 导入（含图片提取与上传）
	imported, skipped, failed := 0, 0, 0
	consecutiveFails := 0 // 连续失败计数：连续 3 篇失败 = 大概率触发反爬（521），提前停，避免无效重试
	for i, m := range metas {
		title, content, err := fetchArticle(m.ID)
		if err != nil {
			fmt.Printf("  [%d/%d] 抓详情失败(id=%s): %v\n", i+1, len(metas), m.ID, err)
			failed++
			consecutiveFails++
			if consecutiveFails >= 3 {
				fmt.Printf("==> 连续 %d 篇抓取失败，大概率触发 CSDN 反爬（HTTP 521 是 Cloudflare 拦截）。\n", consecutiveFails)
				fmt.Println("==> 等 10 分钟冷却后再跑本脚本（已导入的标题会自动跳过，可放心重复跑）。")
				break
			}
			time.Sleep(sleepBetween)
			continue
		}
		consecutiveFails = 0 // 成功一篇就清零
		fmt.Printf("  [%d/%d] %s（原发表于 %s）\n", i+1, len(metas), title, m.Date)

		if *dryRun {
			fmt.Printf("        摘要: %s\n        正文字符数: %d\n", m.Summary, len(content))
			continue // 试跑：看完就停
		}

		if exists[title] {
			fmt.Printf("        - 已存在，跳过\n")
			skipped++
		} else {
			if err := importArticle(token, m, title, content); err != nil {
				fmt.Printf("        - %v\n", err)
				failed++
			} else {
				fmt.Printf("        + 导入成功\n")
				imported++
			}
		}

		if !*importAll {
			fmt.Println("==> 试跑模式（-all 全量导入），已停止")
			break
		}
		time.Sleep(sleepBetween)
	}

	fmt.Printf("==> 完成: 导入 %d / 跳过 %d / 失败 %d\n", imported, skipped, failed)
}
