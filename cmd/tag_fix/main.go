// 命令：go run ./cmd/tag_fix
//
// 功能：一次性数据治理脚本——
//  1. 丰富分类库（Go / Java / 数据库 / 前端 / AI / 并发 / 网络 / 工程 / 算法 / 随笔）
//  2. 丰富标签库（按技术点细分，40+ 个）
//  3. 去重（CSDN 爬虫因标题全半角差异曾重复导入，同标题保留最早一篇）
//  4. 按标题关键词智能分配分类 + 多标签
//
// 幂等：可安全重复运行（分类/标签 FirstOrCreate，文章每次重新匹配覆盖）
package main

import (
	"fmt"
	"os"
	"strings"

	"blog-system/config"
	"blog-system/model"

	"gorm.io/gorm"
)

// ==================== 分类库 ====================

type catDef struct {
	Name        string
	Description string
	Sort        int
}

var catDefs = []catDef{
	{"Go语言", "Go 语言语法、并发、标准库与生态（Gin/GORM/Viper）", 1},
	{"Java", "Java 核心、Spring、JVM 与生态", 2},
	{"数据库", "MySQL、Redis、索引、事务与缓存", 3},
	{"前端", "HTML、CSS、JavaScript 与前端工程化", 4},
	{"AI与大模型", "LLM、Agent、Prompt 与大模型应用", 5},
	{"并发编程", "goroutine、channel、锁与并发模式", 6},
	{"网络编程", "TCP、HTTP、Socket 与网络协议", 7},
	{"工程实践", "架构、规范、工具链与工程化", 8},
	{"算法", "数据结构与算法", 9},
	{"随笔", "生活思考与技术杂谈", 10},
}

// ==================== 标签库 ====================

var tagDefs = []string{
	// Go 生态
	"gin", "gorm", "viper", "context", "反射", "泛型", "interface", "错误处理",
	// 并发
	"goroutine", "channel", "GMP调度", "WaitGroup", "mutex", "多线程",
	// Java
	"Spring Boot", "JVM", "注解", "集合", "IO", "异常",
	// 数据库
	"MySQL", "Redis", "索引", "事务", "SQL优化", "缓存",
	// 前端
	"JavaScript", "HTML", "CSS", "面向对象", "面向过程",
	// AI
	"LLM", "Agent", "Prompt", "ReAct", "大模型训练", "Skill",
	// 网络
	"TCP", "HTTP", "Socket",
	// 通用
	"入门", "实战", "源码解析", "面试", "性能优化", "爬虫", "工具", "规范", "算法",
}

// name → ID 映射（启动时填充）
var catMap = map[string]uint{}
var tagMap = map[string]uint{}

// ==================== 主流程 ====================

func main() {
	if err := config.Init(); err != nil {
		fmt.Println("配置加载失败:", err)
		os.Exit(1)
	}
	if err := model.InitDB(); err != nil {
		fmt.Println("数据库初始化失败:", err)
		os.Exit(1)
	}
	db := model.DB

	ensureCategories(db)
	ensureTags(db)
	dedupArticles(db)
	classifyAll(db)
	cleanup(db)

	fmt.Println("\n==> 数据治理完成")
}

// cleanup 清理冗余数据：删除不在标准库中且 0 篇引用的旧分类/标签
// （历史遗留的"学习笔记"分类、"Go并发"等重复标签）
func cleanup(db *gorm.DB) {
	fmt.Println("== 5. 清理冗余分类/标签 ==")

	// 标准分类名集合
	stdCats := map[string]bool{}
	for _, c := range catDefs {
		stdCats[c.Name] = true
	}
	// 删除非标准且 0 篇的分类
	var cats []model.Category
	db.Find(&cats)
	for _, c := range cats {
		if stdCats[c.Name] {
			continue
		}
		var cnt int64
		db.Model(&model.Article{}).Where("category_id = ?", c.ID).Count(&cnt)
		if cnt == 0 {
			db.Delete(&c)
			fmt.Printf("   删除冗余分类 #%d %s\n", c.ID, c.Name)
		}
	}

	// 标准标签名集合
	stdTags := map[string]bool{}
	for _, t := range tagDefs {
		stdTags[t] = true
	}
	// 删除非标准且 0 篇的标签
	var tags []model.Tag
	db.Find(&tags)
	for _, t := range tags {
		if stdTags[t.Name] {
			continue
		}
		var cnt int64
		db.Table("article_tags").Where("tag_id = ?", t.ID).Count(&cnt)
		if cnt == 0 {
			db.Delete(&t)
			fmt.Printf("   删除冗余标签 #%d %s\n", t.ID, t.Name)
		}
	}
}

// ensureCategories 创建/补全分类（幂等：FirstOrCreate + 更新描述与排序）
func ensureCategories(db *gorm.DB) {
	fmt.Println("== 1. 丰富分类库 ==")
	for _, c := range catDefs {
		var cat model.Category
		db.FirstOrCreate(&cat, model.Category{Name: c.Name})
		// 补全描述和排序（已存在的也更新，保持定义同步）
		db.Model(&cat).Updates(map[string]interface{}{
			"description": c.Description,
			"sort":        c.Sort,
		})
		catMap[c.Name] = cat.ID
		fmt.Printf("   分类 #%d %s — %s\n", cat.ID, c.Name, c.Description)
	}
}

// ensureTags 创建/补全标签（幂等：FirstOrCreate）
func ensureTags(db *gorm.DB) {
	fmt.Printf("== 2. 丰富标签库（%d 个）==\n", len(tagDefs))
	for _, name := range tagDefs {
		var t model.Tag
		db.FirstOrCreate(&t, model.Tag{Name: name})
		tagMap[name] = t.ID
	}
	fmt.Println("   标签已就绪")
}

// dedupArticles 去重：同标题保留 id 最小的一篇，其余软删除
// （CSDN 爬虫曾因标题全/半角冒号、空格差异导致重复导入）
func dedupArticles(db *gorm.DB) {
	fmt.Println("== 3. 去重（同标题保留最早一篇）==")
	var articles []model.Article
	db.Find(&articles) // 软删除的不会被 Find 查出

	seen := map[string]uint{} // title → 保留的 id
	removed := 0
	for _, a := range articles {
		key := normalize(a.Title)
		if keepID, ok := seen[key]; ok {
			// 已有保留篇，删除当前这篇（保留 id 更小的）
			if a.ID < keepID {
				// 当前篇更早：删掉之前保留的，改保留当前
				db.Delete(&model.Article{}, keepID)
				seen[key] = a.ID
			} else {
				db.Delete(&model.Article{}, a.ID)
				removed++
			}
		} else {
			seen[key] = a.ID
		}
	}
	fmt.Printf("   去重完成：删除重复文章 %d 篇，剩余 %d 篇\n", removed, len(articles)-removed)
}

// classifyAll 给所有文章按标题分配分类和多标签
func classifyAll(db *gorm.DB) {
	fmt.Println("== 4. 智能分配分类与标签 ==")
	var articles []model.Article
	db.Find(&articles)

	for _, a := range articles {
		catName, tagNames := classify(a.Title)
		catID := catMap[catName]
		if catID == 0 {
			catID = catMap["随笔"] // 兜底
		}

		// 更新分类
		db.Model(&model.Article{}).Where("id = ?", a.ID).Update("category_id", catID)

		// 替换标签（多对多：Association.Replace 清旧建新）
		tags := make([]model.Tag, 0, len(tagNames))
		for _, tn := range tagNames {
			if id, ok := tagMap[tn]; ok {
				tags = append(tags, model.Tag{BaseModel: model.BaseModel{ID: id}})
			}
		}
		if err := db.Model(&model.Article{BaseModel: model.BaseModel{ID: a.ID}}).
			Association("Tags").Replace(tags); err != nil {
			fmt.Printf("   [!] #%d 标签关联失败: %v\n", a.ID, err)
		}

		fmt.Printf("   #%d [%s] %s → 标签: %v\n", a.ID, catName, a.Title, tagNames)
	}
}

// classify 根据标题返回（分类名, 标签名列表）
// 顺序敏感：先判定大类，再叠加细标签
func classify(title string) (string, []string) {
	// 统一小写 + 去多余空格，便于关键词匹配
	t := strings.ToLower(normalize(title))
	tags := []string{}
	cat := ""

	// ---- 1. 大类判定（顺序即优先级）----
	// 英文关键词用 hasWord（词边界），避免 "engineering" 误含 "gin"
	switch {
	case hasWord(t, "java", "spring", "jvm", "javaweb") || containsAny(t, "注解", "异常"):
		cat = "Java"

	case hasWord(t, "mysql", "redis", "sql") || containsAny(t, "数据库", "索引", "事务", "缓存"):
		cat = "数据库"

	case hasWord(t, "javascript", "html", "css") || containsAny(t, "前端", "面向过程"):
		cat = "前端"

	case hasWord(t, "agent", "llm", "prompt", "skill", "react") || containsAny(t, "大模型"):
		cat = "AI与大模型"

	case hasWord(t, "goroutine", "channel", "waitgroup", "mutex", "gmp") ||
		containsAny(t, "并发", "多线程", "锁"):
		cat = "并发编程"

	case hasWord(t, "tcp", "http", "socket") || containsAny(t, "网络", "协议"):
		cat = "网络编程"

	case containsAny(t, "算法", "数据结构"):
		cat = "算法"

	case hasWord(t, "sdd") || containsAny(t, "工程", "规范", "架构", "工具", "接口") ||
		containsAny(t, "claude code", "harness"):
		cat = "工程实践"

	case hasWord(t, "go", "golang", "gin", "gorm", "viper"):
		cat = "Go语言"

	default:
		cat = "随笔"
	}

	// ---- 2. 细标签叠加（一篇可多标签）----
	// 顺序无优先级，命中即加；用 set 去重
	set := map[string]bool{}
	add := func(names ...string) {
		for _, n := range names {
			set[n] = true
		}
	}

	// Go 生态（英文用 hasWord 防子串误判）
	if hasWord(t, "gin") {
		add("gin")
	}
	if hasWord(t, "gorm") {
		add("gorm")
	}
	if hasWord(t, "viper") {
		add("viper")
	}
	if hasWord(t, "context") {
		add("context")
	}
	if containsAny(t, "反射") {
		add("反射")
	}
	if containsAny(t, "泛型") {
		add("泛型")
	}
	if hasWord(t, "interface") {
		add("interface")
	}
	if containsAny(t, "错误") {
		add("错误处理")
	}

	// 并发
	if hasWord(t, "goroutine") || containsAny(t, "并发") {
		add("goroutine", "channel")
	}
	if hasWord(t, "channel") {
		add("channel")
	}
	if hasWord(t, "gmp") {
		add("GMP调度", "源码解析")
	}
	if hasWord(t, "waitgroup") {
		add("WaitGroup")
	}
	if containsAny(t, "锁") || hasWord(t, "mutex") {
		add("mutex")
	}
	if containsAny(t, "多线程") {
		add("多线程")
	}

	// Java
	if hasWord(t, "spring") {
		add("Spring Boot")
	}
	if hasWord(t, "jvm") {
		add("JVM")
	}
	if containsAny(t, "注解") {
		add("注解")
	}
	if containsAny(t, "集合") {
		add("集合")
	}
	if containsAny(t, "异常") {
		add("异常")
	}

	// 数据库
	if hasWord(t, "mysql") {
		add("MySQL")
	}
	if hasWord(t, "redis") {
		add("Redis")
	}
	if containsAny(t, "索引") {
		add("索引", "性能优化")
	}
	if containsAny(t, "事务") {
		add("事务")
	}
	if containsAny(t, "缓存") {
		add("缓存")
	}

	// 前端
	if hasWord(t, "javascript") {
		add("JavaScript")
	}
	if hasWord(t, "html") {
		add("HTML")
	}
	if hasWord(t, "css") {
		add("CSS")
	}
	if containsAny(t, "面向对象") {
		add("面向对象")
	}
	if containsAny(t, "面向过程") {
		add("面向过程")
	}

	// AI
	if hasWord(t, "llm") || containsAny(t, "大模型") {
		add("LLM")
	}
	if hasWord(t, "agent") {
		add("Agent")
	}
	if hasWord(t, "prompt") {
		add("Prompt")
	}
	if hasWord(t, "react") && hasWord(t, "agent") {
		add("ReAct")
	}
	if containsAny(t, "大模型") && (containsAny(t, "训练") || containsAny(t, "应用")) {
		add("大模型训练")
	}
	if hasWord(t, "skill") {
		add("Skill")
	}

	// 网络
	if hasWord(t, "tcp") || containsAny(t, "网络") {
		add("TCP")
	}
	if hasWord(t, "http") {
		add("HTTP")
	}
	if hasWord(t, "socket") {
		add("Socket")
	}

	// 通用属性标签
	if containsAny(t, "入门", "从零", "初学", "总结") || strings.Contains(t, "从 0") {
		add("入门")
	}
	if containsAny(t, "实战", "搭建", "玩转") {
		add("实战")
	}
	if containsAny(t, "源码") {
		add("源码解析")
	}
	if containsAny(t, "面试") {
		add("面试")
	}
	if containsAny(t, "优化", "性能") {
		add("性能优化")
	}
	if containsAny(t, "爬虫") {
		add("爬虫")
	}
	// 工具标签：仅明确工具名才加（"指南"不等同工具，已移除）
	if containsAny(t, "claude code", "harness", "工具") {
		add("工具")
	}
	if containsAny(t, "规范", "sdd") {
		add("规范")
	}
	if containsAny(t, "算法") {
		add("算法")
	}

	// 至少一个标签：兜底入门
	if len(set) == 0 {
		add("入门")
	}

	for n := range set {
		tags = append(tags, n)
	}
	return cat, tags
}

// hasWord 判断 s 是否包含任意英文关键词作为「独立词」出现
// 避免子串误判：如 "engineering" 不应匹配 "gin"，"google" 不应匹配 "go"
// 独立词定义：关键词前后字符为非英文字母（含字符串边界、空格、中文、标点）
func hasWord(s string, words ...string) bool {
	for _, w := range words {
		if wordBoundary(s, w) {
			return true
		}
	}
	return false
}

// wordBoundary 检查 word 是否作为独立词出现在 s 中
func wordBoundary(s, word string) bool {
	idx := 0
	for {
		pos := strings.Index(s[idx:], word)
		if pos < 0 {
			return false
		}
		pos += idx // 转为绝对位置
		start := pos
		end := pos + len(word)
		// 前边界：start==0 或前一字符非字母
		beforeOK := start == 0 || !isASCIILetter(s[start-1])
		// 后边界：end==len 或后一字符非字母
		afterOK := end >= len(s) || !isASCIILetter(s[end])
		if beforeOK && afterOK {
			return true
		}
		idx = pos + 1
		if idx >= len(s) {
			return false
		}
	}
}

// isASCIILetter 判断字节是否为 ASCII 字母（a-z, A-Z）
// 注意：中文是 UTF-8 多字节，首字节不会落在 ASCII 字母范围，天然视为"非字母"边界
func isASCIILetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// containsAny 判断 s 是否包含 subs 中任一子串
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// normalize 标题归一化：去首尾空白 + 全角转半角冒号/空格，用于去重比较
// （CSDN 两次抓取可能因编码差异产生全半角不同，但内容相同）
func normalize(s string) string {
	s = strings.TrimSpace(s)
	// 全角空格 → 半角
	s = strings.ReplaceAll(s, "\u3000", " ")
	// 全角冒号 → 半角
	s = strings.ReplaceAll(s, "：", ":")
	// 合并连续空格
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}
