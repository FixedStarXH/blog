package middleware

import "testing"

// TestAsyncLogQueue 验证异步日志队列的核心语义：
// 1) 队列满时 submitLog 立即丢弃（best-effort），绝不阻塞请求路径；
// 2) 启动 worker 后 FlushAsyncLogs 能排空队列并正常退出。
// 两个场景放同一函数保证执行顺序（Flush 会关闭全局队列，不能与其他用例混跑）。
func TestAsyncLogQueue(t *testing.T) {
	// ---- 阶段 1：队列满 -> 丢弃，不阻塞 ----
	// 此刻 worker 未启动（无人消费），把缓冲队列灌满
	for i := 0; i < cap(logQueue); i++ {
		submitLog([]any{"filler", i})
	}
	before := droppedLogs.Load()
	submitLog([]any{"overflow", "should-drop"}) // 第 cap+1 条：走 default 分支丢弃并计数
	if after := droppedLogs.Load(); after-before != 1 {
		t.Fatalf("队列满时应丢弃 1 条并计数，实际丢弃 %d 条", after-before)
	}
	drainQueue() // 清空现场，不影响阶段 2

	// ---- 阶段 2：worker 启动 -> 入队 -> Flush 排空 ----
	startAsyncLogWorker()
	submitLog([]any{"k", "v1"})
	submitLog([]any{"k", "v2"})
	FlushAsyncLogs() // 关闭队列并等待 worker 消费完
	if got := submittedLogs.Load(); got < 2 {
		t.Fatalf("期望至少 2 条日志成功入队并消费，实际 %d", got)
	}
}

// drainQueue 手动排空队列（供队列满测试后清理现场）
func drainQueue() {
	for {
		select {
		case <-logQueue:
		default:
			return
		}
	}
}
