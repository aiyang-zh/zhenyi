package zscript

import (
	"context"
	"time"
)

// IScriptEngine defines the unified scripting engine contract.
// IScriptEngine 脚本引擎统一接口。
// It abstracts Lua, JavaScript, and Starlark engines.
// 提供 Lua、JavaScript、Starlark 三种引擎的抽象。
type IScriptEngine interface {
	// LoadScript loads a single script file.
	// LoadScript 加载单个脚本文件。
	// path: 脚本文件路径（相对或绝对）
	// Returns: error when compilation fails.
	// 返回: 错误信息（如果编译失败）。
	LoadScript(path string) error

	// LoadScripts loads scripts in batch (atomic operation).
	// LoadScripts 批量加载脚本（原子操作）。
	// paths: 脚本文件路径列表
	// Returns: error (any failure aborts whole batch to keep atomicity).
	// 返回: 错误信息（任何一个失败都会中止，保证原子性）。
	// Note: lock-free reads + copy-on-write updates via atomic.Value.
	// 注意: 使用 atomic.Value 实现无锁读取 + Copy-on-Write 更新。
	LoadScripts(paths []string) error

	// ReloadScript reloads one script file (hot update).
	// ReloadScript 重新加载单个脚本（热更新）。
	// path: 脚本文件路径
	// Returns: error when compilation fails.
	// 返回: 错误信息（如果编译失败）。
	ReloadScript(path string) error

	// ReloadAllScripts reloads all scripts (hot update).
	// ReloadAllScripts 重新加载所有脚本（热更新）。
	// Returns: error.
	// 返回: 错误信息。
	ReloadAllScripts() error

	// Call invokes a script function.
	// Call 执行脚本函数。
	// params: 调用参数（包含 ActorID、TraceID、Owner 等）
	// function: 脚本函数名（如 "handle_login"）
	// args: 函数参数（可变参数）
	// Returns: execution result and error.
	// 返回: 执行结果、错误信息。
	// Notes:
	// 注意:
	//   1. 支持超时中断（默认 5s）
	//   2. 引擎内部管理 ScriptContext 对象池，业务层无需关心
	Call(ctx context.Context, params *CallParams, function string, args ...interface{}) (interface{}, error)

	// GetStats returns engine statistics.
	// GetStats 获取引擎统计信息。
	// Returns: statistics (call count, error count, latency, etc.).
	// 返回: 统计数据（调用次数、错误次数、延迟等）。
	GetStats() *EngineStats

	// GetType returns engine type.
	// GetType 获取引擎类型。
	// Returns: "lua" | "javascript" | "starlark".
	// 返回: "lua" | "javascript" | "starlark"。
	GetType() string

	// Close closes engine and releases resources.
	// Close 关闭引擎，释放资源。
	// Note: engine cannot be used after Close.
	// 注意: 调用后引擎不可再使用。
	Close()
}

// CallParams contains parameters for Call.
// CallParams Call 方法的参数。
// Business layer fills this struct from actor/message context.
// 业务层从 actor 和 msg 中提取信息填充此结构体。
// ⚠️ 注意: 所有类型与框架保持一致（model.ActorConfig 和 model.Message）
type CallParams struct {
	// ActorID Actor 唯一 ID（必填，对应 ActorConfig.Id）
	ActorID uint64

	// ActorType Actor 类型（必填，对应 ActorConfig.ActorType）
	ActorType uint32

	// Owner Actor 实例引用（可选，用于脚本调用 Go API）
	Owner interface{}

	// TraceID 链路追踪 ID（可选，默认 0，对应 Message.TraceId）
	TraceID uint64

	// MsgID 消息 ID（可选，默认 0，对应 Message.MsgId）
	MsgID int32

	// AuthID 认证 ID（可选，默认 0，对应 Message.AuthId）
	AuthID int64

	// MsgData 消息数据（可选，nil 表示无数据，对应 Message.Data）。
	// Conventions:
	// 约定：
	//   - 框架从网络消息构造 CallParams 时，MsgData 一律传入底层的 []byte（只读视图）。
	//   - 业务/测试自行构造 CallParams 时，可以传入其他类型（map/struct 等），由各引擎自行解释。
	//   - 引擎实现不得在原位修改来自框架的 []byte，避免与对象池/零拷贝语义冲突。
	MsgData interface{}

	// Metadata 扩展元数据（可选，用于传递额外的业务数据）
	Metadata map[string]interface{}
}

// EngineType is the scripting engine type enum.
// EngineType 引擎类型枚举。
type EngineType string

const (
	// EngineTypeLua identifies Lua engine.
	// EngineTypeLua 标识 Lua 引擎。
	EngineTypeLua EngineType = "lua"
	// EngineTypeJS identifies JavaScript engine.
	// EngineTypeJS 标识 JavaScript 引擎。
	EngineTypeJS EngineType = "javascript"
	// EngineTypeStarlark identifies Starlark engine.
	// EngineTypeStarlark 标识 Starlark 引擎。
	EngineTypeStarlark EngineType = "starlark"
	// EngineTypeTengo identifies Tengo engine.
	// EngineTypeTengo 标识 Tengo 引擎。
	EngineTypeTengo EngineType = "tengo"
)

// EngineConfig configures script engine behavior.
// EngineConfig 引擎配置。
type EngineConfig struct {
	// Type is engine type.
	// Type 引擎类型。
	Type EngineType

	// ScriptDir is script directory.
	// ScriptDir 脚本目录。
	ScriptDir string

	// Timeout is script execution timeout (default 5s).
	// Timeout 脚本执行超时时间（默认 5s）。
	Timeout time.Duration

	// MaxMemory is memory limit in bytes (0 means unlimited).
	// MaxMemory 最大内存限制（字节，0 表示不限制）。
	MaxMemory int64

	// EnableInstructionCount enables instruction-count circuit break (debug mode).
	// EnableInstructionCount 是否启用指令计数熔断（Debug 模式）。
	EnableInstructionCount bool

	// MaxInstructions is max instruction count when instruction counting is enabled.
	// MaxInstructions 最大指令数（仅在 EnableInstructionCount=true 时有效）。
	MaxInstructions int64

	// VMPoolSize is VM pool size (Lua/JS only, 0 uses default 100).
	// VMPoolSize VM 池大小（仅 Lua/JS 使用，0 表示使用默认值 100）。
	VMPoolSize int

	// MaxVMUseCount is max VM reuse count (0 means unlimited, default 1000).
	// MaxVMUseCount VM 最大使用次数（超过后淘汰，0 表示不限制，默认 1000）。
	MaxVMUseCount int

	// MaxVMAge is max VM lifetime (0 means unlimited, default 5 minutes).
	// MaxVMAge VM 最大存活时间（超过后淘汰，0 表示不限制，默认 5 分钟）。
	MaxVMAge time.Duration

	// AllowRequireWithoutScriptDir (JavaScript engine only) restores legacy require() resolution
	// when ScriptDir is empty (no directory sandbox; paths follow filepath.Abs). Default false:
	// require() is rejected unless ScriptDir is set, so production defaults stay safe.
	// AllowRequireWithoutScriptDir（仅 JS 引擎）在 ScriptDir 为空时允许历史行为（无目录沙箱）。
	// 默认 false：未配置 ScriptDir 时 require 会直接失败。
	AllowRequireWithoutScriptDir bool
}

// DefaultEngineConfig returns default engine config.
// DefaultEngineConfig 默认引擎配置。
func DefaultEngineConfig() *EngineConfig {
	return &EngineConfig{
		Type:                   EngineTypeJS,
		ScriptDir:              "./scripts",
		Timeout:                5 * time.Second,
		MaxMemory:              0,
		EnableInstructionCount: false,
		MaxInstructions:        1000000,
		VMPoolSize:             100,
		MaxVMUseCount:          1000,            // VM 最多使用 1000 次
		MaxVMAge:               5 * time.Minute, // VM 最多存活 5 分钟
	}
}
