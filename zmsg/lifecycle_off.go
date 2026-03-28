//go:build !debug_lifecycle

package zmsg

import (
	"context"
	"sync"
	"time"
)

// DEBUG_LIFECYCLE is disabled by default (production mode).
// DEBUG_LIFECYCLE 默认关闭（生产环境）。
const DEBUG_LIFECYCLE = false

// These symbols are referenced by msgpool.go and must compile even when DEBUG_LIFECYCLE is false.
// 这些符号在 msgpool.go 中被引用（即使在 if DEBUG_LIFECYCLE 分支内也需要通过编译）。
var (
	liveMessages sync.Map // *Message -> *AllocInfo
)

type AllocInfo struct {
	ID         uint64
	Stack      string
	CreateTime time.Time
}

func trackMessage(_ *Message)   {}
func untrackMessage(_ *Message) {}

func StartLeakDetector(_ context.Context) {}
