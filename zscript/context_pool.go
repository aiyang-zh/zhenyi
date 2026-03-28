package zscript

import (
	"sync"

	"github.com/aiyang-zh/zhenyi-base/zpool"
	"github.com/aiyang-zh/zhenyi/zpoolobs"
)

var (
	contextPoolOnce sync.Once
	ContextPool     *zpool.Pool[*ScriptContext]
)

func initContextPool() {
	contextPoolOnce.Do(func() {
		ContextPool = zpoolobs.NewObservedPool(zpoolobs.PoolNameZScriptContext, func() *ScriptContext {
			return &ScriptContext{}
		})
	})
}

// GetContext gets a ScriptContext from pool and initializes core fields.
// GetContext 从池中获取 ScriptContext 并初始化核心字段。
// The returned context already contains ActorID, ActorType and NowMillis.
// 返回的上下文已包含 ActorID、ActorType 与 NowMillis。
// TraceID defaults to 0 and should be propagated via WithTraceID.
// TraceID 默认值为 0，应通过 WithTraceID 从消息中传递。
func GetContext(actorID uint64, actorType uint32) *ScriptContext {
	initContextPool()
	ctx := ContextPool.Get()
	ctx.init(actorID, actorType) // ✅ 包内调用
	return ctx
}

// PutContext resets and returns ScriptContext back to pool.
// PutContext 重置并归还 ScriptContext 到池中。
// It clears all fields, including Metadata (set to nil).
// 该方法会清理全部字段，包括 Metadata（置为 nil）。
func PutContext(ctx *ScriptContext) {
	ctx.reset() // ✅ 包内调用
	initContextPool()
	ContextPool.Put(ctx)
}
