package zmetrics

import (
	"context"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"sync"
	"time"
)

// Go runtime metrics aligned with prometheus/collectors.NewGoCollector naming.
// Go 运行时指标（与 prometheus/collectors.NewGoCollector 对齐命名）。
var (
	// ---- Memory ----
	// ---- 内存 ----
	GoMemAllocBytes   = Global().Gauge("go_memstats_alloc_bytes", "Current bytes allocated on heap")
	GoMemSysBytes     = Global().Gauge("go_memstats_sys_bytes", "Total bytes obtained from OS")
	GoMemHeapInuse    = Global().Gauge("go_memstats_heap_inuse_bytes", "Heap bytes in use by live objects")
	GoMemHeapIdle     = Global().Gauge("go_memstats_heap_idle_bytes", "Heap bytes waiting to be used")
	GoMemHeapReleased = Global().Gauge("go_memstats_heap_released_bytes", "Heap bytes released to OS")
	GoMemStackInuse   = Global().Gauge("go_memstats_stack_inuse_bytes", "Stack bytes in use")
	GoMemAllocTotal   = Global().Counter("go_memstats_alloc_bytes_total", "Total cumulative bytes allocated")
	GoMemMallocs      = Global().Counter("go_memstats_mallocs_total", "Total mallocs")
	GoMemFrees        = Global().Counter("go_memstats_frees_total", "Total frees")

	// ---- GC ----
	GoGCCycles      = Global().Counter("go_gc_cycles_total", "Total completed GC cycles")
	GoGCPauseTotal  = Global().Gauge("go_gc_pause_total_ns", "Total GC pause time in nanoseconds")
	GoGCLastPauseNs = Global().Gauge("go_gc_last_pause_ns", "Last GC pause duration in nanoseconds")
	GoGCHeapGoal    = Global().Gauge("go_gc_heap_goal_bytes", "Target heap size for next GC")
	GoGCPercent     = Global().Gauge("go_gc_gogc_percent", "Current GOGC value")

	// ---- Goroutine / Threads ----
	// ---- Goroutine / 线程 ----
	GoGoroutines = Global().Gauge("go_goroutines", "Current number of goroutines")
	GoThreads    = Global().Gauge("go_threads", "Current number of OS threads created")
	GoMaxProcs   = Global().Gauge("go_gomaxprocs", "Current GOMAXPROCS value")
)

// StartRuntimeCollector periodically collects Go runtime metrics in background.
// StartRuntimeCollector 后台定时采集 Go 运行时指标。
// interval 建议 5~10s，与 Prometheus scrape_interval 匹配
//
// Note: collector is a global singleton and starts only once; repeated calls are ignored.
// 注意：该采集器是全局单例（只会启动一次）。重复调用会被忽略。
func StartRuntimeCollector(ctx context.Context, interval time.Duration) {
	if ctx == nil {
		return
	}
	if interval <= 0 {
		interval = 30 * time.Second
	}
	startRuntimeCollectorOnce.Do(func() {
		go collectRuntimeLoop(ctx, interval)
	})
}

var startRuntimeCollectorOnce sync.Once

func collectRuntimeLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var ms runtime.MemStats
	gcRefreshEvery := time.Minute
	lastGCRefresh := time.Now().Add(-gcRefreshEvery)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			readGC := time.Since(lastGCRefresh) >= gcRefreshEvery
			collectRuntime(&ms, readGC)
			if readGC {
				lastGCRefresh = time.Now()
			}
		}
	}
}

func collectRuntime(ms *runtime.MemStats, readGC bool) {
	runtime.ReadMemStats(ms)

	GoMemAllocBytes.Set(int64(ms.Alloc))
	GoMemSysBytes.Set(int64(ms.Sys))
	GoMemHeapInuse.Set(int64(ms.HeapInuse))
	GoMemHeapIdle.Set(int64(ms.HeapIdle))
	GoMemHeapReleased.Set(int64(ms.HeapReleased))
	GoMemStackInuse.Set(int64(ms.StackInuse))

	GoMemAllocTotal.Swap(int64(ms.TotalAlloc))
	GoMemMallocs.Swap(int64(ms.Mallocs))
	GoMemFrees.Swap(int64(ms.Frees))

	GoGCCycles.Swap(int64(ms.NumGC))
	GoGCPauseTotal.Set(int64(ms.PauseTotalNs))
	if ms.NumGC > 0 {
		lastIdx := (ms.NumGC + 255) % 256
		GoGCLastPauseNs.Set(int64(ms.PauseNs[lastIdx]))
	}
	GoGCHeapGoal.Set(int64(ms.NextGC))

	if readGC {
		// Note: debug.SetGCPercent(-1) performs global config read-modify-write; keep low-frequency refresh to reduce side effects.
		// 注意：debug.SetGCPercent(-1) 会触发全局配置读改写；这里按低频刷新，减少潜在副作用。
		gcPercent := debug.SetGCPercent(-1)
		debug.SetGCPercent(gcPercent)
		GoGCPercent.Set(int64(gcPercent))
	}

	GoGoroutines.Set(int64(runtime.NumGoroutine()))
	GoMaxProcs.Set(int64(runtime.GOMAXPROCS(0)))

	if p := pprof.Lookup("threadcreate"); p != nil {
		GoThreads.Set(int64(p.Count()))
	}
}
