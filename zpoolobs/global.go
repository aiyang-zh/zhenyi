package zpoolobs

import (
	"sync/atomic"

	"github.com/aiyang-zh/zhenyi-base/ziface"
	"github.com/aiyang-zh/zhenyi-base/zpool"
)

// poolRelay is pinned to pools and forwards events to GetObserver() at runtime.
// poolRelay 固定在 Pool 上，运行时转发到 GetObserver()，便于 Enable 晚于池创建仍能接上观测。
type poolRelay struct{}

var globalPoolRelay poolRelay

// poolRelaySingleton is fixed dependency for zpool.WithObserver to avoid nil-observer snapshots.
// poolRelaySingleton 作为 zpool.WithObserver 的固定依赖，避免池创建时快照 nil observer。
var poolRelaySingleton ziface.IPoolObserver = &globalPoolRelay

var _ ziface.IPoolObserver = &globalPoolRelay

func (*poolRelay) OnPoolCreate(name string) {
	if o := GetObserver(); o != nil {
		o.OnPoolCreate(name)
	}
}
func (*poolRelay) OnNew(name string) {
	if o := GetObserver(); o != nil {
		o.OnNew(name)
	}
}
func (*poolRelay) OnGet(name string) {
	if o := GetObserver(); o != nil {
		o.OnGet(name)
	}
}
func (*poolRelay) OnPut(name string) {
	if o := GetObserver(); o != nil {
		o.OnPut(name)
	}
}
func (*poolRelay) OnPutNil(name string) {
	if o := GetObserver(); o != nil {
		o.OnPutNil(name)
	}
}

var (
	globalObserver atomic.Pointer[observerHolder]
)

type observerHolder struct {
	obs ziface.IPoolObserver
}

// GetObserver returns the current global pool observer.
// GetObserver 返回当前全局对象池观测器；未设置时返回 nil。
func GetObserver() ziface.IPoolObserver {
	h := globalObserver.Load()
	if h == nil {
		return nil
	}
	return h.obs
}

// SetObserver sets global pool observer; nil disables observation.
// SetObserver 设置全局对象池 observer；传 nil 表示关闭观测。
func SetObserver(obs ziface.IPoolObserver) {
	globalObserver.Store(&observerHolder{obs: obs})
}

// Resolve returns obs when non-nil; otherwise falls back to GetObserver().
// Resolve 若 obs 非 nil 则返回 obs，否则返回 GetObserver()。
// It resolves the final observer when caller passes an optional observer.
// 用于调用方传入可选 observer 时解析最终使用的观测器。
func Resolve(obs ziface.IPoolObserver) ziface.IPoolObserver {
	if obs != nil {
		return obs
	}
	return GetObserver()
}

// NewObservedPool creates a named pool wired to global observer via poolRelay.
// NewObservedPool 创建带名称且接入全局 observer 的对象池（经 poolRelay 转发，支持 SetObserver 晚于池创建）。
// It is the unified zhenyi-layer pool creation entry; prefer PoolName* constants for name.
// 用于 zhenyi 层统一池创建入口，name 建议使用 PoolName* 常量。
func NewObservedPool[T any](name string, f func() T) *zpool.Pool[T] {
	return zpool.NewPoolWithOptions(f, zpool.WithName(name), zpool.WithObserver(poolRelaySingleton))
}
