package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"blog-system/config"

	"gorm.io/gorm"
)

// AIService AI 能力服务：摘要 / 润色 / RAG 问答
//
// 设计要点：
//  1. 只依赖标准库 http + JSON，不引入 OpenAI SDK —— 接口协议简单，自己实现更可控、更好讲
//  2. 兼容 OpenAI 格式（/v1/chat/completions、/v1/embeddings），换服务商只改配置
//  3. 全链路降级：AI 不可用/超时/报错时，摘要回退"截取首段"，问答返回友好提示，功能不挂
type AIService struct {
	db     *gorm.DB
	client *http.Client
}

func NewAIService(db *gorm.DB) *AIService {
	secs := config.AppConfig.AI.TimeoutSecs
	if secs <= 0 {
		secs = 30
	}
	return &AIService{
		db:     db,
		client: &http.Client{Timeout: time.Duration(secs) * time.Second},
	}
}

// Enabled AI 是否可用：配置开启 且 有 API key
func (s *AIService) Enabled() bool {
	cfg := config.AppConfig.AI
	return cfg.Enabled && cfg.APIKey != "" && cfg.BaseURL != ""
}

// ----------------------------------------------------------------------------
// 底层：OpenAI 兼容 HTTP 调用（chat 流式 / chat 非流式 / embedding）
// ----------------------------------------------------------------------------

// ChatMessage 对话消息（导出：controller 构造多轮历史时使用）
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatCompletion 通用 chat 调用。stream=true 时返回响应体（调用方负责逐行读 SSE 并 Close）。
func (s *AIService) chatCompletion(ctx context.Context, messages []ChatMessage, temperature float64, stream bool) (*http.Response, error) {
	cfg := config.AppConfig.AI
	payload, err := json.Marshal(map[string]interface{}{
		"model":       cfg.ChatModel,
		"messages":    messages,
		"temperature": temperature,
		"stream":      stream,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(cfg.BaseURL, "/")+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		resp.Body.Close()
		return nil, fmt.Errorf("AI 接口返回 %d: %s", resp.StatusCode, string(body))
	}
	return resp, nil
}

// chat 非流式对话，返回模型完整回答（摘要等短输出场景用）
func (s *AIService) chat(ctx context.Context, messages []ChatMessage, temperature float64) (string, error) {
	resp, err := s.chatCompletion(ctx, messages, temperature, false)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // 上限 2MB，防异常大响应
	if err != nil {
		return "", err
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("AI 返回为空")
	}
	return parsed.Choices[0].Message.Content, nil
}

// embed 文本向量化，返回 float32 向量
func (s *AIService) embed(ctx context.Context, text string) ([]float32, error) {
	cfg := config.AppConfig.AI
	payload, err := json.Marshal(map[string]interface{}{
		"model": cfg.EmbedModel,
		"input": text,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(cfg.BaseURL, "/")+"/embeddings", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("embedding 接口返回 %d: %s", resp.StatusCode, string(body))
	}
	var parsed struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	if len(parsed.Data) == 0 {
		return nil, fmt.Errorf("embedding 返回为空")
	}
	return parsed.Data[0].Embedding, nil
}

// ----------------------------------------------------------------------------
// 业务能力一：AI 摘要（非流式）
// ----------------------------------------------------------------------------

// GenerateSummary 为文章正文生成摘要。
// 降级链：AI 可用 → 调大模型；不可用/失败 → 截取正文首段纯文本。
// 调用方（controller）负责把结果写回文章 summary 字段。
func (s *AIService) GenerateSummary(ctx context.Context, content string) (string, error) {
	text := stripHTML(content)
	text = truncateRunes(text, 3000) // 控制 token 成本：只喂正文前 3000 字

	if !s.Enabled() {
		return fallbackSummary(text), nil
	}
	messages := []ChatMessage{
		{Role: "system", Content: "你是一位技术博客编辑。请为文章写一段摘要：50-100字，客观、简洁、准确概括核心内容。直接输出摘要文本，不要加引号、不要加'摘要：'前缀。"},
		{Role: "user", Content: "请为以下文章生成摘要：\n\n" + text},
	}
	out, err := s.chat(ctx, messages, 0.3)
	if err != nil {
		return fallbackSummary(text), nil
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return fallbackSummary(text), nil
	}
	return out, nil
}

// ----------------------------------------------------------------------------
// 业务能力二：AI 润色（流式 SSE，编辑后台打字机效果）
// ----------------------------------------------------------------------------

// PolishStream 流式润色正文，返回 SSE 响应体（data: {"delta": "..."} 行）。
// 调用方（controller）负责把响应体接到 gin 的 SSE 输出并 Close。
func (s *AIService) PolishStream(ctx context.Context, content string) (*http.Response, error) {
	if !s.Enabled() {
		// 与摘要一致：未配置 API key 时给友好提示，而不是把上游 401 原文暴露给用户
		return nil, errors.New("AI 功能未配置：请在 config.yaml 设置 ai.api_key 后重启")
	}
	text := truncateRunes(stripHTML(content), 3000)
	messages := []ChatMessage{
		{Role: "system", Content: "你是一位资深技术文章编辑。对用户提供的文章内容进行润色：修正错别字、语病，让表达更简洁、专业、通顺；保持原有技术信息与结构不变。只输出润色后的内容，不要任何解释或前缀。"},
		{Role: "user", Content: text},
	}
	return s.chatCompletion(ctx, messages, 0.4, true)
}

// ----------------------------------------------------------------------------
// 工具函数
// ----------------------------------------------------------------------------

// stripHTML 去除 HTML 标签与多余空白，得到纯文本（喂大模型前必须脱 HTML）。
// 关键：块级元素结束标签保留为换行，保留段落结构 ——
// 降级摘要 fallbackSummary 依赖换行按"自然段"拼接，不能把段落压成一行。
var htmlTagRegex = regexp.MustCompile(`<[^>]+>`)
var htmlBlockRegex = regexp.MustCompile(`(?i)</(p|div|h[1-6]|li|ul|ol|blockquote|pre|table|tr|section|article|br)>`)
var htmlInlineSpaceRegex = regexp.MustCompile(`[^\S\n]+`) // 空白但排除换行 → 压成单空格
var htmlBlankLineRegex = regexp.MustCompile(`\n{2,}`)

func stripHTML(s string) string {
	// 先处理 <br>（自闭合，无结束标签），再给块级结束标签补换行
	s = htmlBlockRegex.ReplaceAllString(s, "\n")
	s = htmlTagRegex.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&amp;", "&")
	// 行内空白压成单空格，但保留换行
	s = htmlInlineSpaceRegex.ReplaceAllString(s, " ")
	// 连续空行压成一个（删空段）
	s = htmlBlankLineRegex.ReplaceAllString(s, "\n")
	return strings.TrimSpace(s)
}

// truncateRunes 按字符（非字节）截断，避免中文被截一半
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// fallbackSummary 降级摘要：AI 不可用时尽量给语义完整的摘要。
// 策略：按自然段拼接（stripHTML 后段落以换行分隔），凑满约 120 字即停，
// 保证每段完整（不会出现"一句话截一半 + …"的残缺感）；总长仍超 300 字再截断兜底。
func fallbackSummary(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	var b strings.Builder
	n := 0
	for _, p := range strings.Split(text, "\n") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(p)
		n += len([]rune(p))
		if n >= 120 {
			break
		}
	}
	r := []rune(b.String())
	if len(r) > 300 {
		return string(r[:300]) + "…"
	}
	return b.String()
}
