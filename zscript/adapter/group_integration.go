// Package adapter integrates Group with four script engines via zscript contract and avoids cyclic imports.
// Group 与四种脚本引擎的集成，使用 zscript 契约，通过 adapter 包注入以避免循环引用。
package adapter

import (
	"context"
	"time"

	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi/zactor"
	"github.com/aiyang-zh/zhenyi/ziface"
	"github.com/aiyang-zh/zhenyi/zjs"
	"github.com/aiyang-zh/zhenyi/zlua"
	"github.com/aiyang-zh/zhenyi/zscript"
	"github.com/aiyang-zh/zhenyi/zstarlark"
	"github.com/aiyang-zh/zhenyi/ztengo"
	"go.uber.org/zap"
)

// InitScriptEnginesForGroup initializes Lua/JS/Starlark/Tengo engines for Group.
// InitScriptEnginesForGroup 为 Group 初始化四种脚本引擎（Lua、JS、Starlark、Tengo），接入 actor 生态。
// Call once at startup; actors fetch engine via Group and call with zscript.CallParams.
// 启动时调用一次；Actor 内通过 a.GetGroup().GetScriptEngine(ziface.ScriptEngineLua) 取引擎，再构造 zscript.CallParams 并 Call。
//
// Usage example:
// 使用示例：
//
//	import "github.com/aiyang-zh/zhenyi/zscript/adapter"
//	g := zactor.NewGroup(1, false)
//	_ = adapter.InitScriptEnginesForGroup(g, "./scripts")
//	g.AddActor(myActor)
//	g.Run(ctx)
//
// In actor handler:
// 在 Actor 的 Handler 中：
//
//	eng := a.GetGroup().GetScriptEngine(ziface.ScriptEngineLua)
//	params := &zscript.CallParams{ActorID: a.GetActorId(), ActorType: a.GetActorType(), MsgID: msg.MsgId, ...}
//	result, err := eng.Call(params, "onLogin", msg.Data)
func InitScriptEnginesForGroup(group *zactor.Group, scriptBaseDir string) error {
	luaConfig := &zscript.EngineConfig{
		Type:       zscript.EngineTypeLua,
		ScriptDir:  scriptBaseDir + "/lua",
		VMPoolSize: 100,
		Timeout:    5 * time.Second,
	}
	luaEngine, err := zlua.NewEngine(luaConfig)
	if err != nil {
		zlog.Error("Failed to create Lua engine", zap.Error(err))
		return err
	}
	group.SetScriptEngine(ziface.ScriptEngineLua, NewLuaEngineAdapter(luaEngine))
	zlog.Info("Lua engine initialized", zap.String("scriptDir", luaConfig.ScriptDir))

	jsConfig := &zscript.EngineConfig{
		Type:       zscript.EngineTypeJS,
		ScriptDir:  scriptBaseDir + "/js",
		VMPoolSize: 100,
		Timeout:    5 * time.Second,
	}
	jsEngine, err := zjs.NewEngine(jsConfig)
	if err != nil {
		zlog.Error("Failed to create JS engine", zap.Error(err))
		return err
	}
	group.SetScriptEngine(ziface.ScriptEngineJS, NewJSEngineAdapter(jsEngine))
	zlog.Info("JS engine initialized", zap.String("scriptDir", jsConfig.ScriptDir))

	starlarkConfig := &zscript.EngineConfig{
		Type:      zscript.EngineTypeStarlark,
		ScriptDir: scriptBaseDir + "/starlark",
		Timeout:   5 * time.Second,
	}
	starlarkEngine, err := zstarlark.NewEngine(starlarkConfig)
	if err != nil {
		zlog.Error("Failed to create Starlark engine", zap.Error(err))
		return err
	}
	group.SetScriptEngine(ziface.ScriptEngineStarlark, NewStarlarkEngineAdapter(starlarkEngine))
	zlog.Info("Starlark engine initialized", zap.String("scriptDir", starlarkConfig.ScriptDir))

	tengoConfig := &zscript.EngineConfig{
		Type:      zscript.EngineTypeTengo,
		ScriptDir: scriptBaseDir + "/tengo",
		Timeout:   5 * time.Second,
	}
	tengoEngine, err := ztengo.NewEngine(tengoConfig)
	if err != nil {
		zlog.Error("Failed to create Tengo engine", zap.Error(err))
		return err
	}
	group.SetScriptEngine(ziface.ScriptEngineTengo, NewTengoEngineAdapter(tengoEngine))
	zlog.Info("Tengo engine initialized", zap.String("scriptDir", tengoConfig.ScriptDir))

	zlog.Info("All script engines initialized", zap.String("baseDir", scriptBaseDir))
	return nil
}

// CleanupScriptEngines closes all script engines in Group (typically on shutdown).
// CleanupScriptEngines 关闭 Group 下所有脚本引擎（服务关闭时调用）。
func CleanupScriptEngines(group *zactor.Group) {
	group.CloseScriptEngines()
	zlog.Info("All script engines closed")
}

// ExampleScriptUsage demonstrates script invocation through Group in business code.
// ExampleScriptUsage 示例：在业务中通过 Group 调用脚本。
// ctx 应该由业务在 main 中创建（例如 signal.NotifyContext）并传入。
func ExampleScriptUsage(ctx context.Context, group *zactor.Group) {
	eng := group.GetScriptEngine(ziface.ScriptEngineLua)
	if eng == nil {
		zlog.Error("Lua engine not initialized")
		return
	}
	params := &zscript.CallParams{
		ActorID: 1001, ActorType: 1, MsgID: 1001, AuthID: 123456, TraceID: 789012,
		MsgData: map[string]interface{}{"level": 10, "playerName": "张三"},
	}
	result, err := eng.Call(ctx, params, "calculateReward", 5)
	if err != nil {
		zlog.Error("Script call failed", zap.String("function", "calculateReward"), zap.Error(err))
		return
	}
	zlog.Info("Script call succeeded", zap.Any("result", result))
}

// ExampleHotReload demonstrates hot-reloading all scripts.
// ExampleHotReload 示例：热更新所有脚本。
func ExampleHotReload(group *zactor.Group) error {
	for _, typ := range []ziface.ScriptEngineType{ziface.ScriptEngineLua, ziface.ScriptEngineJS, ziface.ScriptEngineStarlark, ziface.ScriptEngineTengo} {
		eng := group.GetScriptEngine(typ)
		if eng == nil {
			continue
		}
		if err := eng.ReloadAllScripts(); err != nil {
			zlog.Error("Reload failed", zap.String("engine", string(typ)), zap.Error(err))
			return err
		}
		zlog.Info("Scripts reloaded", zap.String("engine", string(typ)))
	}
	return nil
}

// ExampleMonitorScriptEngines demonstrates periodic script-engine stats logging.
// ExampleMonitorScriptEngines 示例：周期打印脚本引擎统计。
func ExampleMonitorScriptEngines(ctx context.Context, group *zactor.Group) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, typ := range []ziface.ScriptEngineType{ziface.ScriptEngineLua, ziface.ScriptEngineJS, ziface.ScriptEngineStarlark, ziface.ScriptEngineTengo} {
				eng := group.GetScriptEngine(typ)
				if eng != nil {
					zlog.Info("Script engine stats", zap.String("engine", string(typ)), zap.Any("stats", eng.GetStats()))
				}
			}
		}
	}
}
