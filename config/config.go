package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

var AppConfig *Config

type Config struct {
	Server ServerConfig `mapstructure:"server"`
	MySQL  MySQLConfig  `mapstructure:"mysql"`
	Redis  RedisConfig  `mapstructure:"redis"`
	JWT    JWTConfig    `mapstructure:"jwt"`
	AI     AIConfig     `mapstructure:"ai"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// Addr 返回 redis 连接地址 host:port
func (r *RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

type ServerConfig struct {
	Port int `mapstructure:"port"`
}

type MySQLConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
}

type JWTConfig struct {
	Secret string `mapstructure:"secret"`
}

// AIConfig AI 能力配置（摘要/润色/RAG 问答）
// 兼容 OpenAI 接口格式（/v1/chat/completions、/v1/embeddings）：
//   - 推荐 SiliconFlow(https://api.siliconflow.cn/v1)：一份 key 同时提供 chat 与 embedding，注册送免费额度
//   - 也可换 DeepSeek(https://api.deepseek.com/v1) 等，只需改 base_url + 模型名
//
// 生产 key 用环境变量注入：BLOG_AI_API_KEY
type AIConfig struct {
	Enabled      bool   `mapstructure:"enabled"`
	BaseURL      string `mapstructure:"base_url"`
	APIKey       string `mapstructure:"api_key"`
	ChatModel    string `mapstructure:"chat_model"`
	EmbedModel   string `mapstructure:"embed_model"`
	TimeoutSecs  int    `mapstructure:"timeout_secs"`
	MaxDailyAsks int    `mapstructure:"max_daily_asks"` // 每日问答次数上限；<=0 表示不限制（防刷接口烧 token 额度）
}

func (m *MySQLConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local", m.User, m.Password, m.Host, m.Port, m.DBName)
}

func Init() error {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	// 环境变量覆盖配置（Docker Compose 用）：
	//   BLOG_MYSQL_HOST → mysql.host、BLOG_REDIS_HOST → redis.host、BLOG_JWT_SECRET → jwt.secret …
	// 本地开发不设环境变量时，仍然读 config.yaml 的值，行为不变
	viper.SetEnvPrefix("BLOG")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// 默认值：容器里没有 config.yaml（被 .gitignore 排除），全靠默认值 + 环境变量兜底
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("mysql.host", "127.0.0.1")
	viper.SetDefault("mysql.port", 3306)
	viper.SetDefault("mysql.user", "root")
	viper.SetDefault("mysql.password", "123456")
	viper.SetDefault("mysql.dbname", "blog_system")
	viper.SetDefault("redis.host", "127.0.0.1")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.db", 0)
	viper.SetDefault("jwt.secret", "my-secret-key-2026")
	// AI 默认关闭：没配 key 时 AI 功能自动降级（摘要回退截取首段、问答返回提示）
	viper.SetDefault("ai.enabled", false)
	viper.SetDefault("ai.base_url", "https://api.siliconflow.cn/v1")
	viper.SetDefault("ai.api_key", "")
	viper.SetDefault("ai.chat_model", "deepseek-ai/DeepSeek-V3")
	viper.SetDefault("ai.embed_model", "BAAI/bge-m3")
	viper.SetDefault("ai.timeout_secs", 30)
	viper.SetDefault("ai.max_daily_asks", 200)

	// 本地开发：config.yaml 存在才读取（缺文件时不报错，Docker 场景只靠 env+默认值）
	if _, err := os.Stat("config.yaml"); err == nil {
		if err := viper.ReadInConfig(); err != nil {
			return fmt.Errorf("读取配置文件失败:%w", err)
		}
	}

	AppConfig = &Config{}
	if err := viper.Unmarshal(AppConfig); err != nil {
		return fmt.Errorf("解析配置失败:%w", err)
	}
	return nil
}
