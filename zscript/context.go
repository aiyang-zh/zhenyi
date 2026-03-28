package zscript

import "github.com/aiyang-zh/zhenyi-base/ztime"

// ScriptContext is runtime execution context passed into scripts.
// ScriptContext 脚本执行上下文。
// 传递给脚本的运行时环境信息。
// ⚠️ 注意: 所有类型与框架保持一致（model.ActorConfig 和 model.Message）
type ScriptContext struct {
	// ActorID Actor 唯一 ID（对应 ActorConfig.Id）
	ActorID uint64

	// ActorType Actor 类型（对应 ActorConfig.ActorType）
	ActorType uint32

	// MsgID 消息 ID（Protobuf 消息类型）
	MsgID int32

	// AuthID 认证ID（玩家ID等）
	AuthID int64

	// MsgData 消息数据（原始数据，业务层自行解析）。
	// 约定：
	//   - 来自网络的 Message.Data 通过 CallParams 注入时为 []byte（只读视图）。
	//   - 上层若需结构化对象，应在脚本内部或中间层自行解码，不修改底层缓冲。
	MsgData interface{}

	// Owner 持有当前运行脚本的 Actor 实例（或其他宿主对象）
	// ⚠️ 注意: 使用 interface{} 避免引入具体 actor 包导致的循环依赖
	// 在具体的 Engine 实现或业务层中，再将其断言为 *actor.Actor
	// 例如: actor := ctx.Owner.(*actor.Actor)
	Owner interface{}

	// TraceID 链路追踪 ID（用于日志关联）
	// 注意: 使用 uint64 类型，与框架的 Message.TraceId 保持一致
	// ⚠️ 应该从消息中继承，而不是重新生成
	TraceID uint64

	// NowMillis 当前时间戳（Unix 毫秒）
	// 注意: 使用时间戳而不是 time.Time，避免跨语言日期对象转换问题
	// 如需秒级时间戳，脚本中可计算: Math.floor(ctx.NowMillis / 1000) 或 ctx.NowMillis / 1000
	NowMillis int64

	// Metadata 扩展元数据（业务自定义）
	// ⚠️ 注意: Reset 时会设为 nil，使用前需要 Lazy Init
	Metadata map[string]interface{}
}

// init initializes script context for object-pool reuse.
// init 初始化脚本上下文（用于对象池复用）。
// Package-private: used only in GetContext, not for business direct calls.
// ⚠️ 包内可见：只在 GetContext 中使用，业务层不应直接调用。
func (c *ScriptContext) init(actorID uint64, actorType uint32) *ScriptContext {
	c.ActorID = actorID
	c.ActorType = actorType
	c.TraceID = 0                            // ⚠️ 默认为 0，调用者应该通过 WithTraceID 传入
	c.NowMillis = ztime.ServerNowUnixMilli() // 使用 zhenyi-base 统一时间（支持时间偏移）
	c.Metadata = nil                         // ⚠️ 初始为 nil，Lazy Init（减少无谓的 make）
	c.Owner = nil
	return c
}

// WithOwner sets Owner (actor instance).
// WithOwner 设置 Owner（Actor 实例）。
func (c *ScriptContext) WithOwner(owner interface{}) *ScriptContext {
	c.Owner = owner
	return c
}

// WithMessage sets message information.
// WithMessage 设置消息信息。
func (c *ScriptContext) WithMessage(msgID int32, authID int64, msgData interface{}) *ScriptContext {
	c.MsgID = msgID
	c.AuthID = authID
	c.MsgData = msgData
	return c
}

// WithTraceID sets TraceID propagated from message.
// WithTraceID 设置 TraceID（从消息中传递）。
func (c *ScriptContext) WithTraceID(traceID uint64) *ScriptContext {
	c.TraceID = traceID
	return c
}

// WithMetadata sets metadata with lazy initialization.
// WithMetadata 设置元数据。
// Lazy init avoids unnecessary map allocation.
// ⚠️ 注意: Lazy Init，防止无谓的 map 分配。
func (c *ScriptContext) WithMetadata(key string, value interface{}) *ScriptContext {
	if c.Metadata == nil {
		c.Metadata = make(map[string]interface{})
	}
	c.Metadata[key] = value
	return c
}

// reset clears context for object-pool reuse.
// reset 重置上下文（用于对象池复用）。
// Package-private: used only in PutContext, not for business direct calls.
// ⚠️ 包内可见：只在 PutContext 中使用，业务层不应直接调用。
func (c *ScriptContext) reset() {
	c.ActorID = 0
	c.ActorType = 0
	c.MsgID = 0
	c.AuthID = 0
	c.MsgData = nil
	c.Owner = nil // Clear Owner reference to avoid memory leaks / ✅ 清理 Owner 引用，防止内存泄漏
	c.TraceID = 0
	c.NowMillis = 0
	// ✅ Discard the old map to avoid keeping a large map in memory.
	// ✅ 直接丢弃旧 map，防止大 map 长期占用内存
	// Created on next use via WithMetadata's Lazy Init.
	// 下次使用时通过 WithMetadata 的 Lazy Init 按需创建
	c.Metadata = nil
}
