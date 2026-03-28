package zactor

import (
	"context"
	"sync"

	"github.com/aiyang-zh/zhenyi/zmsg"
)

// Trace hook function variables.
// When trace is not initialized, these remain nil and tracing is skipped.
// Call SetTraceHooks once at startup (after trace.Init) to enable.

var (
	traceEnabled        func() bool
	traceStartSpan      func(ctx context.Context, name string) (context.Context, func())
	traceContextFromMsg func(ctx context.Context, msg *zmsg.Message) context.Context
	traceGenerateIDs    func(msg *zmsg.Message)
	traceHooksOnce      sync.Once
)

// SetTraceHooks sets trace hook functions for the actor package.
// SetTraceHooks 为 zactor 包设置 tracing hook 函数。
// Must be called once at startup before any Actor starts running.
// 必须在启动阶段且任意 Actor 运行前调用一次。
// Subsequent calls are silently ignored (sync.Once).
// 后续重复调用会被静默忽略（sync.Once）。
func SetTraceHooks(
	enabled func() bool,
	startSpan func(ctx context.Context, name string) (context.Context, func()),
	ctxFromMsg func(ctx context.Context, msg *zmsg.Message) context.Context,
	generateIDs func(msg *zmsg.Message),
) {
	traceHooksOnce.Do(func() {
		traceEnabled = enabled
		traceStartSpan = startSpan
		traceContextFromMsg = ctxFromMsg
		traceGenerateIDs = generateIDs
	})
}

func isTraceEnabled() bool {
	return traceEnabled != nil && traceEnabled()
}
