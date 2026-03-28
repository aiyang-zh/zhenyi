// 以下类型与常量从 github.com/grafana/pyroscope-go 重新导出，业务代码只需 import 本包，无需直接依赖 grafana/pyroscope-go。
package zpyroscope

import (
	"context"

	"github.com/grafana/pyroscope-go"
)

// Config 与上游 pyroscope.Config 一致。
type Config = pyroscope.Config

// Profiler 为上游剖析句柄（Stop / Flush）。
type Profiler = pyroscope.Profiler

// ProfileType 选择采集哪类 profile。
type ProfileType = pyroscope.ProfileType

// Logger 与上游一致；可置 nil 由 Start 使用 StandardLogger。
type Logger = pyroscope.Logger

// LabelSet 用于 TagWrapper / Labels。
type LabelSet = pyroscope.LabelSet

// StandardLogger 为默认日志实现。
var StandardLogger = pyroscope.StandardLogger

// DefaultProfileTypes 为上游默认 profile 集合；Config.ProfileTypes 为空时使用。
var DefaultProfileTypes = pyroscope.DefaultProfileTypes

// DefaultSampleRate 为上游默认采样率常量（见 pyroscope-go 说明）。
const DefaultSampleRate = pyroscope.DefaultSampleRate

const (
	ProfileCPU           = pyroscope.ProfileCPU
	ProfileInuseObjects  = pyroscope.ProfileInuseObjects
	ProfileAllocObjects  = pyroscope.ProfileAllocObjects
	ProfileInuseSpace    = pyroscope.ProfileInuseSpace
	ProfileAllocSpace    = pyroscope.ProfileAllocSpace
	ProfileGoroutines    = pyroscope.ProfileGoroutines
	ProfileMutexCount    = pyroscope.ProfileMutexCount
	ProfileMutexDuration = pyroscope.ProfileMutexDuration
	ProfileBlockCount    = pyroscope.ProfileBlockCount
	ProfileBlockDuration = pyroscope.ProfileBlockDuration
)

// Labels 与 pprof.Labels 一致，用于 TagWrapper。
var Labels = pyroscope.Labels

// TagWrapper 在标签上下文中执行回调（与上游语义一致）。
func TagWrapper(ctx context.Context, labels LabelSet, cb func(context.Context)) {
	pyroscope.TagWrapper(ctx, labels, cb)
}
