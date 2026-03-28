package zmodel

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/aiyang-zh/zhenyi/zmsg"
)

// FrameworkTuning defines framework-level tuning defaults for Actor/Gate components.
// FrameworkTuning 框架级调优参数，用于 Actor/Gate 等组件的默认行为。
// Set once during bootstrap via DefaultFrameworkTuning or SetFrameworkTuning, then treat as read-only.
// 建议在进程启动时通过 zmodel.DefaultFrameworkTuning 或 SetFrameworkTuning 设置一次，运行中只读。
type FrameworkTuning struct {
	// Actor worker pool and batching knobs.
	// Actor 协程池与批处理。
	ActorWorkSizeDefault int           // Actor 默认 worker 池大小，合理区间 1~10000，建议 500（轻量）~2000（重量）
	ActorBatchMin        int           // 自适应批处理最小条数，建议 1
	ActorBatchMax        int           // 自适应批处理最大条数，合理区间 1~1000，建议 200
	ActorBatchTargetP99  time.Duration // 批处理目标 P99 延迟，用于自适应，建议 10ms
	SlowLogThreshold     time.Duration // Handler/消息处理慢日志阈值，超过即打 Warn，建议 10ms，告警可关联 zhenyi_actor_msg_latency_ms
	SlowBatchThreshold   time.Duration // 整批消息处理慢日志阈值（Run 内），建议 20ms
	// Gate RTT tracking knobs.
	// Gate RTT 追踪。
	RTTBufferSize uint32 // RTT 槽位数量（会向上取整为 2 的幂），建议 16384，影响并发连接数
	RTTMaxSamples uint32 // RTT 采样保存数量，用于统计/上报，建议 100000
}

// DefaultFrameworkTuning is default framework tuning profile.
// DefaultFrameworkTuning 默认框架调优，业务可在 init 或 main 中替换为自定义值。
var DefaultFrameworkTuning = FrameworkTuning{
	ActorWorkSizeDefault: 500,
	ActorBatchMin:        1,
	ActorBatchMax:        200,
	ActorBatchTargetP99:  10 * time.Millisecond,
	SlowLogThreshold:     10 * time.Millisecond,
	SlowBatchThreshold:   20 * time.Millisecond,
	RTTBufferSize:        16384,
	RTTMaxSamples:        100000,
}

var frameworkTuningValue atomic.Value // FrameworkTuning

func init() {
	frameworkTuningValue.Store(DefaultFrameworkTuning)
}

// SetFrameworkTuning sets framework tuning during bootstrap.
// SetFrameworkTuning 在进程启动阶段设置框架调优参数。
// Convention: call only at startup and keep read-only at runtime.
// 约定：只在启动期调用，运行中只读。
func SetFrameworkTuning(t FrameworkTuning) {
	frameworkTuningValue.Store(t)
}

// GetFrameworkTuning returns current framework tuning (concurrency-safe, by value).
// GetFrameworkTuning 返回当前使用的框架调优（并发安全，返回副本）。
func GetFrameworkTuning() FrameworkTuning {
	if v := frameworkTuningValue.Load(); v != nil {
		return v.(FrameworkTuning)
	}
	return DefaultFrameworkTuning
}

// ActorModeConfig defines actor execution mode configuration.
// ActorModeConfig Actor 模式配置。
type ActorModeConfig struct {
	Mode               int `json:"mode" mapstructure:"mode"`                                       // 0=顺序 1=并发
	ConcurrentPoolSize int `json:"concurrentPoolSize,omitempty" mapstructure:"concurrentPoolSize"` // 协程池大小
	ConcurrentMaxBatch int `json:"concurrentMaxBatch,omitempty" mapstructure:"concurrentMaxBatch"` // 最大批次
}

// IsSequential reports whether mode is sequential.
// IsSequential 是否为顺序模式。
func (c ActorModeConfig) IsSequential() bool {
	return c.Mode == 0
}

// IsConcurrent reports whether mode is concurrent.
// IsConcurrent 是否为并发模式。
func (c ActorModeConfig) IsConcurrent() bool {
	return c.Mode == 1
}

// GetPoolSize returns concurrency pool size with default fallback.
// GetPoolSize 获取协程池大小（有默认值）。
func (c ActorModeConfig) GetPoolSize() int {
	if c.ConcurrentPoolSize <= 0 {
		return 100
	}
	return c.ConcurrentPoolSize
}

// GetMaxBatch returns max batch size with default fallback.
// GetMaxBatch 获取最大批次大小（有默认值）。
func (c ActorModeConfig) GetMaxBatch() int {
	if c.ConcurrentMaxBatch <= 0 {
		return 50
	}
	return c.ConcurrentMaxBatch
}

// ActorConfig describes one actor instance configuration.
// ActorConfig 描述一个 Actor 实例的配置。
type ActorConfig struct {
	Id              uint64          `json:"id"`        // id
	Process         uint32          `json:"process"`   // 进程
	Name            string          `json:"name"`      // 名称
	ActorType       uint32          `json:"actorType"` // 类型
	Index           uint32          `json:"index"`     // 索引
	Addr            string          `json:"addr"`      // 地址
	IsLimiter       bool            `json:"isLimiter"`
	Rate            int             `json:"rate"`
	Burst           int             `json:"burst"`
	WorkSize        uint32          `json:"workSize"`
	MaxRPCPending   uint32          `json:"maxRPCPending"`             // RPC 并发槽数（0=默认4096，必须是 2 的幂）
	MaxRestarts     uint32          `json:"maxRestarts"`               // Actor 崩溃后最大重启次数（0=默认3）
	ModeConfig      ActorModeConfig `json:"modeConfig"`                // 执行模式配置
	SupportedMsgIDs []int32         `json:"supportedMsgIds,omitempty"` // 当前 Actor 实例支持的 msgId 集合（用于路由/发现）
}

// GetTopic returns unique topic used for cross-process messaging.
// GetTopic 返回用于跨进程消息的唯一 topic。
func (a ActorConfig) GetTopic() string {
	return fmt.Sprintf("topic_%d_%d_%d", a.ActorType, a.Index, a.Id)
}

// GetNameTopic returns per-actorType shared topic name.
// GetNameTopic 返回同一 actorType 共享的 topic 名称。
func (a ActorConfig) GetNameTopic() string {
	return fmt.Sprintf("topic_name_%d", a.ActorType)
}

// GetActorId returns actor ID.
// GetActorId 返回 Actor ID。
func (a ActorConfig) GetActorId() uint64 {
	return a.Id
}

// GetActorType returns actor type.
// GetActorType 返回 Actor 类型。
func (a ActorConfig) GetActorType() uint32 {
	return a.ActorType
}

// ActorServerRegister is the discovery registration payload.
// ActorServerRegister 用于服务发现的注册载荷。
type ActorServerRegister struct {
	Key         string      `json:"key"`
	Count       int32       `json:"count"`
	Weight      int32       `json:"weight"`
	ActorConfig ActorConfig `json:"actor"`
}

// CmdType is Actor command type.
// CmdType Actor 命令类型。
type CmdType = uint8

const (
	CmdTypeMsg    CmdType = 0 // 网络消息
	CmdTypeTick   CmdType = 1 // 定时器 Tick
	CmdTypeSafeFn CmdType = 2 // 线程安全的内部闭包
	CmdTypeTickFn CmdType = 3 // 定时回调注册
	CmdTypeClient CmdType = 4 // 客户端消息
	CmdTypeAsync  CmdType = 5 // Actor 主线程执行异步回调任务
)

// ActorCmd is queue envelope (value-type enqueue to reduce GC pressure).
// ActorCmd 信封结构体（值类型入队，减少 GC 压力）。
// TickFn is pointer for CmdTypeTickFn only, avoiding ~64-byte per-message copy.
// TickFn 改为指针：仅 CmdTypeTickFn 使用，避免每条消息拷贝 ~64 bytes。
type ActorCmd struct {
	TickNow int64           // CmdType_Tick 时间戳
	Msg     *zmsg.Message   // CmdTypeMsg / CmdTypeClient
	Ctx     context.Context // 可选：携带 trace/cancel 链，nil 时使用 Actor 默认 ctx
	TickFn  *TickFnItem     // CmdTypeTickFn（指针，减小 ActorCmd 体积）
	Fn      func()          // CmdType_SafeFn
	Any     interface{}     // CmdTypeAsync 承载内部任务对象
	Type    uint8           // 消息类型
}

// Release releases retained message in command envelope (if any).
// Release 释放命令信封中持有的消息（若存在）。
func (c *ActorCmd) Release() {
	if c.Msg != nil {
		c.Msg.Release()
		c.Msg = nil
	}
}

// Retain retains message in command envelope (if any).
// Retain 对命令信封中持有的消息做 Retain（若存在）。
func (c *ActorCmd) Retain() {
	if c.Msg != nil {
		c.Msg.Retain()
	}
}
