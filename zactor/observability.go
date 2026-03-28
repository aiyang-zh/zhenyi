package zactor

import (
	"fmt"

	"github.com/aiyang-zh/zhenyi/zmetrics"
)

// handlerObserver holds per-handler observability context.
// handlerObserver 单个 Handler 的可观测性上下文（Init 阶段预计算，Run 阶段只读）。
//
// Design principles:
// 设计原则：
//   - 所有字符串在 Init 阶段预构建，Run 阶段零字符串分配
//   - metric 指针直接引用 zmetrics.HandlerMetric（同类型 Actor 实例共享）
//   - spanName 预拼接，避免运行时 fmt.Sprintf
type handlerObserver struct {
	metric   *zmetrics.HandlerMetric // pre-resolved, shared across same actor type
	spanName string                  // pre-computed: "handler.<msgId>"
}

// newHandlerObserver 创建 handler 可观测性上下文（仅 Init 阶段调用）
func newHandlerObserver(actorId uint64, actorType uint32, msgId int32) *handlerObserver {
	return &handlerObserver{
		metric:   zmetrics.GetHandlerMetric(actorId, actorType, msgId),
		spanName: fmt.Sprintf("handler.%d", msgId),
	}
}
