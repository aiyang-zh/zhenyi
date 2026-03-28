package zactor

import (
	"sync/atomic"

	"github.com/aiyang-zh/zhenyi-base/ztime"
)

type cbState int32

const (
	cbClosed   cbState = 0 // Closed state (pass-through) / 正常通行
	cbOpen     cbState = 1 // Open state (fast-fail) / 快速失败
	cbHalfOpen cbState = 2 // Half-open probing state / 试探中

	defaultCBThreshold  = 5         // Open circuit after N continuous failures / 连续失败 N 次后熔断
	defaultCBCooldownMs = 10 * 1000 // Cooldown after opening (10s) / 熔断后 10s 冷却
)

// circuitBreaker is a lightweight breaker for a single targetActorId.
// circuitBreaker 针对单个 targetActorId 的轻量级熔断器。
// Methods are generally called on actor Run goroutine (single-threaded), so no locks are needed.
// 所有方法仅在 Actor Run goroutine 中调用（单线程），无需加锁。
// recordFailure/recordSuccess 可能从 worker goroutine 调用时使用原子操作保证安全。
type circuitBreaker struct {
	state      atomic.Int32
	failures   atomic.Int32
	lastFailMs atomic.Int64
	threshold  int32
	cooldownMs int64
}

func newCircuitBreaker() *circuitBreaker {
	return &circuitBreaker{
		threshold:  defaultCBThreshold,
		cooldownMs: defaultCBCooldownMs,
	}
}

// allow returns true when request is allowed.
// allow 返回 true 表示允许发起请求。
func (cb *circuitBreaker) allow() bool {
	st := cbState(cb.state.Load())
	switch st {
	case cbClosed:
		return true
	case cbOpen:
		if ztime.ServerNowUnixMilli()-cb.lastFailMs.Load() > cb.cooldownMs {
			cb.state.CompareAndSwap(int32(cbOpen), int32(cbHalfOpen))
			return true
		}
		return false
	case cbHalfOpen:
		return true
	}
	return true
}

func (cb *circuitBreaker) recordSuccess() {
	cb.failures.Store(0)
	cb.state.Store(int32(cbClosed))
}

func (cb *circuitBreaker) recordFailure() {
	cb.lastFailMs.Store(ztime.ServerNowUnixMilli())
	n := cb.failures.Add(1)
	if n >= cb.threshold {
		cb.state.Store(int32(cbOpen))
	}
}
