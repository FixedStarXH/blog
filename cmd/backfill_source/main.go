// 命令：go run ./cmd/backfill_source
//
// 功能：为已导入的 CSDN 文章回填原文链接（source_url）。
//
// 背景：早期导入脚本没存 CSDN 原文地址，导致文章内裂图兜底只能跳博客主页。
// 本脚本按"标题"匹配：抓 CSDN RSS 源（标题→原文URL）↔ 拉本站后台文章（标题→id），
// 相同标题的文章用后台更新接口写入 sourceUrl。幂等：已有原文链接的自动跳过。
//
// 为什么用 RSS 而不是爬列表页：CSDN 列表页连续请求会触发 Cloudflare 521 反爬；
// RSS 源一次请求返回全部文章的标题+原文链接，请求少、几乎不会被拦截。
//
// 用法：
//
//	go run ./cmd/backfill_source            # 回填（每篇间隔 500ms，防止后端压力过大）
//	go run ./cmd/backfill_source -dry       # 只列出将要更新的，不真正修改
package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ==================== 配置 ====================
var (
	baseURL  = flag.String("base", "http://localhost:8080", "博客后端地址")
	csdnUser = flag.String("user", "FixedstarXH", "CSDN 用户名")
	blogUser = flag.String("login-user", "Lumi", "博客后台账号")
	blogPass = flag.String("login-pass", "123456", "博客后台密码")
	dryRun   = flag.Bool("dry", false, "true=只打印将更新的文章，不真正修改")
)

const userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

var httpClient = &http.Client{Timeout: 30 * time.Second}

// RSS item：CSDN 订阅源每条 <item> 含标题和原文链接
type rssItem struct {
	Title string `xml:"title"`
	Link  string `xml:"link"`
}

type rssFeed struct {
	Channel struct {
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
}

type articleMeta struct {
	Title string
	URL   string
}

type apiResp struct {
	Code int             `json:"code"`
	Msg  string          `json:"message"`
	Data json.RawMessage `json:"data"`
}

// ==================== 通用请求 ====================

func fetchHTML(url string) (string, error) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", "https://blog.csdn.net/")
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	return string(b), nil
}

func getJSON(url, token string) (*apiResp, error) {
	req, _ := http.NewRequest("GET", url, nil)
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

func putJSON(url, token string, body any) (*apiResp, error) {
	buf, _ := json.Marshal(body)
	req, _ := http.NewRequest("PUT", url, bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+token)
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

func login() (string, error) {
	buf, _ := json.Marshal(map[string]string{"username": *blogUser, "password": *blogPass})
	resp, err := httpClient.Post(*baseURL+"/api/admin/login", "application/json", bytes.NewReader(buf))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var r apiResp
	json.NewDecoder(resp.Body).Decode(&r)
	if r.Code != 200 {
		return "", fmt.Errorf("登录失败(%d): %s", r.Code, r.Msg)
	}
	var data struct {
		Token string `json:"token"`
	}
	json.Unmarshal(r.Data, &data)
	return data.Token, nil
}

// ==================== 抓 CSDN RSS（标题→原文URL） ====================

func fetchCsdnArticles() ([]articleMeta, error) {
	body, err := fetchHTML(fmt.Sprintf("https://blog.csdn.net/%s/rss/list", *csdnUser))
	if err != nil {
		return nil, err
	}
	var feed rssFeed
	if err := xml.Unmarshal([]byte(body), &feed); err != nil {
		return nil, err
	}
	metas := make([]articleMeta, 0, len(feed.Channel.Items))
	for _, it := range feed.Channel.Items {
		if it.Title == "" || it.Link == "" {
			continue
		}
		metas = append(metas, articleMeta{Title: it.Title, URL: it.Link})
	}
	return metas, nil
}

// ==================== 拉本站后台文章（标题→id / sourceUrl） ====================

type localArticle struct {
	ID        uint   `json:"id"`
	Title     string `json:"title"`
	SourceURL string `json:"sourceUrl"`
}

func fetchLocalArticles(token string) ([]localArticle, error) {
	const pageSize = 10 // 后台列表接口 pageSize 上限为 10
	var list []localArticle
	for page := 1; ; page++ {
		r, err := getJSON(fmt.Sprintf("%s/api/admin/articles?page=%d&pageSize=%d", *baseURL, page, pageSize), token)
		if err != nil {
			return nil, err
		}
		if r.Code != 200 {
			return nil, fmt.Errorf("拉文章列表失败(%d): %s", r.Code, r.Msg)
		}
		var data struct {
			Total int            `json:"total"`
			List  []localArticle `json:"list"`
		}
		if err := json.Unmarshal(r.Data, &data); err != nil {
			return nil, err
		}
		list = append(list, data.List...)
		if page*pageSize >= int(data.Total) {
			break
		}
	}
	return list, nil
}

// ==================== 主流程 ====================

func main() {
	flag.Parse()

	token, err := login()
	if err != nil {
		fmt.Println("[失败]", err)
		return
	}
	fmt.Printf("==> 后台登录成功: %s\n", *blogUser)

	fmt.Println("==> 拉取 CSDN 文章列表…")
	csdn, err := fetchCsdnArticles()
	if err != nil {
		fmt.Println("[失败] 抓 CSDN:", err)
		return
	}
	fmt.Printf("==> CSDN 共 %d 篇\n", len(csdn))

	local, err := fetchLocalArticles(token)
	if err != nil {
		fmt.Println("[失败] 拉本地文章:", err)
		return
	}
	fmt.Printf("==> 本站共 %d 篇\n", len(local))

	// 标题→CSDN URL 映射
	urlByTitle := make(map[string]string, len(csdn))
	for _, m := range csdn {
		if m.Title != "" {
			urlByTitle[m.Title] = m.URL
		}
	}

	updated, skipped, missing := 0, 0, 0
	for _, a := range local {
		url, ok := urlByTitle[a.Title]
		if !ok {
			missing++
			continue
		}
		if a.SourceURL != "" {
			skipped++
			continue
		}
		fmt.Printf("  [%d] %s\n      -> %s\n", a.ID, a.Title, url)
		if *dryRun {
			continue
		}
		// 后台更新接口需要完整字段：先拿详情再原样回填 + sourceUrl
		detail, err := getJSON(fmt.Sprintf("%s/api/admin/articles/%d", *baseURL, a.ID), token)
		if err != nil || detail.Code != 200 {
			fmt.Printf("      ! 拉详情失败: %v\n", err)
			continue
		}
		var d struct {
			Title      string `json:"title"`
			Summary    string `json:"summary"`
			Content    string `json:"content"`
			CoverImage string `json:"coverImage"`
			Status     int    `json:"status"`
			IsTop      bool   `json:"isTop"`
			Password   string `json:"password"`
			CategoryID uint   `json:"categoryId"`
			PublishAt  any    `json:"publishAt"`
		}
		if err := json.Unmarshal(detail.Data, &d); err != nil {
			fmt.Printf("      ! 解析详情失败: %v\n", err)
			continue
		}
		body := map[string]any{
			"title":      d.Title,
			"summary":    d.Summary,
			"content":    d.Content,
			"coverImage": d.CoverImage,
			"status":     d.Status,
			"isTop":      d.IsTop,
			"password":   d.Password,
			"categoryId": d.CategoryID,
			"publishAt":  d.PublishAt,
			"tagIds":     []uint{},
			"sourceUrl":  url,
		}
		r, err := putJSON(fmt.Sprintf("%s/api/admin/articles/%d", *baseURL, a.ID), token, body)
		if err != nil || r.Code != 200 {
			fmt.Printf("      ! 更新失败: %v (%s)\n", err, r.Msg)
			continue
		}
		fmt.Printf("      + 已回填原文链接\n")
		updated++
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Printf("==> 完成: 回填 %d / 已有跳过 %d / 本站无对应CSDN %d\n", updated, skipped, missing)
	if *dryRun {
		fmt.Println("==> 试跑模式（未做任何修改），去掉 -dry 执行真实回填")
	}
}
