package middleware

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
)

// InitLogger 初始化全局结构化日志（slog）：控制台 + 文件双输出
func InitLogger() error {
	// 确保日志目录存在（文件夹不存在 OpenFile 会报错）
	if err := os.MkdirAll("logs", 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join("logs", "app.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	// 双输出：终端 + 文件，一份日志两个地方写
	multi := io.MultiWriter(os.Stdout, f)
	logger := slog.New(slog.NewJSONHandler(multi, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger) // 之后所有 slog.Info 都走这个 logger
	return nil
}

// RequestLogger 请求日志中间件：记录方法/路径/状态码/耗时/IP
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// 洋葱中间件：Next 放行，执行后面的中间件和 handler
		c.Next()

		// Next 返回后请求已处理完，此时写日志（状态码、耗时都知道了）
		slog.Info("request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			"ip", c.ClientIP(),
			"size", c.Writer.Size(),
		)
	}
}
