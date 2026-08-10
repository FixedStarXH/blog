package service

import (
	"reflect"
	"strings"
	"testing"

	"blog-system/config"
)

// init 设置测试配置（ConsumeAIQuota 等需要读 config.AppConfig）
// 每日上限设 0 = 不限额：验证"未配置限额直接放行"的降级分支
func init() {
	config.AppConfig = &config.Config{
		AI: config.AIConfig{MaxDailyAsks: 0},
	}
}

// ----------------------------------------------------------------------------
// RAG 降级链核心：切块 / 关键词 / 截断 / 摘要 / 余弦 / 脱HTML / 向量编解码
// ----------------------------------------------------------------------------

func TestChunkTextBasic(t *testing.T) {
	// 多段短文：每段独立成块
	chunks := chunkText("第一段。\n第二段。\n第三段。")
	if len(chunks) != 3 {
		t.Fatalf("3 个段落应切成 3 块，实际 %d 块: %v", len(chunks), chunks)
	}
}

func TestChunkTextLongPara(t *testing.T) {
	// 单段 1200 字符：先按 500 硬切，尾块带 50 字重叠
	// 推导：第1块 0-499、第2块 450-949（前50字与第1块重叠）、第3块 950-1199
	long := strings.Repeat("测", 1200)
	chunks := chunkText(long)
	t.Logf("chunkText(1200字) → %d 块, 长度: %v", len(chunks), lens(chunks))
	if len(chunks) != 3 {
		t.Fatalf("1200 字应切成 3 块，实际 %d 块", len(chunks))
	}
	// 注意：len() 返回字节数，中文 3 字节/字，必须按 rune 数比较
	if n0, n1 := len([]rune(chunks[0])), len([]rune(chunks[1])); n0 != chunkSize || n1 != chunkSize {
		t.Errorf("前两块长度应恰为 %d，实际 %d/%d", chunkSize, n0, n1)
	}
	// 重叠验证：第2块开头 50 字 == 第1块结尾 50 字（切缝处语义不割断）
	r0, r1 := []rune(chunks[0]), []rune(chunks[1])
	if string(r1[:chunkOverlap]) != string(r0[len(r0)-chunkOverlap:]) {
		t.Error("相邻块缺少重叠：切缝处语义可能被割断")
	}
}

func TestExtractKeywords(t *testing.T) {
	kw := extractKeywords("Go 并发中的 Context 有什么作用？")
	// "Go"、"Context" 保留；中文按词保留（"并发中的"是连续汉字字段）
	for _, want := range []string{"Go", "Context", "并发中的"} {
		if !contains(kw, want) {
			t.Errorf("关键词应包含 %q，实际 %v", want, kw)
		}
	}
}

func TestExtractKeywordsDedup(t *testing.T) {
	kw := extractKeywords("锁 锁 并发 并发")
	// 去重：每个关键词只出现一次
	seen := map[string]bool{}
	for _, k := range kw {
		if seen[k] {
			t.Errorf("关键词重复: %v", kw)
		}
		seen[k] = true
	}
}

func TestExtractKeywordsDropSingleRune(t *testing.T) {
	// 单字太泛（"a"、"b"）直接丢弃 → 空结果
	if kw := extractKeywords("a b c"); len(kw) != 0 {
		t.Errorf("全单字应被丢弃，实际 %v", kw)
	}
}

func TestTruncateRunes(t *testing.T) {
	// 中文按字符截断：不能截出半个字符
	s := "一二三四五六七八九十"
	if got := truncateRunes(s, 5); got != "一二三四五" {
		t.Errorf("截 5 字应得 %q，实际 %q", "一二三四五", got)
	}
	// 短文本不截
	if got := truncateRunes(s, 100); got != s {
		t.Errorf("短于上限不应截断，实际 %q", got)
	}
	// 空字符串
	if got := truncateRunes("", 5); got != "" {
		t.Errorf("空字符串应返回空，实际 %q", got)
	}
}

func TestFallbackSummary(t *testing.T) {
	// 多段文本：按自然段拼接凑 120 字，段落必须完整
	paras := []string{
		strings.Repeat("段一", 30), // 60 字
		strings.Repeat("段二", 30), // 60 字（拼接后 120 达标）
		strings.Repeat("段三", 30), // 不该被纳入（已凑够）
	}
	sum := fallbackSummary(strings.Join(paras, "\n"))
	if !strings.Contains(sum, "段二") {
		t.Errorf("摘要应拼入第 2 段，实际: %q", sum)
	}
	if strings.Contains(sum, "段三") {
		t.Errorf("凑够 120 字后不应再拼第 3 段，实际: %q", sum)
	}
}

func TestFallbackSummaryEmpty(t *testing.T) {
	if got := fallbackSummary("  \n  "); got != "" {
		t.Errorf("空白文本应返回空摘要，实际 %q", got)
	}
}

func TestCosine(t *testing.T) {
	// 相同向量 → 1
	if c := cosine([]float32{1, 0, 0}, []float32{1, 0, 0}); c != 1 {
		t.Errorf("相同向量余弦应为 1，实际 %v", c)
	}
	// 正交 → 0
	if c := cosine([]float32{1, 0}, []float32{0, 1}); c != 0 {
		t.Errorf("正交向量余弦应为 0，实际 %v", c)
	}
	// 长度不等 → 0（防御）
	if c := cosine([]float32{1}, []float32{1, 2}); c != 0 {
		t.Errorf("维度不同应返回 0，实际 %v", c)
	}
	// 零向量 → 0（防除零）
	if c := cosine([]float32{0, 0}, []float32{1, 0}); c != 0 {
		t.Errorf("零向量应返回 0，实际 %v", c)
	}
}

func TestStripHTML(t *testing.T) {
	got := stripHTML("<p>第一段</p><p>第二段<br>换行</p>")
	// 块级结束标签（</p>）→ 换行，段落结构保留
	if !strings.Contains(got, "第一段\n") || !strings.Contains(got, "第二段") {
		t.Errorf("段落应保留换行结构，实际: %q", got)
	}
	// <br> → 换行（自闭合标签修复的核心）
	if !strings.Contains(got, "\n换行") {
		t.Errorf("<br> 应转成换行，实际: %q", got)
	}
}

func TestVectorRoundTrip(t *testing.T) {
	// 向量 编码 → 解码 应无损还原（RAG 索引落库/读取的关键）
	v := []float32{0.123, -1.456, 7.89, 0}
	got := decodeVector(encodeVector(v))
	if !reflect.DeepEqual(v, got) {
		t.Errorf("向量编解码应无损还原: 原 %v, 得 %v", v, got)
	}
}

func TestConsumeAIQuotaDegrade(t *testing.T) {
	// 未配置上限（MaxDailyAsks=0）：必须直接放行，不能卡死功能
	// （cache.Enabled 在测试环境为 false，天然模拟"Redis 不可用"）
	s := &AIService{}
	if ok, limit := s.ConsumeAIQuota(); !ok || limit != 0 {
		t.Errorf("未配置限额时应放行（limit=0），实际 ok=%v limit=%d", ok, limit)
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// lens 调试辅助：返回字符串切片中各元素的字符长度
func lens(list []string) []int {
	out := make([]int, 0, len(list))
	for _, v := range list {
		out = append(out, len([]rune(v)))
	}
	return out
}
