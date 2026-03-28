// Package zpyroscope wraps Grafana Pyroscope (github.com/grafana/pyroscope-go) for optional
// continuous CPU/heap profiling in zhenyi applications. It is separate from zmetrics (Prometheus):
// import this package only when you need profiling; otherwise no Pyroscope code is linked.
//
// 本包为可选依赖：仅在被 import 时链接 Pyroscope 客户端；与 zmetrics 指标服务解耦。
// 类型与常量见 export.go，业务无需再 import github.com/grafana/pyroscope-go。
package zpyroscope

import (
	"context"
	"sync"

	"github.com/grafana/pyroscope-go"
)

// Start 启动剖析，并对空 Logger / 空 ProfileTypes 填入默认值。
// 须调用 Profiler.Stop 结束（通常在 main 中 defer）。
func Start(cfg Config) (*Profiler, error) {
	return pyroscope.Start(normalize(cfg))
}

// StartWithContext 在 ctx 取消时停止剖析；返回的 stop 可幂等调用。
func StartWithContext(ctx context.Context, cfg Config) (stop func(), err error) {
	prof, err := pyroscope.Start(normalize(cfg))
	if err != nil {
		return nil, err
	}
	var once sync.Once
	stopOnce := func() {
		once.Do(func() {
			_ = prof.Stop()
		})
	}
	go func() {
		<-ctx.Done()
		stopOnce()
	}()
	return stopOnce, nil
}

func normalize(cfg Config) pyroscope.Config {
	if cfg.Logger == nil {
		cfg.Logger = StandardLogger
	}
	if len(cfg.ProfileTypes) == 0 {
		cfg.ProfileTypes = DefaultProfileTypes
	}
	return cfg
}
