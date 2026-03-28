package zactor

import (
	"github.com/panjf2000/ants/v2"
	"go.uber.org/zap"

	"github.com/aiyang-zh/zhenyi/zmetrics"
)

// UpdateWorkerPoolSize adjusts async worker pool size at runtime.
// UpdateWorkerPoolSize 运行时调整异步业务协程池大小。
// It runs on actor main thread through SafeFn and is thread-safe.
// 通过 SafeFn 在 Actor 主线程执行，线程安全。
func (a *Actor) UpdateWorkerPoolSize(newSize int) {
	a.safeUpdate(func() {
		if a.workerPool == nil || newSize <= 0 {
			return
		}
		a.workerPool.Tune(newSize)
		a.GetLogger().Info("Worker pool size updated",
			zap.Int("newSize", newSize),
			zap.Int("cap", a.workerPool.Cap()))
	})
}

// UpdateRateLimit adjusts rate-limiting parameters at runtime.
// UpdateRateLimit 运行时调整限流参数。
// Rate and Burst are stored in ActorConfig for business-layer usage.
// Rate 和 Burst 存储在 ActorConfig 中，由业务层读取使用。
func (a *Actor) UpdateRateLimit(rate, burst int) {
	a.safeUpdate(func() {
		a.Rate = rate
		a.Burst = burst
		a.GetLogger().Info("Rate limit updated",
			zap.Int("rate", rate),
			zap.Int("burst", burst))
	})
}

// UpdateMaxRPCPending adjusts RPC slot count at runtime.
// UpdateMaxRPCPending 运行时调整 RPC 并发槽数。
// Note: sender slot array is fixed-size, so this requires sender rebuild.
// 注意：由于 sender 的 slot 数组是固定大小的，此方法需要重建 sender。
// Safe only when no in-flight RPC exists.
// 仅在当前无进行中 RPC 时安全执行。
func (a *Actor) UpdateMaxRPCPending(newSize uint32) {
	a.safeUpdate(func() {
		if newSize <= 0 {
			return
		}
		a.MaxRPCPending = newSize
		a.GetLogger().Info("MaxRPCPending updated (effective after sender rebuild)",
			zap.Uint32("newSize", newSize))
	})
}

// RebuildWorkerPool rebuilds the worker pool completely for extreme cases.
// RebuildWorkerPool 完全重建协程池（极端情况使用，如检测到池死锁）。
func (a *Actor) RebuildWorkerPool(poolSize int) {
	a.safeUpdate(func() {
		newPool, err := ants.NewPoolWithFunc(poolSize, func(arg interface{}) {
			switch task := arg.(type) {
			case *asyncTask:
				if task == nil {
					return
				}
				if task.msg != nil && task.fMsg != nil {
					task.runWithMsg()
					return
				}
				task.runSimple()
			}
		},
			ants.WithPreAlloc(true),     // Pre-allocate to reduce runtime allocations / 预分配，减少运行时分配
			ants.WithNonblocking(false), // Blocking when pool is full / 阻塞模式：池满时等待，而不是返回错误
			ants.WithPanicHandler(func(err interface{}) {
				zmetrics.ActorPanicCount.Inc()
				a.GetLogger().Error("RebuildWorkerPool panic recovered",
					zap.Any("panic", err))
			}),
		)
		if err != nil {
			return
		}
		oldPool := a.workerPool
		a.workerPool = newPool
		if oldPool != nil {
			oldPool.Release()
		}
		a.GetLogger().Info("Worker pool rebuilt",
			zap.Int("newSize", poolSize))
	})
}
