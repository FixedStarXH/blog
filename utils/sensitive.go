package utils

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// sensitiveWords 敏感词库：评论/昵称命中后替换为等长 * 掩码。
// 可按需增删，注意避免收录易误伤日常用语的词。
var sensitiveWords = []string{
	// 辱骂人身类
	"傻逼", "傻比", "傻b", "脑残", "白痴", "弱智", "贱人", "婊子", "妈的",
	"操你妈", "去你妈", "你妈逼", "卧槽你妈", "草泥马", "滚蛋", "去死", "死全家",
	"废物", "垃圾货", "sb", "tmd", "nmd", "fuck", "shit",
	// 广告营销类
	"加微信", "加我微信", "微信号", "加vx", "威信号", "加qq", "加群", "私聊",
	"代刷", "刷单", "兼职", "转账", "汇款",
	// 赌博色情类
	"赌博", "博彩", "下注", "赌球", "色情", "裸聊", "约炮", "一夜情",
	// 政治敏感类（示例，可按需调整）
	"法轮功", "六四", "天安门事件", "台独", "藏独", "疆独", "港独",
}

// SensitiveWords 返回敏感词库副本（供接口输出给前端做提交前即时检测）
func SensitiveWords() []string {
	out := make([]string, len(sensitiveWords))
	copy(out, sensitiveWords)
	return out
}

// NormalizeSensitive 归一化：转小写并去掉所有非字母/数字字符。
// 用于防变体绕过检测（如 "傻 逼"、"F u c k"），中文按字母保留。
func NormalizeSensitive(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

// FilterSensitive 敏感词过滤：原文中命中的词替换为等长 * 掩码（英文大小写不敏感）。
// 返回过滤后的文本与是否命中。
func FilterSensitive(text string) (string, bool) {
	if text == "" {
		return text, false
	}
	hit := false
	lower := strings.ToLower(text)
	for _, w := range sensitiveWords {
		wl := strings.ToLower(w)
		if !strings.Contains(lower, wl) {
			continue
		}
		hit = true
		mask := strings.Repeat("*", utf8.RuneCountInString(w))
		var b strings.Builder
		b.Grow(len(text))
		i := 0
		for {
			j := strings.Index(lower[i:], wl)
			if j < 0 {
				b.WriteString(text[i:])
				break
			}
			j += i
			b.WriteString(text[i:j])
			b.WriteString(mask)
			i = j + len(wl)
		}
		text = b.String()
		lower = strings.ToLower(text)
	}
	return text, hit
}
