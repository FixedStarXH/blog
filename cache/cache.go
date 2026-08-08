// Package cache 封装 Redis 客户端与通用缓存操作
//
// 设计要点：
//  1. 所有缓存失败都"静默降级"——Redis 挂了不影响主流程，只是缓存穿透回数据库
//  2. 统一用 JSON 序列化（结构体直接存），key 前缀隔离业务域
//  3. 提供 Get/Set/Del/DelPrefix 四个基础操作 + 视图计数专用方法
package cache

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	Client *redis.Client
	Ctx    = context.Background()
)

// Redis 是否可用（连接失败时为 false，所有方法直接跳过）
var Enabled = false

// Init 初始化 Redis 连接（失败只告警不 panic：缓存是锦上添花，不能因为 Redis 挂了就启动失败）
func Init(addr, password string, db int) {
	Client = redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  3 * time.Second, // 连接超时：Redis 挂了快速失败，不要拖住请求
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	})

	ctx, cancel := context.WithTimeout(Ctx, 3*time.Second)
	defer cancel()

	if err := Client.Ping(ctx).Err(); err != nil {
		log.Printf("[cache] Redis 连接失败，缓存层已禁用（降级直查数据库）: %v", err)
		Enabled = false
		Client = nil
		return
	}
	Enabled = true
	log.Println("[cache] Redis 缓存层已启用")
}

// ------------------------------------------------------------------
// 通用 JSON 缓存
// ------------------------------------------------------------------

// Get 读取缓存到 target；命中返回 true，未命中/失败返回 false
func Get(key string, target any) bool {
	if !Enabled {
		return false
	}
	data, err := Client.Get(Ctx, key).Bytes()
	if err != nil {
		return false // 未命中或 Redis 故障：都当没缓存
	}
	if err := json.Unmarshal(data, target); err != nil {
		log.Printf("[cache] 反序列化失败 key=%s: %v", key, err)
		return false
	}
	return true
}

// Set 写入缓存（ttl<=0 表示不过期）
func Set(key string, value any, ttl time.Duration) {
	if !Enabled {
		return
	}
	data, err := json.Marshal(value)
	if err != nil {
		return
	}
	if ttl <= 0 {
		Client.Set(Ctx, key, data, 0)
		return
	}
	Client.Set(Ctx, key, data, ttl)
}

// Del 删除一个或多个 key
func Del(keys ...string) {
	if !Enabled || len(keys) == 0 {
		return
	}
	Client.Del(Ctx, keys...)
}

// DelPrefix 按前缀批量删除（用 SCAN 避免 KEYS 阻塞 Redis，生产安全写法）
// 例：DelPrefix("articles:list:") 清掉所有文章列表缓存
func DelPrefix(prefix string) {
	if !Enabled || prefix == "" {
		return
	}
	iter := Client.Scan(Ctx, 0, prefix+"*", 100).Iterator()
	var keys []string
	for iter.Next(Ctx) {
		keys = append(keys, iter.Val())
	}
	if len(keys) == 0 {
		return
	}
	if err := Client.Del(Ctx, keys...).Err(); err != nil {
		log.Printf("[cache] 清理前缀缓存失败 %s*: %v", prefix, err)
	}
}

// ------------------------------------------------------------------
// 缓存 key 统一管理（业务前缀 + 参数拼 key，方便 DelPrefix 批量清理）
// ------------------------------------------------------------------

const (
	KeyArticleList = "articles:list:" // 文章列表：articles:list:{page}:{pageSize}:{...筛选参数}
	KeyArticle     = "article:"       // 文章详情：article:{id}
	KeyHot         = "articles:hot:"  // 热门文章：articles:hot:{limit}
	KeyArchives    = "archives"       // 时间归档（只有一个）
	KeyCategories  = "categories"     // 分类列表（含文章数）
	KeyTags        = "tags"           // 标签列表（含文章数）
)

// 各缓存 TTL（秒）
const (
	TTLList   = 60 * time.Second // 列表：浏览量大，短 TTL 保证新文章能快速出现
	TTLDetail = 5 * time.Minute  // 详情：内容不常变，长一点
	TTLStatic = 5 * time.Minute  // 分类/标签/归档等低频数据
)

// InvalidateArticleRelated 文章相关缓存全失效（新建/编辑/删除/审核后调用）
// 一次清掉：该文章详情 + 所有列表 + 热门 + 归档 + 分类计数 + 标签计数
func InvalidateArticleRelated(articleID uint) {
	Del(KeyArticle + uintToString(articleID))
	DelPrefix(KeyArticleList)
	Del(KeyHot, KeyArchives, KeyCategories, KeyTags)
}

// InvalidateTaxonomy 分类/标签变更后清理（只影响计数类缓存）
func InvalidateTaxonomy() {
	Del(KeyCategories, KeyTags, KeyArchives)
	DelPrefix(KeyArticleList)
}

func uintToString(v uint) string {
	// 简单数字转字符串，避免引入 strconv 使调用处啰嗦
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
