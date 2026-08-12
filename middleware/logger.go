package middleware

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

// ==================== 异步请求日志 ====================
// 背景：每个请求同步写日志（控制台 + 文件双输出）会阻塞请求处理，高并发下成为吞吐瓶颈。
// 方案：请求侧只把日志条目丢进内存队列（select-default 满则丢弃，绝不阻塞请求），
//       后台 worker 协程负责真正落盘，实现"日志异步化、请求零 IO 等待"。
// 说明：日志为 best-effort 语义——队列满丢弃并计数，进程异常退出最多丢 chan 容量内的日志。

var (
	logQueue      = make(chan []any, 1024) // 请求日志队列（容量 1024，远超常规单机并发）
	logWG         sync.WaitGroup           // worker 生命周期（Flush 时等待排空）
	shutdownOnce  sync.Once                // 保证队列只关闭一次
	shutdownLogs  atomic.Bool              // 关闭后不再入队
	droppedLogs   atomic.Int64             // 队列满被丢弃的日志条数（诊断用）
	submittedLogs atomic.Int64             // 成功入队条数
)

// startAsyncLogWorker 启动日志落盘 worker：消费队列，真正写入 slog（MultiWriter：控制台 + 文件）
func startAsyncLogWorker() {
	logWG.Add(1)
	go func() {
		defer logWG.Done()
		for rec := range logQueue {
			slog.Info("request", rec...)
		}
	}()
}

// submitLog 入队一条请求日志。队列满时直接丢弃并计数（best-effort），调用方（请求路径）永不阻塞。
func submitLog(rec []any) {
	if shutdownLogs.Load() {
		return
	}
	select {
	case logQueue <- rec:
		submittedLogs.Add(1)
	default:
		droppedLogs.Add(1)
	}
}

// FlushAsyncLogs 排空日志队列并停止 worker（进程退出/测试结束前调用）。
// 只能调用一次：调用后队列关闭，submitLog 不再入队。
func FlushAsyncLogs() {
	shutdownOnce.Do(func() {
		shutdownLogs.Store(true)
		close(logQueue)
		logWG.Wait()
	})
}

// InitLogger 初始化全局结构化日志（slog）：控制台 + 文件双输出，并启动异步落盘 worker
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

	startAsyncLogWorker()
	return nil
}

// RequestLogger 请求日志中间件：异步记录方法/路径/状态码/耗时/IP，请求路径零 IO 等待。
// 设置环境变量 REQUEST_LOG=off 可完全跳过（压测/日志风暴时连入队都省掉）。
func RequestLogger() gin.HandlerFunc {
	if os.Getenv("REQUEST_LOG") == "off" {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		start := time.Now()

		// 洋葱中间件：Next 放行，执行后面的中间件和 handler
		c.Next()

		// 组装日志字段后入队：写盘由后台 worker 完成，这里不阻塞请求
		rec := []any{
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			"ip", c.ClientIP(),
			"size", c.Writer.Size(),
		}
		submitLog(rec)
	}
}
