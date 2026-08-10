package controller

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"blog-system/model"
	"blog-system/service"
	"blog-system/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AIController AI 能力接口（摘要 / 润色 / 问答）
type AIController struct {
	svc *service.AIService
}

func NewAIController(svc *service.AIService) *AIController {
	return &AIController{svc: svc}
}

// GenerateSummary AI 生成摘要（后台编辑页"AI 摘要"按钮）
// POST /api/admin/ai/summary  Body: {"articleId": 1}
// 返回 {"summary": "..."}，是否采用由编辑确认（不自动改库）
func (c *AIController) GenerateSummary(ctx *gin.Context) {
	var req struct {
		ArticleID uint `json:"articleId" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.Fail(ctx, "参数错误：请传 articleId")
		return
	}
	var article model.Article
	if err := model.DB.Select("content").First(&article, req.ArticleID).Error; err != nil {
		utils.Fail(ctx, "文章不存在")
		return
	}
	summary, err := c.svc.GenerateSummary(ctx.Request.Context(), article.Content)
	if err != nil {
		utils.Fail(ctx, "摘要生成失败："+err.Error())
		return
	}
	utils.Success(ctx, gin.H{"summary": summary})
}

// GenerateSummaryByContent AI 生成摘要（登录用户：投稿/写文章页用，正文还没入库没有 articleId）
// POST /api/ai/summary  Body: {"content": "..."}
func (c *AIController) GenerateSummaryByContent(ctx *gin.Context) {
	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.Fail(ctx, "参数错误：内容不能为空")
		return
	}
	if len(req.Content) > 50000 { // 防超大正文刷接口（正常文章几万字符封顶）
		utils.Fail(ctx, "内容过长")
		return
	}
	summary, err := c.svc.GenerateSummary(ctx.Request.Context(), req.Content)
	if err != nil {
		utils.Fail(ctx, "摘要生成失败："+err.Error())
		return
	}
	utils.Success(ctx, gin.H{"summary": summary})
}

// SummarizeArticle 前台文章"一键总结本文"（公开接口：游客也能用）
// POST /api/articles/:id/summary
// 只允许已发布文章；严格限流在路由层（AI 每次调用都花钱，防刷接口烧额度）
func (c *AIController) SummarizeArticle(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		utils.Fail(ctx, "无效的文章ID")
		return
	}
	var article model.Article
	if err := model.DB.Select("content", "summary").Where("status = ?", model.ArticleStatusPublished).First(&article, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.Fail(ctx, "文章不存在或未发布")
			return
		}
		utils.Error(ctx, "获取文章失败")
		return
	}
	summary, err := c.svc.GenerateSummary(ctx.Request.Context(), article.Content)
	if err != nil {
		utils.Fail(ctx, "总结失败："+err.Error())
		return
	}
	// AI 未配置时 GenerateSummary 内部会降级"按段落拼摘要"；若作者写过简介（summary 字段），
	// 优先用简介——它是作者亲手写的完整概述，比任何自动截断都更准
	if !c.svc.Enabled() && article.Summary != "" {
		summary = article.Summary
	}
	utils.Success(ctx, gin.H{"summary": summary})
}

// Polish AI 润色（后台编辑页"AI 润色"按钮，SSE 流式打字机效果）
// POST /api/admin/ai/polish  Body: {"content": "..."}
func (c *AIController) Polish(ctx *gin.Context) {
	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.Fail(ctx, "参数错误：内容不能为空")
		return
	}
	body, err := c.svc.PolishStream(ctx.Request.Context(), req.Content)
	if err != nil {
		utils.Fail(ctx, "润色失败："+err.Error())
		return
	}
	streamSSE(ctx, body)
}

// IndexArticle 为单篇文章建立 RAG 索引
// POST /api/admin/ai/index/:id
func (c *AIController) IndexArticle(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		utils.Fail(ctx, "无效的文章ID")
		return
	}
	n, err := c.svc.IndexArticle(ctx.Request.Context(), uint(id))
	if err != nil {
		utils.Fail(ctx, "索引失败："+err.Error())
		return
	}
	utils.Success(ctx, gin.H{"chunks": n})
}

// IndexAll 重建全部已发布文章索引（后台"一键索引"）
// POST /api/admin/ai/index-all
func (c *AIController) IndexAll(ctx *gin.Context) {
	n, failed, err := c.svc.ReindexAll(ctx.Request.Context())
	if err != nil {
		utils.Fail(ctx, "索引失败："+err.Error())
		return
	}
	// 全部失败（如 AI 未配置 / embedding 接口异常）→ 明确报错，不让前端以为成功了
	if len(failed) > 0 && n == 0 {
		utils.Fail(ctx, "索引失败：AI 服务未配置或向量化出错，请检查 config.yaml 的 ai.api_key")
		return
	}
	utils.Success(ctx, gin.H{"chunks": n, "failed": failed})
}

// IndexStatus 已索引文章数（后台展示"AI 问答可用性"）
// GET /api/admin/ai/index-status
func (c *AIController) IndexStatus(ctx *gin.Context) {
	n, err := c.svc.IndexedCount()
	if err != nil {
		utils.Error(ctx, "查询索引状态失败")
		return
	}
	utils.Success(ctx, gin.H{
		"indexedArticles": n,
		"enabled":         c.svc.Enabled(),
	})
}

// Ask 智能问答（前台公开接口，SSE 流式，支持多轮上下文）
// POST /api/ai/ask  Body: {"question":"...", "articleId": 可选(限定单篇文章), "history": [{"role":"user|assistant","content":"..."}]}
func (c *AIController) Ask(ctx *gin.Context) {
	var req struct {
		Question  string                `json:"question" binding:"required"`
		ArticleID *uint                 `json:"articleId"`
		History   []service.ChatMessage `json:"history"`
	}
	if err := ctx.ShouldBindJSON(&req); err != nil {
		utils.Fail(ctx, "参数错误：问题不能为空")
		return
	}
	// 历史消息安全校验：只收 user/assistant、限 8 条、每条 ≤ 500 字、总数 ≤ 2000 字
	// （防注入 system 角色、防超长 history 刷 token）
	history := make([]service.ChatMessage, 0, len(req.History))
	total := 0
	for _, m := range req.History {
		role := strings.ToLower(strings.TrimSpace(m.Role))
		if role != "user" && role != "assistant" {
			continue
		}
		content := strings.TrimSpace(m.Content)
		if content == "" || len([]rune(content)) > 500 {
			continue
		}
		total += len([]rune(content))
		if total > 2000 {
			break
		}
		if len(history) >= 8 {
			break
		}
		history = append(history, service.ChatMessage{Role: role, Content: content})
	}
	// AI 每次调用都消耗 token 额度：先检查每日问答上限（超限当天直接拒绝）
	// 未配置限额或 Redis 不可用时 ConsumeAskQuota 自动放行
	if ok, limit := c.svc.ConsumeAskQuota(); !ok {
		utils.Fail(ctx, fmt.Sprintf("今日 AI 问答次数已达上限（%d 次），明天再来吧", limit))
		return
	}
	body, err := c.svc.AskStream(ctx.Request.Context(), req.Question, req.ArticleID, history)
	if err != nil {
		utils.Fail(ctx, err.Error())
		return
	}
	streamSSE(ctx, body)
}

// ----------------------------------------------------------------------------
// SSE 流式转发：上游 OpenAI 格式 → 统一 {delta} 格式
//
// 上游每行：data: {"choices":[{"delta":{"content":"文字"}}]}
// 转发给前端：data: {"delta":"文字"}，结尾 data: [DONE]
// 这样前端只需一个 ReadableStream 解析函数，不用关心上游格式
// ----------------------------------------------------------------------------
func streamSSE(ctx *gin.Context, body *http.Response) {
	defer body.Body.Close()

	ctx.Header("Content-Type", "text/event-stream; charset=utf-8")
	ctx.Header("Cache-Control", "no-cache")
	ctx.Header("Connection", "keep-alive")
	ctx.Header("X-Accel-Buffering", "no") // 禁用 nginx 缓冲，保证实时
	flusher, ok := ctx.Writer.(http.Flusher)
	if !ok {
		return
	}

	scanner := bufio.NewScanner(body.Body)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 默认 64KB 限制对长响应不够，放宽到 1MB
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			raw, _ := json.Marshal(gin.H{"delta": chunk.Choices[0].Delta.Content})
			fmt.Fprintf(ctx.Writer, "data: %s\n\n", raw)
			flusher.Flush()
		}
	}
	// 结束标记
	fmt.Fprint(ctx.Writer, "data: [DONE]\n\n")
	flusher.Flush()
}
