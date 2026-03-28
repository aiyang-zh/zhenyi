package zmetrics

import (
	"sync/atomic"

	"github.com/aiyang-zh/zhenyi-base/zlog"
)

var actorPanicHookAdded atomic.Bool

// EnsureActorPanicHook 将 zhenyi_actor_panic_total 挂到 zlog 的 panic 回调链（AppendPanicHook）；幂等。
// zmetrics.Enable / EnableWithOptions 会调用；未启用指标服务时，首个 zactor.NewActor 也会触发，避免漏计 Recover 路径。
func EnsureActorPanicHook() {
	if !actorPanicHookAdded.CompareAndSwap(false, true) {
		return
	}
	zlog.AppendPanicHook(func() {
		ActorPanicCount.Inc()
	})
}
