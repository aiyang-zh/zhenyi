// Package adapter provides adapters for four script engines, avoiding cyclic imports between zscript and engine packages.
// Package adapter 提供四种脚本引擎适配器，避免 zscript 与 zlua/zjs/ztengo/zstarlark 循环引用。
// Import github.com/aiyang-zh/zhenyi/zscript/adapter, then create adapters and inject into Group.
// 使用方请 import github.com/aiyang-zh/zhenyi/zscript/adapter，再调用 NewLuaEngineAdapter 等并注入 Group。
package adapter

import (
	"context"
	"github.com/aiyang-zh/zhenyi/ziface"
	"github.com/aiyang-zh/zhenyi/zjs"
	"github.com/aiyang-zh/zhenyi/zlua"
	"github.com/aiyang-zh/zhenyi/zscript"
	"github.com/aiyang-zh/zhenyi/zstarlark"
	"github.com/aiyang-zh/zhenyi/ztengo"
)

// -------- Lua --------
// LuaEngineAdapter adapts zlua.Engine to ziface.IScriptEngine.
// LuaEngineAdapter 将 zlua.Engine 适配到 ziface.IScriptEngine。
type LuaEngineAdapter struct {
	engine *zlua.Engine
}

// NewLuaEngineAdapter creates Lua script-engine adapter.
// NewLuaEngineAdapter 创建 Lua 脚本引擎适配器。
func NewLuaEngineAdapter(engine *zlua.Engine) ziface.IScriptEngine {
	return &LuaEngineAdapter{engine: engine}
}

func (a *LuaEngineAdapter) Call(ctx context.Context, params interface{}, function string, args ...interface{}) (interface{}, error) {
	callParams, ok := params.(*zscript.CallParams)
	if !ok {
		return nil, zscript.ErrInvalidArgument
	}
	return a.engine.Call(ctx, callParams, function, args...)
}

func (a *LuaEngineAdapter) LoadScript(path string) error     { return a.engine.LoadScript(path) }
func (a *LuaEngineAdapter) LoadScripts(paths []string) error { return a.engine.LoadScripts(paths) }
func (a *LuaEngineAdapter) ReloadScript(path string) error   { return a.engine.ReloadScript(path) }
func (a *LuaEngineAdapter) ReloadAllScripts() error          { return a.engine.ReloadAllScripts() }
func (a *LuaEngineAdapter) GetStats() interface{}            { return a.engine.GetStats() }
func (a *LuaEngineAdapter) GetType() string                  { return a.engine.GetType() }
func (a *LuaEngineAdapter) Close()                           { a.engine.Close() }

// -------- JavaScript --------
// JSEngineAdapter adapts zjs.Engine to ziface.IScriptEngine.
// JSEngineAdapter 将 zjs.Engine 适配到 ziface.IScriptEngine。
type JSEngineAdapter struct {
	engine *zjs.Engine
}

// NewJSEngineAdapter creates JavaScript script-engine adapter.
// NewJSEngineAdapter 创建 JavaScript 脚本引擎适配器。
func NewJSEngineAdapter(engine *zjs.Engine) ziface.IScriptEngine {
	return &JSEngineAdapter{engine: engine}
}

func (a *JSEngineAdapter) Call(ctx context.Context, params interface{}, function string, args ...interface{}) (interface{}, error) {
	callParams, ok := params.(*zscript.CallParams)
	if !ok {
		return nil, zscript.ErrInvalidArgument
	}
	return a.engine.Call(ctx, callParams, function, args...)
}

func (a *JSEngineAdapter) LoadScript(path string) error     { return a.engine.LoadScript(path) }
func (a *JSEngineAdapter) LoadScripts(paths []string) error { return a.engine.LoadScripts(paths) }
func (a *JSEngineAdapter) ReloadScript(path string) error   { return a.engine.ReloadScript(path) }
func (a *JSEngineAdapter) ReloadAllScripts() error          { return a.engine.ReloadAllScripts() }
func (a *JSEngineAdapter) GetStats() interface{}            { return a.engine.GetStats() }
func (a *JSEngineAdapter) GetType() string                  { return a.engine.GetType() }
func (a *JSEngineAdapter) Close()                           { a.engine.Close() }

// -------- Starlark --------
// StarlarkEngineAdapter adapts zstarlark.Engine to ziface.IScriptEngine.
// StarlarkEngineAdapter 将 zstarlark.Engine 适配到 ziface.IScriptEngine。
type StarlarkEngineAdapter struct {
	engine *zstarlark.Engine
}

// NewStarlarkEngineAdapter creates Starlark script-engine adapter.
// NewStarlarkEngineAdapter 创建 Starlark 脚本引擎适配器。
func NewStarlarkEngineAdapter(engine *zstarlark.Engine) ziface.IScriptEngine {
	return &StarlarkEngineAdapter{engine: engine}
}

func (a *StarlarkEngineAdapter) Call(ctx context.Context, params interface{}, function string, args ...interface{}) (interface{}, error) {
	callParams, ok := params.(*zscript.CallParams)
	if !ok {
		return nil, zscript.ErrInvalidArgument
	}
	return a.engine.Call(ctx, callParams, function, args...)
}

func (a *StarlarkEngineAdapter) LoadScript(path string) error     { return a.engine.LoadScript(path) }
func (a *StarlarkEngineAdapter) LoadScripts(paths []string) error { return a.engine.LoadScripts(paths) }
func (a *StarlarkEngineAdapter) ReloadScript(path string) error   { return a.engine.ReloadScript(path) }
func (a *StarlarkEngineAdapter) ReloadAllScripts() error          { return a.engine.ReloadAllScripts() }
func (a *StarlarkEngineAdapter) GetStats() interface{}            { return a.engine.GetStats() }
func (a *StarlarkEngineAdapter) GetType() string                  { return a.engine.GetType() }
func (a *StarlarkEngineAdapter) Close()                           { a.engine.Close() }

// -------- Tengo --------
// TengoEngineAdapter adapts ztengo.Engine to ziface.IScriptEngine.
// TengoEngineAdapter 将 ztengo.Engine 适配到 ziface.IScriptEngine。
type TengoEngineAdapter struct {
	engine *ztengo.Engine
}

// NewTengoEngineAdapter creates Tengo script-engine adapter.
// NewTengoEngineAdapter 创建 Tengo 脚本引擎适配器。
func NewTengoEngineAdapter(engine *ztengo.Engine) ziface.IScriptEngine {
	return &TengoEngineAdapter{engine: engine}
}

func (a *TengoEngineAdapter) Call(ctx context.Context, params interface{}, function string, args ...interface{}) (interface{}, error) {
	callParams, ok := params.(*zscript.CallParams)
	if !ok {
		return nil, zscript.ErrInvalidArgument
	}
	return a.engine.Call(ctx, callParams, function, args...)
}

func (a *TengoEngineAdapter) LoadScript(path string) error     { return a.engine.LoadScript(path) }
func (a *TengoEngineAdapter) LoadScripts(paths []string) error { return a.engine.LoadScripts(paths) }
func (a *TengoEngineAdapter) ReloadScript(path string) error   { return a.engine.ReloadScript(path) }
func (a *TengoEngineAdapter) ReloadAllScripts() error          { return a.engine.ReloadAllScripts() }
func (a *TengoEngineAdapter) GetStats() interface{}            { return a.engine.GetStats() }
func (a *TengoEngineAdapter) GetType() string                  { return a.engine.GetType() }
func (a *TengoEngineAdapter) Close()                           { a.engine.Close() }
