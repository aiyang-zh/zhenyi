package zjs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aiyang-zh/zhenyi-base/zerrs"
	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi-base/zpool"
	"github.com/aiyang-zh/zhenyi/zpoolobs"
	"github.com/aiyang-zh/zhenyi/zscript"
	"github.com/dop251/goja"
	"go.uber.org/zap"
)

// vmWrapper wraps goja.Runtime for lifecycle management.
// vmWrapper 包装 goja.Runtime，用于生命周期管理。
type vmWrapper struct {
	vm       *goja.Runtime
	gen      int64        // 所属代数
	useCount atomic.Int32 // 使用计数
	bornTime time.Time    // 创建时间

	// ✅ 核心优化：每个 VM 独占一个 Context 容器
	// 这个指针永远不变，JS 全局变量 'ctx' 永远指向这个地址
	// 避免了每次 Call 时 vm.Set 的巨额分配（约 20-30 allocs）
	sharedCtx *zscript.ScriptContext

	// timeout 复用：每个 VM 复用一个 Timer，避免每次 Call 创建新的 Timer 对象。
	timeoutTimer           *time.Timer
	timeoutFired           atomic.Bool
	timeoutCallbackRunning atomic.Bool
}

// Engine is JavaScript script-engine implementation (zhenyi/zscript contract).
// Engine JavaScript 脚本引擎实现（使用 zhenyi/zscript 契约）。
type Engine struct {
	config *zscript.EngineConfig
	logger *zlog.Logger
	stats  *zscript.StatsCollector

	programs atomic.Value

	modulePrograms   atomic.Value
	moduleProgramsMu sync.Mutex

	globalGen       atomic.Int64
	closed          atomic.Bool
	vmPool          *zpool.Pool[*vmWrapper]
	argsPool        *zpool.Pool[[]goja.Value]
	consoleArgsPool *zpool.Pool[[]interface{}]
	// scriptDirAbs is the canonical absolute ScriptDir used for require path checks.
	// If empty, require() is rejected unless allowRequireWithoutScriptDir is true (legacy).
	scriptDirAbs string
	// allowRequireWithoutScriptDir enables legacy require resolution when scriptDirAbs is empty.
	allowRequireWithoutScriptDir bool

	// Runtime metrics.
	// 监控指标。
	vmCreated   atomic.Int64
	vmDestroyed atomic.Int64

	// Cached config values.
	// 配置缓存。
	maxVMUseCount int
	maxVMAge      time.Duration

	writeMu sync.Mutex // Load/Reload 互斥锁
}

// NewEngine creates a JavaScript engine.
// NewEngine 创建 JavaScript 脚本引擎。
func NewEngine(config *zscript.EngineConfig) (*Engine, error) {
	if config == nil {
		config = zscript.DefaultEngineConfig()
		config.Type = zscript.EngineTypeJS
	}

	engine := &Engine{
		config:                       config,
		logger:                       zlog.GetDefaultLog(),
		stats:                        zscript.NewStatsCollector("javascript"),
		maxVMUseCount:                config.MaxVMUseCount,
		maxVMAge:                     config.MaxVMAge,
		allowRequireWithoutScriptDir: config.AllowRequireWithoutScriptDir,
		argsPool: zpoolobs.NewObservedPool(zpoolobs.PoolNameZJsArgs, func() []goja.Value {
			return make([]goja.Value, 0, 8)
		}),
		consoleArgsPool: zpoolobs.NewObservedPool(zpoolobs.PoolNameZJsConsoleArgs, func() []interface{} {
			return make([]interface{}, 0, 8)
		}),
	}

	if engine.maxVMUseCount <= 0 {
		engine.maxVMUseCount = 500
	}
	if engine.maxVMAge == 0 {
		engine.maxVMAge = 30 * time.Minute
	}

	engine.programs.Store(make(map[string]*goja.Program))
	engine.modulePrograms.Store(make(map[string]*goja.Program))
	engine.globalGen.Store(1)

	// Pre-compute ScriptDir absolute path for require sandboxing.
	// If it fails, we keep sandbox disabled (scriptDirAbs stays empty).
	if config.ScriptDir != "" {
		if abs, err := filepath.Abs(config.ScriptDir); err == nil {
			engine.scriptDirAbs = filepath.Clean(abs)
		}
	}

	engine.vmPool = zpoolobs.NewObservedPool(zpoolobs.PoolNameZJsVMWrapper, func() *vmWrapper {
		return engine.safeCreateVM()
	})

	engine.logger.Info("JavaScript engine created",
		zap.String("scriptDir", config.ScriptDir),
		zap.Duration("timeout", config.Timeout),
		zap.Int("maxVMUseCount", engine.maxVMUseCount),
		zap.Duration("maxVMAge", engine.maxVMAge))

	return engine, nil
}

func (e *Engine) isUnderScriptDir(absPath string) bool {
	if e.scriptDirAbs == "" {
		return false
	}
	rel, err := filepath.Rel(e.scriptDirAbs, absPath)
	if err != nil {
		return false
	}
	rel = filepath.Clean(rel)
	if rel == "." {
		return true
	}
	// Outside when rel starts with "../" or is exactly "..".
	prefix := ".." + string(os.PathSeparator)
	return rel != ".." && !strings.HasPrefix(rel, prefix)
}

func (e *Engine) resolveRequireAbsPath(rawPath string) (string, error) {
	if e.scriptDirAbs == "" {
		if !e.allowRequireWithoutScriptDir {
			return "", fmt.Errorf("require: ScriptDir is empty; set EngineConfig.ScriptDir, or set AllowRequireWithoutScriptDir for legacy unsandboxed require")
		}
		if abs, err := filepath.Abs(rawPath); err == nil {
			return abs, nil
		}
		return rawPath, nil
	}

	var abs string
	if filepath.IsAbs(rawPath) {
		abs = rawPath
	} else {
		abs = filepath.Join(e.scriptDirAbs, rawPath)
	}
	abs = filepath.Clean(abs)
	if abs2, err := filepath.Abs(abs); err == nil {
		abs = abs2
	}

	if !e.isUnderScriptDir(abs) {
		return "", fmt.Errorf("require: path outside ScriptDir: %s", rawPath)
	}
	return abs, nil
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
		w := e.createVM()
		if w != nil {
			return w
		}
		e.logger.Warn("VM creation returned nil, retrying", zap.Int("attempt", i+1))
		time.Sleep(10 * time.Millisecond)
	}

	// 多次尝试仍失败：记录错误，交给上层通过 vmPool.Get() → zerrs 错误处理
	e.logger.Error("failed to create JS VM after retries")
	return nil
}

// createVM contains core VM creation flow.
// createVM 核心创建逻辑。
func (e *Engine) createVM() *vmWrapper {
	currentGen := e.globalGen.Load()

	vm := goja.New()

	err := e.registerConsole(vm)
	if err != nil {
		return nil
	}
	err = e.registerRequire(vm)
	if err != nil {
		return nil
	}

	// 预加载所有主脚本 (Pre-warm)
	programs := e.getProgramsMap()
	for name, prog := range programs {
		_, err = vm.RunProgram(prog)
		if err != nil {
			e.logger.Error("Fatal: preload script failed in new JS VM",
				zap.String("script", name),
				zap.Error(err))
			// 初始化失败：丢弃该 VM，返回 nil，让上层走错误流转
			return nil
		}
	}

	e.vmCreated.Add(1)

	// ✅ 预创建一个永久的 Context 对象
	sharedCtx := &zscript.ScriptContext{}

	// ✅ 一次性注入！
	// JS 中的 'ctx' 变量将永久绑定到这个 sharedCtx 的内存地址
	// Goja 会为它建立反射缓存，后续没有任何分配
	err = vm.Set("ctx", sharedCtx)
	if err != nil {
		return nil
	}

	// 创建 wrapper 后初始化 timeout timer（只初始化一次）
	wrapper := &vmWrapper{
		vm:        vm,
		gen:       currentGen,
		bornTime:  time.Now(),
		sharedCtx: sharedCtx,
	}

	// 用一个很长的初始时长创建 Timer，随后会在每次 Call 里 Stop/Reset。
	// 回调会设置 timeoutFired 并中断 VM；退出前会等待回调执行完毕，避免跨调用误中断。
	wrapper.timeoutTimer = time.AfterFunc(365*24*time.Hour, func() {
		wrapper.timeoutCallbackRunning.Store(true)
		wrapper.timeoutFired.Store(true)
		wrapper.vm.Interrupt("timeout")
		wrapper.timeoutCallbackRunning.Store(false)
	})
	// 初始 timer 不应触发
	wrapper.timeoutTimer.Stop()

	return wrapper
}

// Call executes a script function.
// Call 执行脚本函数。
func (e *Engine) Call(callCtx context.Context, params *zscript.CallParams, function string, args ...interface{}) (result interface{}, retErr error) {
	if e.closed.Load() {
		return nil, zerrs.New(zscript.ErrTypeScript, "engine closed")
	}
	if callCtx == nil {
		return nil, zerrs.New(zerrs.ErrTypeValidation, "zjs.Engine.Call: ctx is required")
	}
	if params == nil {
		return nil, zerrs.New(zerrs.ErrTypeValidation, "zjs.Engine.Call: params is nil")
	}

	start := time.Now()
	var err error

	defer func() {
		e.stats.RecordCall(time.Since(start), err)
		if retErr == nil && err != nil {
			retErr = err
		}
	}()

	// 1. 获取 VM
	wrapper := e.vmPool.Get()
	if wrapper == nil || wrapper.vm == nil {
		e.logger.Error("Failed to get valid VM from pool")
		return nil, zerrs.New(zscript.ErrTypeScript, "no valid VM available")
	}

	vm := wrapper.vm
	uses := wrapper.useCount.Add(1)

	// 🛡️ 关键：清除上次可能残留的中断
	vm.ClearInterrupt()

	// ✅ 2. 数据填充 (Struct Copy)
	// 直接把数据填入 wrapper.sharedCtx
	// 因为 Goja 已经绑定了这个指针，JS 立即就能看到新数据
	sharedCtx := wrapper.sharedCtx
	sharedCtx.ActorID = params.ActorID
	sharedCtx.ActorType = params.ActorType
	sharedCtx.MsgID = params.MsgID
	sharedCtx.AuthID = params.AuthID
	sharedCtx.TraceID = params.TraceID
	sharedCtx.MsgData = params.MsgData
	sharedCtx.NowMillis = time.Now().UnixMilli()
	sharedCtx.Owner = params.Owner

	// 处理 Metadata（延迟初始化）
	if params.Metadata != nil {
		if sharedCtx.Metadata == nil {
			sharedCtx.Metadata = make(map[string]interface{})
		}
		// 清空旧数据
		for k := range sharedCtx.Metadata {
			delete(sharedCtx.Metadata, k)
		}
		// 填充新数据
		for k, v := range params.Metadata {
			sharedCtx.Metadata[k] = v
		}
	} else {
		sharedCtx.Metadata = nil
	}

	shouldDestroy := false

	// 3. 生命周期管理 (Defer)
	defer func() {
		// Panic 捕获
		if r := recover(); r != nil {
			// goja 在从 Go 回调向 JS 抛异常时，会使用 panic 搭配内部 Exception。
			// 这里需要区分“真实 Go panic”与“JS 异常”，避免把 JS 业务错误误当成宿主崩溃。
			if ex, ok := r.(*goja.Exception); ok {
				if wrapper.timeoutFired.Load() {
					err = zscript.ErrScriptTimeout
				} else {
					err = zerrs.Wrap(ex, zscript.ErrTypeScript, "js exception")
				}
			} else {
				shouldDestroy = true
				e.logger.Error("Panic in JS execution", zap.Any("panic", r))
				err = zerrs.Newf(zscript.ErrTypeScript, "panic: %v", r)
			}
		}

		// 检查代数（热更淘汰）
		if wrapper.gen != e.globalGen.Load() {
			shouldDestroy = true
		}

		// 检查使用次数
		if uses > int32(e.maxVMUseCount) {
			shouldDestroy = true
		}

		// 检查存活时间
		if time.Since(wrapper.bornTime) > e.maxVMAge {
			shouldDestroy = true
		}

		// 运行时错误（可能导致状态不一致，建议销毁）
		if err != nil {
			shouldDestroy = true
		}

		if shouldDestroy {
			// 直接丢弃，等待 GC
			e.vmDestroyed.Add(1)
		} else {
			// ✅ 清理敏感数据（但不需要 Set，因为下次会覆盖）
			sharedCtx.MsgData = nil
			sharedCtx.Metadata = nil
			e.vmPool.Put(wrapper)
		}
	}()

	// 4. 准备参数
	fnVal := vm.Get(function)
	fn, ok := goja.AssertFunction(fnVal)
	if !ok {
		err = zscript.ErrFunctionNotFound
		return nil, err
	}

	// 使用框架池化技术准备参数
	jsArgs := e.argsPool.Get()[:0]
	for _, arg := range args {
		jsArgs = append(jsArgs, vm.ToValue(arg))
	}
	defer func() {
		e.argsPool.Put(jsArgs[:0])
	}()

	// 5. 超时控制（条件启用，减少 Timer 开销）
	timeout := e.config.Timeout
	// timeout 复用：每个 VM 只初始化一次 Timer。
	// 正常情况下上一轮 Call 会在结束前停止 timer；这里仅在回调正在运行时 Stop/等待以避免竞态。
	wrapper.timeoutFired.Store(false)
	if timeout != 0 {
		if timeout < 0 {
			timeout = 5 * time.Second
		}
		if wrapper.timeoutTimer != nil {
			// 绝大多数情况下回调不在运行：跳过 Stop/等待，降低热路径开销。
			if wrapper.timeoutCallbackRunning.Load() {
				_ = wrapper.timeoutTimer.Stop()
				for wrapper.timeoutCallbackRunning.Load() {
					runtime.Gosched()
				}
			}
			wrapper.timeoutTimer.Reset(timeout)
		}
	}

	// 6. 同步执行
	resVal, callErr := fn(goja.Undefined(), jsArgs...)

	// 停止定时器（防止触发超时回调残留）
	if wrapper.timeoutTimer != nil {
		if !wrapper.timeoutTimer.Stop() {
			for wrapper.timeoutCallbackRunning.Load() {
				runtime.Gosched()
			}
		}
	}

	// 检查是否发生了超时
	if wrapper.timeoutFired.Load() {
		e.stats.RecordTimeout()
		shouldDestroy = true
		return nil, zscript.ErrScriptTimeout
	}

	// 检查其他错误
	if callErr != nil {
		err = zerrs.Wrap(callErr, zscript.ErrTypeScript, "js execution failed")
		return nil, err
	}

	return resVal.Export(), nil
}

// registerRequire is enhanced require implementation with absolute-path dedup.
// registerRequire 增强版：支持绝对路径去重。
func (e *Engine) registerRequire(vm *goja.Runtime) error {
	if vm.Get("_moduleCache") == nil {
		err := vm.Set("_moduleCache", vm.NewObject())
		if err != nil {
			return err
		}
	}

	err := vm.Set("require", func(call goja.FunctionCall) goja.Value {
		if len(call.Arguments) == 0 {
			panic(vm.NewTypeError("require: missing path"))
		}
		rawPath := call.Arguments[0].String()

		// 1. Resolve module path under ScriptDir sandbox.
		modulePath, err := e.resolveRequireAbsPath(rawPath)
		if err != nil {
			panic(vm.NewGoError(err))
		}

		// 2. 检查 VM 内实例缓存
		cache := vm.Get("_moduleCache").(*goja.Object)
		if exports := cache.Get(modulePath); exports != nil && exports != goja.Undefined() {
			return exports
		}

		// 3. 检查全局 Program 缓存
		progCache := e.getModulePrograms()
		prog, ok := progCache[modulePath]

		if !ok {
			// 首次加载
			code, err := os.ReadFile(modulePath) // #nosec G304 -- modulePath is resolved under ScriptDir sandbox
			if err != nil {
				panic(vm.NewGoError(fmt.Errorf("require: failed to read %s: %v", modulePath, err)))
			}

			prog, err = goja.Compile(filepath.Base(modulePath), string(code), true)
			if err != nil {
				panic(vm.NewGoError(fmt.Errorf("require: compile failed %s: %v", modulePath, err)))
			}

			// 更新全局缓存
			e.moduleProgramsMu.Lock()
			oldCache := e.getModulePrograms()
			newCache := make(map[string]*goja.Program, len(oldCache)+1)
			for k, v := range oldCache {
				newCache[k] = v
			}
			newCache[modulePath] = prog
			e.modulePrograms.Store(newCache)
			e.moduleProgramsMu.Unlock()
		}

		// 4. 模拟 CommonJS 加载
		moduleObj := vm.NewObject()
		exportsObj := vm.NewObject()
		err = moduleObj.Set("exports", exportsObj)
		if err != nil {
			panic(vm.NewGoError(err))
		}

		// 预先放入缓存防止循环引用
		err = cache.Set(modulePath, exportsObj)
		if err != nil {
			panic(vm.NewGoError(err))
		}

		oldModule := vm.Get("module")
		oldExports := vm.Get("exports")

		err = vm.Set("module", moduleObj)
		if err != nil {
			panic(vm.NewGoError(err))
		}
		err = vm.Set("exports", exportsObj)
		if err != nil {
			panic(vm.NewGoError(err))
		}

		_, err = vm.RunProgram(prog)
		if err != nil {
			panic(vm.NewGoError(err))
		}

		err = vm.Set("module", oldModule)
		if err != nil {
			panic(vm.NewGoError(err))
		}
		err = vm.Set("exports", oldExports)
		if err != nil {
			panic(vm.NewGoError(err))
		}

		if err != nil {
			err = cache.Set(modulePath, goja.Undefined())
			if err != nil {
				panic(vm.NewGoError(err))
			}
			panic(vm.NewGoError(fmt.Errorf("require: execution failed %s: %v", modulePath, err)))
		}

		finalExports := moduleObj.Get("exports")
		err = cache.Set(modulePath, finalExports)
		if err != nil {
			panic(vm.NewGoError(err))
		}

		return finalExports
	})
	if err != nil {
		return err
	}
	return nil
}

// ----------------- 以下为标准实现 -----------------

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
	// 尝试清理对应的模块缓存
	if absPath, err := filepath.Abs(path); err == nil {
		e.clearModuleProgram(absPath)
	}
	return e.loadScriptsInternal([]string{path}, true)
}

// ReloadAllScripts reloads all loaded scripts and clears module program cache.
// ReloadAllScripts 重载所有已加载脚本，并清空模块 Program 缓存。
func (e *Engine) ReloadAllScripts() error {
	e.stats.RecordReload()
	e.modulePrograms.Store(make(map[string]*goja.Program))

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
	start := time.Now()

	newProgs := make(map[string]*goja.Program, len(paths))
	for _, path := range paths {
		code, err := os.ReadFile(path) // #nosec G304 -- path comes from engine-controlled script map
		if err != nil {
			return zerrs.Wrapf(err, zscript.ErrTypeScript, "read failed: %s", path)
		}
		prog, err := goja.Compile(filepath.Base(path), string(code), true)
		if err != nil {
			return zerrs.Wrapf(err, zscript.ErrTypeScript, "compile failed: %s", path)
		}
		newProgs[path] = prog
	}

	e.writeMu.Lock()
	oldMap := e.getProgramsMap()
	finalMap := make(map[string]*goja.Program, len(oldMap)+len(newProgs))
	for k, v := range oldMap {
		finalMap[k] = v
	}
	for k, v := range newProgs {
		finalMap[k] = v
	}
	e.programs.Store(finalMap)

	if isReload {
		newGen := e.globalGen.Add(1)
		e.logger.Info("JS generation upgraded", zap.Int64("gen", newGen))
	}
	e.writeMu.Unlock()

	e.stats.RecordCall(time.Since(start), nil)
	e.logger.Info("JavaScript scripts loaded", zap.Int("count", len(paths)), zap.Bool("reload", isReload))
	return nil
}

func (e *Engine) registerConsole(vm *goja.Runtime) error {
	console := vm.NewObject()
	err := console.Set("log", func(call goja.FunctionCall) goja.Value {
		args := e.consoleArgsPool.Get()[:0]
		for _, arg := range call.Arguments {
			args = append(args, arg.Export())
		}
		e.logger.Info("JS_CONSOLE", zap.Any("args", args))
		// 清空元素引用，避免旧的 interface{} 引用残留在底层数组导致内存被额外保活。
		for i := range args {
			args[i] = nil
		}
		e.consoleArgsPool.Put(args[:0])
		return goja.Undefined()
	})
	if err != nil {
		return err
	}
	return vm.Set("console", console)
}

func (e *Engine) getProgramsMap() map[string]*goja.Program {
	v := e.programs.Load()
	if v == nil {
		return make(map[string]*goja.Program)
	}
	m, ok := v.(map[string]*goja.Program)
	if !ok {
		e.logger.Error("programs type assertion failed")
		return make(map[string]*goja.Program)
	}
	return m
}

func (e *Engine) getModulePrograms() map[string]*goja.Program {
	v := e.modulePrograms.Load()
	if v == nil {
		return make(map[string]*goja.Program)
	}
	m, ok := v.(map[string]*goja.Program)
	if !ok {
		e.logger.Error("modulePrograms type assertion failed")
		return make(map[string]*goja.Program)
	}
	return m
}

func (e *Engine) clearModuleProgram(path string) {
	e.moduleProgramsMu.Lock()
	defer e.moduleProgramsMu.Unlock()

	old := e.getModulePrograms()
	newCache := make(map[string]*goja.Program, len(old))
	for k, v := range old {
		if k != path {
			newCache[k] = v
		}
	}
	e.modulePrograms.Store(newCache)
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

// Close closes the engine and prevents further calls.
// Close 关闭引擎并禁止后续调用。
func (e *Engine) Close() {
	if !e.closed.CompareAndSwap(false, true) {
		return
	}
	e.globalGen.Add(1)
	e.programs.Store(make(map[string]*goja.Program))
	e.modulePrograms.Store(make(map[string]*goja.Program))
	e.logger.Info("JavaScript engine closed")
}

// GetType returns engine type string.
// GetType 返回引擎类型字符串。
func (e *Engine) GetType() string {
	return string(zscript.EngineTypeJS)
}
