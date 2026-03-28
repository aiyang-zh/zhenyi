package zactor

import (
	"bytes"
	"context"
	"github.com/aiyang-zh/zhenyi-base/zcoll"
	"runtime/pprof"
	"time"

	"go.uber.org/zap"

	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi/zmetrics"
)

const (
	defaultWatchdogInterval     = 200 * time.Millisecond
	defaultWatchdogCooldown     = 10 * time.Second
	defaultWatchdogMaxDumpBytes = 64 * 1024
)

// Watchdog periodically scans all actors and reports handlers blocked over threshold.
// Watchdog 周期扫描所有 Actor，并上报处理时长超过阈值的阻塞 Handler。
type Watchdog struct {
	group        *Group
	threshold    time.Duration
	interval     time.Duration
	cooldown     time.Duration
	maxDumpBytes int
	log          *zlog.Logger
	lastDump     *zcoll.SyncMap[uint64, int64] // actorId → int64 (nanos of last report)
}

func newWatchdog(group *Group, threshold time.Duration) *Watchdog {
	return &Watchdog{
		group:        group,
		threshold:    threshold,
		interval:     defaultWatchdogInterval,
		cooldown:     defaultWatchdogCooldown,
		maxDumpBytes: defaultWatchdogMaxDumpBytes,
		log:          zlog.CloneDefaultLog("watchdog"),
		lastDump:     zcoll.NewSyncMap[uint64, int64](),
	}
}

func (w *Watchdog) run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.log.Info("Watchdog started",
		zap.Duration("threshold", w.threshold),
		zap.Duration("interval", w.interval),
		zap.Duration("cooldown", w.cooldown))

	for {
		select {
		case <-ctx.Done():
			w.log.Info("Watchdog stopped")
			return
		case <-ticker.C:
			w.scan()
		}
	}
}

func (w *Watchdog) scan() {
	now := time.Now().UnixNano()

	w.group.actors.Range(func(actorId uint64, item *ActorItem) bool {
		a, ok := item.IActor.(*Actor)
		if !ok {
			return true
		}

		startNano := a.GetProcessingStart()
		if startNano == 0 {
			return true
		}

		blocked := time.Duration(now - startNano)
		if blocked < w.threshold {
			return true
		}

		if last, loaded := w.lastDump.Load(actorId); loaded {
			if time.Duration(now-last) < w.cooldown {
				return true
			}
		}
		w.lastDump.Store(actorId, now)

		var stackDump string
		truncated := false
		if p := pprof.Lookup("goroutine"); p != nil {
			var buf bytes.Buffer
			// 2 表示带更完整的堆栈信息；注意这会 dump 全量 goroutine，必须受 cooldown 保护。
			_ = p.WriteTo(&buf, 2)
			b := buf.Bytes()
			if w.maxDumpBytes > 0 && len(b) > w.maxDumpBytes {
				b = b[:w.maxDumpBytes]
				truncated = true
			}
			stackDump = string(b)
		}

		w.log.Error("BLOCKED handler detected",
			zap.Uint64("actorId", actorId),
			zap.String("actor", item.IActor.GetTopic()),
			zap.Duration("blocked", blocked),
			zap.Bool("stackTruncated", truncated),
			zap.String("goroutineStacks", stackDump),
		)

		zmetrics.ActorBlockedCount.Inc()
		return true
	})
}
