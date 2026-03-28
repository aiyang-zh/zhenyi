package zmsg

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"

	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi-base/zpool"
	"github.com/aiyang-zh/zhenyi/zmetrics"
	"github.com/aiyang-zh/zhenyi/zpoolobs"
	"go.uber.org/zap"
)

var (
	messagePoolOnce sync.Once
	messagePool     *zpool.Pool[*Message]
)

func getMessagePool() *zpool.Pool[*Message] {
	messagePoolOnce.Do(func() {
		messagePool = zpoolobs.NewObservedPool(zpoolobs.PoolNameZMsgMessage, func() *Message {
			return &Message{
				Data: make([]byte, 0, 256),
			}
		})
	})
	return messagePool
}

// GetMessage obtains a message from pool with initial refcount = 1.
// GetMessage 从池中获取消息（初始引用计数 = 1）。
func GetMessage() *Message {
	msg := getMessagePool().Get()
	msg.PoolReset()
	atomic.StoreInt32(&msg.RefCount, 1)

	if DEBUG_LIFECYCLE {
		trackMessage(msg)
	}
	return msg
}

// Retain increases reference count and returns the same message for chaining.
// Retain 引用计数 +1，并返回自身以便链式调用。
func (m *Message) Retain() *Message {
	if m == nil {
		return nil
	}
	newRef := atomic.AddInt32(&m.RefCount, 1)
	if DEBUG_LIFECYCLE {
		if info, ok := liveMessages.Load(m); ok {
			log.Printf("MSG#%d Retain (refCount: %d -> %d)",
				info.(*AllocInfo).ID, newRef-1, newRef)
		}

		if newRef <= 1 {
			panic("Retain called on released message")
		}
	}
	return m
}

// Release decreases refcount and returns message to pool when it reaches zero.
// Release 减少引用计数，降为 0 时自动回收到池。
func (m *Message) Release() {
	if m == nil {
		return
	}

	newRef := atomic.AddInt32(&m.RefCount, -1)
	if DEBUG_LIFECYCLE {
		if info, ok := liveMessages.Load(m); ok {
			log.Printf("MSG#%d Release (refCount: %d -> %d)",
				info.(*AllocInfo).ID, newRef+1, newRef)
		}
	}
	if newRef == 0 {
		// Fast path: single-reference release, checked first for branch prediction.
		// 快速路径：单次引用释放，优先检查以利于分支预测。
		if DEBUG_LIFECYCLE {
			untrackMessage(m)
		}
		atomic.StoreInt32(&m.RefCount, 0)
		if cap(m.Data) > 4096 {
			m.Data = nil
		}
		getMessagePool().Put(m)
		return
	}
	if newRef < 0 {
		zmetrics.MsgPoolDoubleRelease.Add(1)
		if DEBUG_LIFECYCLE {
			panic(fmt.Sprintf("Double release detected! refCount=%d msgId=%d", newRef, m.MsgId))
		}
		zlog.Error("Double release detected", zap.Int32("refCount", newRef), zap.Int32("msgId", m.MsgId))
		atomic.StoreInt32(&m.RefCount, 0)
	}
}

// MustRelease safely releases message (useful with defer).
// MustRelease 安全释放（用于 defer）。
func (m *Message) MustRelease() {
	if m != nil {
		m.Release()
	}
}

// LoadRefCount returns current reference count (0 for nil).
// LoadRefCount 返回当前引用计数（nil 时返回 0）。
func (m *Message) LoadRefCount() int32 {
	if m == nil {
		return 0
	}
	return atomic.LoadInt32(&m.RefCount)
}
