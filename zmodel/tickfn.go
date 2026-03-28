package zmodel

import (
	"context"
	"time"
)

// TickFnItem is a periodic callback item scheduled by Actor.Update.
// TickFnItem 定时回调项：按间隔在 Actor 的 Update 中被调度执行。
type TickFnItem struct {
	Do       func(ctx context.Context, nowTs int64)
	Interval time.Duration
	LastTime int64
	Name     string
}

// NewTickFnItem creates a periodic callback item.
// NewTickFnItem 创建定时回调项。
func NewTickFnItem(name string, interval time.Duration, f func(ctx context.Context, nowTs int64)) *TickFnItem {
	return &TickFnItem{
		Do:       f,
		Interval: interval,
		Name:     name,
	}
}
