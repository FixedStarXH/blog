package utils

import (
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// HTMLToMarkdown 将文章正文 HTML 转换为 Markdown（供"导出 Markdown"下载）。
// 覆盖常见标签：标题/段落/加粗/斜体/删除线/行内代码/代码块/引用/列表/图片/链接/分割线/换行。
func HTMLToMarkdown(htmlStr string) string {
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return htmlStr // 解析失败原样返回，保证导出不丢内容
	}
	var b strings.Builder
	for c := doc.FirstChild; c != nil; c = c.NextSibling {
		convertMDNode(c, &b, 0)
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return out
	}
	return out + "\n"
}

// convertMDNode 递归转换 DOM 节点到 Markdown。preLevel>0 表示处于 <pre> 代码块内（文本原样输出）。
func convertMDNode(n *html.Node, b *strings.Builder, preLevel int) {
	if n.Type == html.TextNode {
		if preLevel > 0 {
			b.WriteString(n.Data)
		} else {
			b.WriteString(compactSpace(n.Data))
		}
		return
	}
	if n.Type != html.ElementNode {
		return
	}

	children := func() {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			convertMDNode(c, b, preLevel)
		}
	}
	// 块级元素前确保独占一行
	startLine := func() {
		s := b.String()
		if s != "" && !strings.HasSuffix(s, "\n") {
			b.WriteString("\n")
		}
	}

	switch n.Data {
	case "h1", "h2", "h3", "h4", "h5", "h6":
		startLine()
		b.WriteString(strings.Repeat("#", int(n.Data[1]-'0')) + " ")
		children()
		b.WriteString("\n\n")
	case "p":
		startLine()
		children()
		b.WriteString("\n\n")
	case "br":
		b.WriteString("  \n")
	case "strong", "b":
		b.WriteString("**")
		children()
		b.WriteString("**")
	case "em", "i":
		b.WriteString("*")
		children()
		b.WriteString("*")
	case "del", "s", "strike":
		b.WriteString("~~")
		children()
		b.WriteString("~~")
	case "code":
		if preLevel > 0 {
			children()
		} else {
			b.WriteString("`" + nodeText(n) + "`")
		}
	case "pre":
		startLine()
		lang := ""
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && c.Data == "code" {
				lang = classLanguage(c)
			}
		}
		b.WriteString("```" + lang + "\n")
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			convertMDNode(c, b, preLevel+1)
		}
		b.WriteString("\n```\n\n")
	case "blockquote":
		startLine()
		b.WriteString("> ")
		children()
		b.WriteString("\n\n")
	case "ul", "ol":
		startLine()
		convertMDList(n, b, n.Data == "ol", 0, preLevel)
		b.WriteString("\n")
	case "hr":
		startLine()
		b.WriteString("---\n\n")
	case "img":
		b.WriteString("![" + attr(n, "alt") + "](" + attr(n, "src") + ")")
	case "a":
		href := attr(n, "href")
		var t strings.Builder
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			convertMDNode(c, &t, preLevel)
		}
		text := strings.TrimSpace(t.String())
		if text == "" {
			text = href
		}
		b.WriteString("[" + text + "](" + href + ")")
	case "div", "section", "article", "figure", "figcaption", "table", "tbody", "tr", "td", "th", "span":
		startLine()
		children()
		b.WriteString("\n")
	default:
		children()
	}
}

// convertMDList 递归转换 ul/ol（支持嵌套，depth 控制缩进）
func convertMDList(n *html.Node, b *strings.Builder, ordered bool, depth, preLevel int) {
	idx := 1
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type != html.ElementNode {
			continue
		}
		switch c.Data {
		case "li":
			b.WriteString(strings.Repeat("  ", depth))
			if ordered {
				b.WriteString(strconv.Itoa(idx) + ". ")
			} else {
				b.WriteString("- ")
			}
			for cc := c.FirstChild; cc != nil; cc = cc.NextSibling {
				if cc.Type == html.ElementNode && (cc.Data == "ul" || cc.Data == "ol") {
					b.WriteString("\n")
					convertMDList(cc, b, cc.Data == "ol", depth+1, preLevel)
					continue
				}
				convertMDNode(cc, b, preLevel)
			}
			b.WriteString("\n")
			idx++
		case "ul", "ol":
			convertMDList(c, b, c.Data == "ol", depth, preLevel)
		}
	}
}

// nodeText 提取节点下所有纯文本（行内代码用）
func nodeText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(x *html.Node) {
		if x.Type == html.TextNode {
			b.WriteString(x.Data)
		}
		for c := x.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

// classLanguage 从 <code class="language-go"> 提取语言名
func classLanguage(n *html.Node) string {
	for _, k := range strings.Fields(attr(n, "class")) {
		if strings.HasPrefix(k, "language-") {
			return strings.TrimPrefix(k, "language-")
		}
	}
	return ""
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

// compactSpace 把连续空白压缩为单个空格（HTML 语义）
func compactSpace(s string) string {
	var b strings.Builder
	space := false
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			space = true
			continue
		}
		if space && b.Len() > 0 {
			b.WriteByte(' ')
		}
		space = false
		b.WriteRune(r)
	}
	return b.String()
}
