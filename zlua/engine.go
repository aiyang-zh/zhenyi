package zlua

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	lua "github.com/yuin/gopher-lua"
	"go.uber.org/zap"

	"github.com/aiyang-zh/zhenyi-base/zerrs"
	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi-base/zpool"
	"github.com/aiyang-zh/zhenyi/zpoolobs"
	"github.com/aiyang-zh/zhenyi/zscript"
)

// vmWrapper wraps Lua VM.
// vmWrapper 包装 Lua VM。
type vmWrapper struct {
	L        *lua.LState
	gen      int64        // 所属代数
	useCount atomic.Int32 // 原子计数
	bornTime time.Time
	// ✅ 不再需要 ctxTable，改用 UserData + Lazy Loading
}

// Engine is Lua script-engine implementation.
// Engine Lua 脚本引擎实现。
type Engine struct {
	config *zscript.EngineConfig
	logger *zlog.Logger
	stats  *zscript.StatsCollector

	programs atomic.Value // map[string]*lua.FunctionProto

	globalGen atomic.Int64 // 全局代系
	closed    atomic.Bool  // 关闭标记
	vmPool    *zpool.Pool[*vmWrapper]

	// Runtime metrics.
	// 统计指标。
	vmCreated   atomic.Int64
	vmDestroyed atomic.Int64

	// Cached config values to avoid hot-path config reads.
	// 配置缓存（避免热路径读取 config）。
	maxVMUseCount int
	maxVMAge      time.Duration
	argsPool      *zpool.Pool[[]lua.LValue]
	writeMu       sync.Mutex
}

// NewEngine creates Lua engine instance.
// NewEngine 创建 Lua 引擎。
func NewEngine(config *zscript.EngineConfig) (*Engine, error) {
	if config == nil {
		config = zscript.DefaultEngineConfig()
		config.Type = zscript.EngineTypeLua
	}

	engine := &Engine{
		config:        config,
		logger:        zlog.GetDefaultLog(),
		stats:         zscript.NewStatsCollector("lua"),
		maxVMUseCount: config.MaxVMUseCount,
		maxVMAge:      config.MaxVMAge,
		argsPool: zpoolobs.NewObservedPool(zpoolobs.PoolNameZLuaArgs, func() []lua.LValue {
			return make([]lua.LValue, 0, 8)
		}),
	}

	if engine.maxVMUseCount <= 0 {
		engine.maxVMUseCount = 500
	}
	if engine.maxVMAge == 0 {
		engine.maxVMAge = 30 * time.Minute
	}

	engine.programs.Store(make(map[string]*lua.FunctionProto))
	engine.globalGen.Store(1)

	// 初始化 VM 池
	engine.vmPool = zpoolobs.NewObservedPool(zpoolobs.PoolNameZLuaVMWrapper, func() *vmWrapper {
		return engine.safeCreateVM()
	})

	engine.logger.Info("Lua engine created",
		zap.String("scriptDir", config.ScriptDir),
		zap.Duration("timeout", config.Timeout),
		zap.Int("maxVMUseCount", engine.maxVMUseCount),
		zap.Duration("maxVMAge", engine.maxVMAge))

	return engine, nil
}

// safeCreateVM safely creates VM with unified error flow (no panic).
// safeCreateVM 安全创建 VM，统一走错误流转（不再 panic）。
func (e *Engine) safeCreateVM() *vmWrapper {
	if e.closed.Load() {
		// 引擎已关闭：视为调用方误用，由上层通过 Call 等接口拿到错误
		e.logger.Error("cannot create VM: engine is closed")
		return nil
	}

	const maxRetries = 3
	for i := 0; i < maxRetries; i++ {
		vm := e.createVM()
		if vm != nil {
			return vm
		}
		e.logger.Warn("VM creation returned nil (resource error), retrying", zap.Int("attempt", i+1))
		time.Sleep(10 * time.Millisecond)
	}

	// 多次尝试仍失败：记录错误，交给上层通过 vmPool.Get() → zerrs 错误处理
	e.logger.Error("failed to create Lua VM after multiple attempts")
	return nil
}

// createVM contains core VM creation flow.
// createVM 核心创建逻辑。
func (e *Engine) createVM() *vmWrapper {
	currentGen := e.globalGen.Load()

	vm := lua.NewState(lua.Options{
		CallStackSize: 256,
		RegistrySize:  1024 * 64,
		SkipOpenLibs:  true, // 手动加载库
	})

	if vm == nil {
		return nil
	}

	e.setupSafeEnv(vm)

	// ✅ 注册 Context 类型 (每个新 VM 只需要做一次)
	e.registerContextType(vm)

	protos := e.getProgramsMap()
	for name, proto := range protos {
		lFunc := vm.NewFunctionFromProto(proto)
		vm.Push(lFunc)
		if err := vm.PCall(0, 0, nil); err != nil {
			e.logger.Error("Fatal: preloaded script execution failed",
				zap.String("script", name),
				zap.Error(err))
			vm.Close()
			// 初始化失败：丢弃该 VM，返回 nil，让上层走错误流转
			return nil
		}
	}

	e.vmCreated.Add(1)

	return &vmWrapper{
		L:        vm,
		gen:      currentGen,
		bornTime: time.Now(),
	}
}

// setupSafeEnv configures sandbox environment.
// setupSafeEnv 设置沙箱环境。
// It loads only safe standard libraries and excludes dangerous os/io.
// 只加载安全的标准库，不加载危险的 os、io。
func (e *Engine) setupSafeEnv(vm *lua.LState) {
	// 只打开安全的库
	lua.OpenBase(vm)   // 基础库（print、type、tonumber 等）
	lua.OpenTable(vm)  // table
	lua.OpenString(vm) // string
	lua.OpenMath(vm)   // math

	// 明确禁用危险函数（即使没有加载，也显式设置 nil）
	vm.SetGlobal("loadfile", lua.LNil)
	vm.SetGlobal("dofile", lua.LNil)
	vm.SetGlobal("require", lua.LNil)
	vm.SetGlobal("load", lua.LNil)
	vm.SetGlobal("loadstring", lua.LNil)

	// 日志桥接（重载 print 函数回调）
	vm.SetGlobal("print", vm.NewFunction(func(L *lua.LState) int {
		top := L.GetTop()
		args := make([]interface{}, 0, top)
		for i := 1; i <= top; i++ {
			args = append(args, L.Get(i).String())
		}
		e.logger.Info("LUA", zap.Any("msg", args))
		return 0
	}))
}

// registerContextType registers Context type (UserData + lazy loading).
// registerContextType 注册 Context 类型（UserData + Lazy Loading）。
func (e *Engine) registerContextType(L *lua.LState) {
	mt := L.NewTypeMetatable("ScriptContext")
	L.SetGlobal("ScriptContext", mt)

	// ✅ 核心魔法：__index 元方法（按需取值，Zero-Copy）
	L.SetField(mt, "__index", L.NewFunction(func(L *lua.LState) int {
		// 1. 取出 UserData
		ud := L.CheckUserData(1)
		// 2. 取出要访问的字段名
		key := L.CheckString(2)
		// 3. 拿到 Go 原始对象
		ctx, ok := ud.Value.(*zscript.ScriptContext)
		if !ok {
			L.Push(lua.LNil)
			return 1
		}

		// 4. 按需返回 (Lazy Load - 只转换被访问的字段)
		switch key {
		case "ActorID":
			L.Push(lua.LNumber(ctx.ActorID))
		case "ActorType":
			L.Push(lua.LNumber(ctx.ActorType))
		case "MsgID":
			L.Push(lua.LNumber(ctx.MsgID))
		case "AuthID":
			L.Push(lua.LString(strconv.FormatInt(ctx.AuthID, 10)))
		case "TraceID":
			L.Push(lua.LString(strconv.FormatUint(ctx.TraceID, 10)))
		case "NowMillis":
			L.Push(lua.LNumber(ctx.NowMillis))
		case "Owner":
			// ⚡️ 只有脚本真的用了 ctx.Owner，这里才会执行转换！
			L.Push(e.goToLua(L, ctx.Owner))
		case "MsgData":
			// ⚡️ 只有脚本真的用了 ctx.MsgData，这里才会执行转换！
			if b, ok := ctx.MsgData.([]byte); ok {
				// Memoize：避免同一次 Call 中重复执行 string(b) 造成多次分配。
				s := string(b)
				ctx.MsgData = s
				L.Push(lua.LString(s))
			} else {
				L.Push(e.goToLua(L, ctx.MsgData))
			}
		case "Metadata":
			// ⚡️ 只有脚本真的用了 ctx.Metadata，这里才会执行转换！
			L.Push(e.goToLua(L, ctx.Metadata))
		default:
			L.Push(lua.LNil)
		}
		return 1
	}))
}

// LoadScript loads/compiles one script file into engine cache.
// LoadScript 加载/编译一个脚本文件到引擎缓存。
func (e *Engine) LoadScript(path string) error {
	return e.loadScriptsInternal([]string{path}, false)
}

// LoadScripts loads/compiles multiple script files into engine cache.
// LoadScripts 批量加载/编译脚本文件到引擎缓存。
func (e *Engine) LoadScripts(paths []string) error {
	return e.loadScriptsInternal(paths, false)
}

// ReloadScript reloads one script file and bumps generation to retire old VMs.
// ReloadScript 重载单个脚本文件，并递增代数以淘汰旧 VM。
func (e *Engine) ReloadScript(path string) error {
	e.stats.RecordReload()
	return e.loadScriptsInternal([]string{path}, true)
}

// ReloadAllScripts reloads all loaded scripts.
// ReloadAllScripts 重载所有已加载脚本。
func (e *Engine) ReloadAllScripts() error {
	e.stats.RecordReload()
	programsMap := e.getProgramsMap()
	paths := make([]string, 0, len(programsMap))
	for path := range programsMap {
		paths = append(paths, path)
	}
	return e.loadScriptsInternal(paths, true)
}

func (e *Engine) loadScriptsInternal(paths []string, isReload bool) error {
	if len(paths) == 0 {
		return nil
	}

	// 1. 预编译验证
	compilerVM := lua.NewState()
	defer compilerVM.Close()

	newProtos := make(map[string]*lua.FunctionProto, len(paths))
	for _, path := range paths {
		code, err := os.ReadFile(path) // #nosec G304 -- path is derived from loaded script keys
		if err != nil {
			return zerrs.Wrapf(err, zscript.ErrTypeScript, "read failed: %s", path)
		}
		fn, err := compilerVM.Load(bytes.NewReader(code), filepath.Base(path))
		if err != nil {
			return zerrs.Wrapf(err, zscript.ErrTypeScript, "compile failed: %s", path)
		}
		newProtos[path] = fn.Proto
	}

	// 2. 原子更新
	e.writeMu.Lock()
	defer e.writeMu.Unlock()

	oldMap := e.getProgramsMap()
	finalMap := make(map[string]*lua.FunctionProto, len(oldMap)+len(newProtos))
	for k, v := range oldMap {
		finalMap[k] = v
	}
	for k, v := range newProtos {
		finalMap[k] = v
	}
	e.programs.Store(finalMap)
	newGen := e.globalGen.Add(1)

	if isReload {
		e.logger.Info("Lua Engine upgraded generation", zap.Int64("gen", newGen))
	}

	e.logger.Info("Lua scripts loaded", zap.Int("count", len(paths)), zap.Bool("reload", isReload))
	return nil
}

// Call executes a script function.
// Call 执行脚本函数。
func (e *Engine) Call(callCtx context.Context, params *zscript.CallParams, function string, args ...interface{}) (result interface{}, retErr error) {
	if e.closed.Load() {
		return nil, zerrs.New(zscript.ErrTypeScript, "engine closed")
	}
	if callCtx == nil {
		return nil, zerrs.New(zerrs.ErrTypeValidation, "zlua.Engine.Call: ctx is required")
	}
	if params == nil {
		return nil, zerrs.New(zerrs.ErrTypeValidation, "zlua.Engine.Call: params is nil")
	}

	start := time.Now()
	var err error

	ctx := zscript.GetContext(params.ActorID, params.ActorType)
	defer func() {
		e.stats.RecordCall(time.Since(start), err)
		zscript.PutContext(ctx)
		if retErr == nil && err != nil {
			retErr = err
		}
	}()

	ctx.WithOwner(params.Owner).
		WithTraceID(params.TraceID).
		WithMessage(params.MsgID, params.AuthID, params.MsgData)

	if params.Metadata != nil {
		for k, v := range params.Metadata {
			ctx.WithMetadata(k, v)
		}
	}

	// 1. 获取 VM
	wrapper := e.vmPool.Get()
	if wrapper == nil || wrapper.L == nil {
		return nil, zerrs.New(zscript.ErrTypeScript, "failed to obtain valid VM")
	}

	vm := wrapper.L
	currentUses := wrapper.useCount.Add(1)

	shouldDestroy := false
	defer func() {
		if r := recover(); r != nil {
			shouldDestroy = true
			e.logger.Error("Panic in Lua call", zap.Any("panic", r))
			err = zerrs.Newf(zscript.ErrTypeScript, "panic: %v", r)
		}

		// 检查代数
		if wrapper.gen != e.globalGen.Load() {
			shouldDestroy = true
		}

		// 检查使用次数
		if currentUses > int32(e.maxVMUseCount) {
			shouldDestroy = true
		}

		// 检查存活时间
		if time.Since(wrapper.bornTime) > e.maxVMAge {
			shouldDestroy = true
		}

		if shouldDestroy {
			vm.Close()
			e.vmDestroyed.Add(1)
		} else {
			// 清理环境：重置 Lua 栈与上下文；保持与调用方相同的 Value 链，但移除已过期的 cancel/deadline。
			vm.SetTop(0)
			vm.SetGlobal("ctx", lua.LNil)
			// 释放对上一次 callCtx 的引用，避免复用时取消语义被掩盖。
			// 下次 Call 开始时会重新 SetContext(callCtx)。
			vm.SetContext(context.TODO())

			// ✅ Lua VM 的 GC 由 Go 的 GC 管理，无需手动触发
			// gopher-lua 的内存由 Go 管理，当 VM 被释放时自动回收

			e.vmPool.Put(wrapper)
		}
	}()

	// 2. 每次 Call 都显式设置上下文，确保在 config.Timeout==0 时取消语义仍然生效。
	vm.SetContext(callCtx)

	// 3. 注入上下文（复用 VM 中的 Context table）
	e.injectContext(wrapper, ctx)

	// 4. 超时控制（条件启用，减少 Context 开销）
	//
	// ⚡️ 优化策略：只有显式设置超时时才启用 Context
	// - Timeout = 0:  不启用超时（极速模式，零开销）
	// - Timeout < 0:  使用默认 5s 超时
	// - Timeout > 0:  使用指定超时
	//
	// Context 开销：约 +7 allocs（相比 Timeout=0）
	timeout := e.config.Timeout
	if timeout != 0 {
		if timeout < 0 {
			timeout = 5 * time.Second
		}
		// 基于调用方传入的 ctx 派生超时控制，继承 trace/value。
		runCtx, cancel := context.WithTimeout(callCtx, timeout)
		defer cancel()
		vm.SetContext(runCtx)
	}

	// 5. 参数准备（使用框架池化技术）
	luaArgs := e.argsPool.Get()[:0]
	for _, arg := range args {
		luaArgs = append(luaArgs, e.goToLua(vm, arg))
	}
	defer func() {
		e.argsPool.Put(luaArgs[:0])
	}()

	// 6. 执行
	if err = vm.CallByParam(lua.P{
		Fn:      vm.GetGlobal(function),
		NRet:    1,
		Protect: true,
	}, luaArgs...); err != nil {
		shouldDestroy = true // 运行时错误后销毁 VM
		// 检查是否是超时错误
		if err.Error() == "context deadline exceeded" {
			e.stats.RecordTimeout()
			return nil, zscript.ErrScriptTimeout
		}
		return nil, zerrs.Wrap(err, zscript.ErrTypeScript, "lua execution failed")
	}

	// 6. 结果处理
	ret := vm.Get(-1)
	result = e.luaToGo(ret)
	vm.Pop(1)

	return result, nil
}

// Close closes engine.
// Close 关闭引擎。
func (e *Engine) Close() {
	if !e.closed.CompareAndSwap(false, true) {
		return
	}
	e.globalGen.Add(1)
	e.programs.Store(make(map[string]*lua.FunctionProto))
	e.logger.Info("Lua engine closed")
}

// GetStats returns engine runtime stats snapshot.
// GetStats 返回引擎运行时统计快照。
func (e *Engine) GetStats() *zscript.EngineStats {
	stats := e.stats.GetStats()
	if stats.Metadata == nil {
		stats.Metadata = make(map[string]interface{})
	}
	stats.Metadata["vm_created"] = e.vmCreated.Load()
	stats.Metadata["vm_destroyed"] = e.vmDestroyed.Load()
	stats.Metadata["vm_active"] = e.vmCreated.Load() - e.vmDestroyed.Load()

	programsMap := e.getProgramsMap()
	stats.ScriptFiles = make([]zscript.ScriptFileInfo, 0, len(programsMap))
	for path := range programsMap {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		stats.ScriptFiles = append(stats.ScriptFiles, zscript.ScriptFileInfo{
			Path:     path,
			Size:     info.Size(),
			LoadTime: info.ModTime(),
		})
	}
	return stats
}

// GetType returns engine type string.
// GetType 返回引擎类型字符串。
func (e *Engine) GetType() string { return string(zscript.EngineTypeLua) }

// ----------------- 辅助函数 -----------------

func (e *Engine) getProgramsMap() map[string]*lua.FunctionProto {
	v := e.programs.Load()
	if v == nil {
		return make(map[string]*lua.FunctionProto)
	}
	m, ok := v.(map[string]*lua.FunctionProto)
	if !ok {
		e.logger.Error("Critical: programs type assertion failed")
		return make(map[string]*lua.FunctionProto)
	}
	return m
}

func (e *Engine) injectContext(wrapper *vmWrapper, ctx *zscript.ScriptContext) {
	vm := wrapper.L

	// ✅ 1. 创建 UserData (极轻量，仅包装指针)
	ud := vm.NewUserData()
	ud.Value = ctx

	// ✅ 2. 挂载元表 (绑定 __index 行为)
	vm.SetMetatable(ud, vm.GetTypeMetatable("ScriptContext"))

	// ✅ 3. 设置全局变量 (Zero-Copy，按需访问)
	vm.SetGlobal("ctx", ud)
}

func (e *Engine) goToLua(vm *lua.LState, val interface{}) lua.LValue {
	if val == nil {
		return lua.LNil
	}

	switch v := val.(type) {
	case bool:
		return lua.LBool(v)
	case string:
		return lua.LString(v)
	case []byte:
		return lua.LString(string(v))
	case float64:
		return lua.LNumber(v)
	case float32:
		return lua.LNumber(v)
	case int:
		return lua.LNumber(v)
	case int8:
		return lua.LNumber(v)
	case int16:
		return lua.LNumber(v)
	case int32:
		return lua.LNumber(v)
	case int64:
		if v > 9007199254740991 || v < -9007199254740991 {
			return lua.LString(strconv.FormatInt(v, 10))
		}
		return lua.LNumber(v)
	case uint:
		return lua.LNumber(v)
	case uint8:
		return lua.LNumber(v)
	case uint16:
		return lua.LNumber(v)
	case uint32:
		return lua.LNumber(v)
	case uint64:
		if v > 9007199254740991 {
			return lua.LString(strconv.FormatUint(v, 10))
		}
		return lua.LNumber(v)
	case map[string]interface{}:
		table := vm.NewTable()
		for k, val := range v {
			table.RawSetString(k, e.goToLua(vm, val))
		}
		return table
	case []interface{}:
		table := vm.NewTable()
		for i, val := range v {
			table.RawSetInt(i+1, e.goToLua(vm, val))
		}
		return table
	default:
		return lua.LNil
	}
}

func (e *Engine) luaToGo(val lua.LValue) interface{} {
	switch v := val.(type) {
	case *lua.LNilType:
		return nil
	case lua.LBool:
		return bool(v)
	case lua.LNumber:
		return float64(v)
	case lua.LString:
		return string(v)
	case *lua.LTable:
		if e.isArray(v) {
			return e.luaTableToSlice(v)
		}
		return e.luaTableToMap(v)
	default:
		return nil
	}
}

func (e *Engine) isArray(table *lua.LTable) bool {
	n := table.MaxN()
	if n == 0 {
		return false
	}
	for i := 1; i <= n; i++ {
		if table.RawGetInt(i) == lua.LNil {
			return false
		}
	}
	var hasNonNumericKey bool
	table.ForEach(func(key, _ lua.LValue) {
		if _, ok := key.(lua.LNumber); !ok {
			hasNonNumericKey = true
		}
	})
	return !hasNonNumericKey
}

func (e *Engine) luaTableToSlice(table *lua.LTable) []interface{} {
	count := table.MaxN()
	result := make([]interface{}, 0, count)
	for i := 1; i <= count; i++ {
		result = append(result, e.luaToGo(table.RawGetInt(i)))
	}
	return result
}

func (e *Engine) luaTableToMap(table *lua.LTable) map[string]interface{} {
	result := make(map[string]interface{})
	table.ForEach(func(key, val lua.LValue) {
		if keyStr, ok := key.(lua.LString); ok {
			result[string(keyStr)] = e.luaToGo(val)
		}
	})
	return result
}
