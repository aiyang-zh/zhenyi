package ziface

import (
	"context"
	"github.com/aiyang-zh/zhenyi/zmodel"
)

// ScriptEngineType identifies script engine kinds for Get/Set APIs.
// ScriptEngineType 脚本引擎类型，用于 GetScriptEngine/SetScriptEngine，避免魔法字符串。
type ScriptEngineType string

const (
	// ScriptEngineLua indicates Lua engine.
	// ScriptEngineLua 表示 Lua 引擎。
	ScriptEngineLua ScriptEngineType = "lua"
	// ScriptEngineJS indicates JavaScript engine.
	// ScriptEngineJS 表示 JavaScript 引擎。
	ScriptEngineJS ScriptEngineType = "javascript"
	// ScriptEngineStarlark indicates Starlark engine.
	// ScriptEngineStarlark 表示 Starlark 引擎。
	ScriptEngineStarlark ScriptEngineType = "starlark"
	// ScriptEngineTengo indicates Tengo engine.
	// ScriptEngineTengo 表示 Tengo 引擎。
	ScriptEngineTengo ScriptEngineType = "tengo"
)

// IScriptEngine defines a unified script engine contract.
// IScriptEngine 脚本引擎接口（兼容所有引擎类型）。
// Note: this is a simplified contract for Group-level management.
// 注意：此接口是简化版本，用于 Group 统一管理。
type IScriptEngine interface {
	// Call invokes a script function.
	// Call 执行脚本函数。
	// params is typically *zscript.CallParams (implementation may type-assert).
	// params: *zscript.CallParams 类型的参数（实际引擎实现中会进行类型断言）。
	Call(ctx context.Context, params interface{}, function string, args ...interface{}) (interface{}, error)

	LoadScript(path string) error
	LoadScripts(paths []string) error
	ReloadScript(path string) error
	ReloadAllScripts() error
	GetStats() interface{}
	GetType() string
	Close()
}

// IGroup defines management operations for a group of Actors.
// IGroup Actor 组接口：管理一组 Actor 的注册、查找、Run、发现与脚本引擎等。
// Implemented by zactor.Group.
// 实现体为 zactor.Group。
type IGroup interface {
	AddActor(iActor IActor)
	GetActorById(actorId uint64) IActor
	GetOtherActorById(actorId uint64) (zmodel.ActorConfig, bool)
	Run(ctx context.Context) error
	GetDiscoverer() Discoverer
	SetDiscoverer(discover Discoverer)
	IsSingle() bool
	GetActorCh() chan IActor
	FindPoolActorByType(actorType uint32) (zmodel.ActorConfig, error)

	// In-process routing: register supported msgIds and query candidates.
	// 进程内路由：RegisterRoutes 注册 Actor 支持的 msgId；LookupActorsByMsgID 供 Router/Gate 查询候选。
	RegisterRoutes(actor IActor, msgIDs []int32)
	LookupActorsByMsgID(msgID int32) []IActor
	// GetOtherActorConfigs returns other-process Actor config snapshot.
	// GetOtherActorConfigs 返回当前进程视角下其他进程的 Actor 配置快照（用于跨进程路由）。
	GetOtherActorConfigs() []zmodel.ActorConfig

	// Script engine management (use ScriptEngineType constants).
	// 脚本引擎管理（使用 ScriptEngineType 常量，如 ziface.ScriptEngineLua）。
	GetScriptEngine(engineType ScriptEngineType) IScriptEngine
	CloseScriptEngines()
}

// IGroupRouteTableView is an optional IGroup extension for zero-allocation local route lookup.
// IGroupRouteTableView 是 IGroup 的可选扩展接口：提供进程内路由表的只读视图，用于路由热路径零分配读取。
// Callers must not mutate the returned slice; use IGroup.LookupActorsByMsgID for mutable copies.
// 调用方不得修改返回切片；需要可变副本时应使用 IGroup.LookupActorsByMsgID。
type IGroupRouteTableView interface {
	LookupActorsByMsgIDView(msgID int32) []IActor
}

// IGroupRemoteRouteTableView is an optional IGroup extension for zero-allocation remote candidate lookup.
// IGroupRemoteRouteTableView 是 IGroup 的可选扩展接口：提供跨进程候选表的只读视图，用于 Gate 远程路由热路径零分配读取。
// Callers must not mutate the returned slice; use copy-returning method when mutation is needed.
// 调用方不得修改返回切片；需要可变副本时应使用实现方的副本方法。
type IGroupRemoteRouteTableView interface {
	LookupOtherActorConfigsByMsgIDView(msgID int32) []zmodel.ActorConfig
}
