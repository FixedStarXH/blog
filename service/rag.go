package service

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"unicode"

	"blog-system/model"
)

// ----------------------------------------------------------------------------
// RAG（Retrieval-Augmented Generation）：基于文章内容的智能问答
//
// 流程（三步）：
//  1. 索引：文章正文 → 切成 ~500 字的小块 → 每块调 embedding 接口向量化 → 存 article_chunks 表
//  2. 检索：用户提问 → 问题向量化 → 与库里所有块做余弦相似度 → 取 top-k 最相关的块
//  3. 生成：把命中的块拼进 prompt → 大模型流式回答（回答有依据，可溯源到具体文章）
//
// 不用向量数据库的原因：文章量级（几百篇）下，全量加载算余弦毫秒级完成，
// 一张表 + 内存计算即可，少一个中间件、好部署好讲。
// ----------------------------------------------------------------------------

// 切块参数：中文每块目标长度与重叠量
const (
	chunkSize    = 500 // 目标字符数（rune）
	chunkOverlap = 50  // 前后块重叠字符（避免语义在切缝处被割断）
)

// chunkText 把纯文本切成块（按段落优先，段太长再按长度硬切）
func chunkText(text string) []string {
	paras := strings.Split(text, "\n")
	var chunks []string
	var cur []rune
	flush := func() {
		if len(cur) == 0 {
			return
		}
		chunks = append(chunks, strings.TrimSpace(string(cur)))
		cur = nil
	}
	for _, p := range paras {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		runes := []rune(p)
		// 段本身就很长：先切掉能并进当前块的部分，再整段硬切
		for len(runes) > 0 {
			room := chunkSize - len(cur)
			if room >= len(runes) {
				cur = append(cur, runes...)
				runes = nil
			} else if room > chunkOverlap {
				// 当前块还有空间：补一段，然后 flush
				cur = append(cur, runes[:room]...)
				runes = runes[room:]
				flush()
			} else {
				flush()
			}
			// 每次 flush 后，新块从"重叠尾巴"开始（保留上一块结尾 chunkOverlap 字）
			if len(cur) == 0 && len(runes) > chunkSize {
				cur = append(cur, runes[:chunkOverlap]...)
				runes = runes[chunkOverlap:]
			}
		}
		flush()
	}
	flush()
	return chunks
}

// ----------------------------- 向量序列化 -----------------------------

// encodeVector float32 向量 → 二进制 blob（顺序写入，MySQL blob 列）
func encodeVector(v []float32) []byte {
	buf := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}

// decodeVector blob → float32 向量
func decodeVector(b []byte) []float32 {
	n := len(b) / 4
	v := make([]float32, n)
	for i := 0; i < n; i++ {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return v
}

// cosine 余弦相似度（-1..1，越大越相关；向量零长度返回 0）
func cosine(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// ----------------------------- 索引 -----------------------------

// IndexArticle 为单篇文章建立/重建索引：
// 先删旧块（文章更新后内容变了，旧向量失效），再切块、向量化、入库。
// 返回建了多少块；失败时已删的旧块保持删除（下次重建会补齐）。
func (s *AIService) IndexArticle(ctx context.Context, articleID uint) (int, error) {
	if !s.Enabled() {
		return 0, fmt.Errorf("AI 服务未配置（请在 config.yaml 填写 ai.api_key 或设置环境变量 BLOG_AI_API_KEY）")
	}
	var article model.Article
	if err := s.db.Select("id", "content").First(&article, articleID).Error; err != nil {
		return 0, err
	}
	text := stripHTML(article.Content)
	text = strings.ReplaceAll(text, " ", "\n") // 纯文本按句断开，让段落切分更贴合语义
	chunks := chunkText(text)
	if len(chunks) == 0 {
		return 0, nil
	}

	// 旧块直接硬删（派生数据，重建即重写）
	if err := s.db.Where("article_id = ?", articleID).Delete(&model.ArticleChunk{}).Error; err != nil {
		return 0, err
	}

	// 逐块向量化入库（小批量逐条，简单可靠）
	// 失败任一块 → 整个索引操作失败，由调用方决定是否回滚提示
	rows := make([]model.ArticleChunk, 0, len(chunks))
	for i, c := range chunks {
		vec, err := s.embed(ctx, c)
		if err != nil {
			return 0, fmt.Errorf("第 %d 块向量化失败: %w", i+1, err)
		}
		rows = append(rows, model.ArticleChunk{
			ArticleID:  articleID,
			ChunkIndex: i,
			Content:    c,
			Embedding:  encodeVector(vec),
		})
	}
	if err := s.db.Create(&rows).Error; err != nil {
		return 0, err
	}
	return len(rows), nil
}

// ReindexAll 重建全部已发布文章的索引（后台"一键索引"）
// 逐篇调用 IndexArticle，返回成功/失败统计。
func (s *AIService) ReindexAll(ctx context.Context) (indexed int, failed []uint, err error) {
	var ids []uint
	if err := s.db.Model(&model.Article{}).
		Where("status = ?", model.ArticleStatusPublished).
		Pluck("id", &ids).Error; err != nil {
		return 0, nil, err
	}
	for _, id := range ids {
		n, err := s.IndexArticle(ctx, id)
		if err != nil {
			failed = append(failed, id)
			continue
		}
		indexed += n
	}
	return indexed, failed, nil
}

// IndexedCount 已索引的文章数（后台展示用）
func (s *AIService) IndexedCount() (int64, error) {
	var n int64
	err := s.db.Model(&model.ArticleChunk{}).Distinct("article_id").Count(&n).Error
	return n, err
}

// ----------------------------- 检索 + 生成 -----------------------------

// 降级"全文模式"：服务商不支持 embedding（如 DeepSeek 官方无向量接口）时，
// 问答跳过向量检索，直接把文章全文交给大模型回答。
//   - 单篇文章问答（articleID 非 nil）：用该文章正文
//   - 全局问答（articleID nil）：按问题关键词匹配最相关的 3 篇
const fullTextMaxRunes = 6000 // 每篇截断字符数（控制 token 与响应速度）

// extractKeywords 从问题中提取关键词：按非字母数字切分，丢弃过短词（单字太泛）
func extractKeywords(q string) []string {
	fields := strings.FieldsFunc(q, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r))
	})
	seen := make(map[string]bool)
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if len([]rune(f)) < 2 || seen[f] {
			continue
		}
		seen[f] = true
		out = append(out, f)
	}
	return out
}

// fullTextContext 全文模式的上下文：单篇用全文；全局按关键词匹配最相关的文章
func (s *AIService) fullTextContext(ctx context.Context, question string, articleID *uint) (string, error) {
	if articleID != nil {
		var a model.Article
		if err := s.db.Select("id", "title", "content").First(&a, *articleID).Error; err != nil {
			return "", err
		}
		return fmt.Sprintf("【来自文章《%s》】\n%s", a.Title, truncateRunes(stripHTML(a.Content), fullTextMaxRunes)), nil
	}

	// 全局：关键词命中数决定文章优先级，取 top3
	kw := extractKeywords(question)
	var arts []model.Article
	if err := s.db.Select("id", "title", "content").
		Where("status = ?", model.ArticleStatusPublished).
		Find(&arts).Error; err != nil {
		return "", err
	}
	type cand struct {
		title string
		text  string
		score int
	}
	var cands []cand
	for _, a := range arts {
		text := stripHTML(a.Content)
		score := 0
		for _, k := range kw {
			if strings.Contains(a.Title, k) {
				score += 3 // 标题命中权重更高
			}
			if strings.Contains(text, k) {
				score++
			}
		}
		if score > 0 {
			cands = append(cands, cand{a.Title, text, score})
		}
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].score > cands[j].score })
	if len(cands) > 3 {
		cands = cands[:3]
	}
	if len(cands) == 0 {
		return "", fmt.Errorf("没有在文章中找到相关内容，换个问法试试")
	}
	parts := make([]string, 0, len(cands))
	for _, c := range cands {
		parts = append(parts, fmt.Sprintf("【来自文章《%s》】\n%s", c.title, truncateRunes(c.text, fullTextMaxRunes)))
	}
	return strings.Join(parts, "\n\n---\n\n"), nil
}

// ragSystemPrompt 问答系统提示词（检索模式与全文模式共用）
func ragSystemPrompt() ChatMessage {
	return ChatMessage{Role: "system", Content: "你是「Lumi 博客」的 AI 助手。请基于提供的文章内容回答用户问题：\n1. 回答要有依据，优先引用文章内容；\n2. 文章中没有相关内容时明确说'文章中没有相关内容'，不要编造；\n3. 用简洁的中文回答。"}
}

// topHit 一条检索命中的块（带相似度，用于排序与提示"依据哪篇文章"）
type topHit struct {
	Chunk     model.ArticleChunk
	ArticleID uint
	Score     float64
}

// search 问题向量化后，在库里检索最相似的 topK 个块
func (s *AIService) search(ctx context.Context, qVec []float32, articleID *uint, topK int) ([]topHit, error) {
	query := s.db.Model(&model.ArticleChunk{})
	if articleID != nil {
		query = query.Where("article_id = ?", *articleID)
	}
	var chunks []model.ArticleChunk
	if err := query.Find(&chunks).Error; err != nil {
		return nil, err
	}

	hits := make([]topHit, 0, len(chunks))
	for _, c := range chunks {
		v := decodeVector(c.Embedding)
		score := cosine(qVec, v)
		if score > 0.25 { // 阈值：相关性过低的不算命中（防答非所问）
			hits = append(hits, topHit{Chunk: c, ArticleID: c.ArticleID, Score: score})
		}
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > topK {
		hits = hits[:topK]
	}
	return hits, nil
}

// AskStream 智能问答（流式）：检索命中块 → 拼 prompt → 大模型流式回答。
//
// articleID 非 nil 时只在单篇文章内检索（文章详情页"问这篇文章"）；
// nil 时全站检索（全局 AI 助手）。
// history 为多轮对话历史（controller 已校验 role/content/条数），插在问题之前，
// 让模型能承接上文（"完整对话界面"的关键）。
// 返回 SSE 响应体；AI 不可用/未索引时返回明确的业务错误（前端给提示）。
func (s *AIService) AskStream(ctx context.Context, question string, articleID *uint, history []ChatMessage) (*http.Response, error) {
	question = strings.TrimSpace(question)
	if question == "" {
		return nil, fmt.Errorf("问题不能为空")
	}
	if !s.Enabled() {
		return nil, fmt.Errorf("AI 服务未配置（请在 config.yaml 填写 ai.api_key 或设置环境变量 BLOG_AI_API_KEY）")
	}

	// ① 问题向量化；服务商不支持 embedding（如 DeepSeek 官方无向量接口）时自动降级
	qVec, err := s.embed(ctx, question)
	if err != nil {
		// 降级"全文模式"：跳过向量检索，把文章全文交给大模型直接回答
		contextText, ctxErr := s.fullTextContext(ctx, question, articleID)
		if ctxErr != nil {
			return nil, ctxErr
		}
		messages := []ChatMessage{ragSystemPrompt()}
		messages = append(messages, history...)
		messages = append(messages, ChatMessage{Role: "user", Content: fmt.Sprintf("以下是文章内容：\n\n%s\n\n---\n\n用户问题：%s", contextText, question)})
		return s.chatCompletion(ctx, messages, 0.3, true)
	}

	// ② 检索 top-5 相关块
	hits, err := s.search(ctx, qVec, articleID, 5)
	if err != nil {
		return nil, err
	}
	if len(hits) == 0 {
		return nil, fmt.Errorf("没有在文章中找到相关内容，换个问法试试")
	}

	// ③ 拼 prompt：按相关度倒序拼接命中块，附上文章标题作为来源
	var contextParts []string
	for _, h := range hits {
		title := "未知文章"
		s.db.Model(&model.Article{}).Where("id = ?", h.ArticleID).Select("title").Scan(&title)
		contextParts = append(contextParts, fmt.Sprintf("【来自文章《%s》】\n%s", title, h.Chunk.Content))
	}
	contextText := strings.Join(contextParts, "\n\n---\n\n")

	messages := []ChatMessage{ragSystemPrompt()}
	// ④ 多轮历史（最多 8 条，controller 已限）插在问题之前，保证上下文连贯
	messages = append(messages, history...)
	messages = append(messages, ChatMessage{Role: "user", Content: fmt.Sprintf("以下是相关文章片段：\n\n%s\n\n---\n\n用户问题：%s", contextText, question)})

	// ⑤ 大模型流式回答（SSE）
	return s.chatCompletion(ctx, messages, 0.3, true)
}
