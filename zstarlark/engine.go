package zstarlark

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aiyang-zh/zhenyi-base/zerrs"
	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi-base/zpool"
	"github.com/aiyang-zh/zhenyi/zpoolobs"
	"github.com/aiyang-zh/zhenyi/zscript"
	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
	"go.uber.org/zap"
)

// defaultStarlarkFileOptions controls file-level parsing/resolution behavior.
// defaultStarlarkFileOptions 控制文件级的解析/语义开关。
// It replaces legacy resolve.Allow* global variables.
// 它用于替代历史上的 resolve.Allow* 全局变量。
var defaultStarlarkFileOptions = syntax.FileOptions{
	Set:             true,
	While:           true,
	TopLevelControl: true,
	GlobalReassign:  true,
	Recursion:       true, // disable recursion check for functions in this file
}

// lazyContext implements lazily-converted script context (similar to Lua UserData).
// lazyContext 实现 Lazy Loading Context（类似 Lua UserData）。
// It implements starlark.Mapping and converts only when ctx["key"] is accessed.
// 实现 starlark.Mapping 接口，脚本使用 ctx["key"] 语法访问时才转换。
type lazyContext struct {
	engine *Engine
	ctx    *zscript.ScriptContext
}

// Implements starlark.Value.
// 实现 starlark.Value 接口。
func (lc *lazyContext) String() string        { return fmt.Sprintf("<context actor=%d>", lc.ctx.ActorID) }
func (lc *lazyContext) Type() string          { return "context" }
func (lc *lazyContext) Freeze()               {} // Context 是只读的
func (lc *lazyContext) Truth() starlark.Bool  { return starlark.True }
func (lc *lazyContext) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable type: context") }

// Implements starlark.Mapping (core: lazy loading).
// 实现 starlark.Mapping 接口（核心：Lazy Loading）。
func (lc *lazyContext) Get(key starlark.Value) (starlark.Value, bool, error) {
	keyStr, ok := key.(starlark.String)
	if !ok {
		return nil, false, fmt.Errorf("context key must be string, got %s", key.Type())
	}

	ctx := lc.ctx
	name := string(keyStr)

	// ⚡️ 只转换被访问的字段
	switch name {
	case "ActorID":
		return starlark.MakeInt(int(ctx.ActorID)), true, nil
	case "ActorType":
		return starlark.MakeInt(int(ctx.ActorType)), true, nil
	case "MsgID":
		return starlark.MakeInt(int(ctx.MsgID)), true, nil
	case "AuthID":
		return starlark.MakeInt64(ctx.AuthID), true, nil
	case "TraceID":
		// TraceID 是 uint64；避免 uint64 -> int64 溢出回绕，直接导出为 uint64。
		return starlark.MakeUint64(ctx.TraceID), true, nil
	case "NowMillis":
		return starlark.MakeInt64(ctx.NowMillis), true, nil
	case "Owner":
		if ctx.Owner != nil {
			return lc.engine.goToStarlark(ctx.Owner), true, nil
		}
		return starlark.None, true, nil
	case "MsgData":
		if ctx.MsgData != nil {
			return lc.engine.goToStarlark(ctx.MsgData), true, nil
		}
		return starlark.None, true, nil
	case "Metadata":
		if ctx.Metadata != nil {
			return lc.engine.goToStarlark(ctx.Metadata), true, nil
		}
		return starlark.None, true, nil
	default:
		return nil, false, nil // Key not found
	}
}

// Implements starlark.HasAttrs (supports ctx.ActorID syntax).
// 实现 starlark.HasAttrs 接口（支持 ctx.ActorID 语法）。
func (lc *lazyContext) Attr(name string) (starlark.Value, error) {
	val, found, err := lc.Get(starlark.String(name))
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, starlark.NoSuchAttrError(fmt.Sprintf("context has no .%s attribute", name))
	}
	return val, nil
}

func (lc *lazyContext) AttrNames() []string {
	return []string{"ActorID", "ActorType", "MsgID", "AuthID", "TraceID", "NowMillis", "Owner", "MsgData", "Metadata"}
}

// Engine is the Starlark engine implementation of zscript.IScriptEngine.
// Engine 是 zscript.IScriptEngine 的 Starlark 引擎实现。
type Engine struct {
	config *zscript.EngineConfig
	logger *zlog.Logger
	stats  *zscript.StatsCollector

	// atomic.Value stores *FunctionIndex for lock-free reads via atomic swap.
	// atomic.Value 存储 *FunctionIndex
	// 使用原子替换来实现无锁读取。
	functionIndex atomic.Value

	// Engine runtime state.
	// 引擎状态。
	closed   atomic.Bool
	argsPool *zpool.Pool[[]starlark.Value]
	writeMu  sync.Mutex
}

// FunctionIndex is the preprocessed index for loaded scripts and callable functions.
// FunctionIndex 是已加载脚本与可调用函数的预处理索引。
type FunctionIndex struct {
	// Funcs maps function name to callable (O(1) lookup).
	// Funcs 函数名 -> 函数对象（O(1) 查找）。
	Funcs map[string]starlark.Callable
	// Modules maps script path to module globals (for Reload).
	// Modules 脚本路径 -> 模块内容（用于 Reload）。
	Modules map[string]starlark.StringDict
}

// NewEngine creates a Starlark engine.
// NewEngine 创建 Starlark 引擎。
// If config is nil, it uses zscript.DefaultEngineConfig() and forces Type=EngineTypeStarlark.
// 若 config 为 nil，则使用 zscript.DefaultEngineConfig() 并强制 Type=EngineTypeStarlark。
func NewEngine(config *zscript.EngineConfig) (*Engine, error) {
	if config == nil {
		config = zscript.DefaultEngineConfig()
		config.Type = zscript.EngineTypeStarlark
	}

	engine := &Engine{
		config: config,
		logger: zlog.GetDefaultLog(),
		stats:  zscript.NewStatsCollector("starlark"),
		argsPool: zpoolobs.NewObservedPool(zpoolobs.PoolNameZStarlarkArgs, func() []starlark.Value {
			return make([]starlark.Value, 0, 8)
		}),
	}

	// 初始化空索引
	engine.functionIndex.Store(&FunctionIndex{
		Funcs:   make(map[string]starlark.Callable),
		Modules: make(map[string]starlark.StringDict),
	})

	engine.logger.Info("Starlark engine created",
		zap.String("scriptDir", config.ScriptDir))

	return engine, nil
}

// LoadScript loads one script file and updates the function index (copy-on-write).
// LoadScript 加载单个脚本文件，并更新函数索引（copy-on-write）。
func (e *Engine) LoadScript(path string) error {
	return e.loadScriptsInternal([]string{path}, false)
}

// LoadScripts loads a batch of script files and updates the function index (copy-on-write).
// LoadScripts 批量加载脚本文件，并更新函数索引（copy-on-write）。
func (e *Engine) LoadScripts(paths []string) error {
	return e.loadScriptsInternal(paths, false)
}

// ReloadScript hot-reloads one script file and updates the index (copy-on-write).
// ReloadScript 热重载单个脚本文件，并更新索引（copy-on-write）。
func (e *Engine) ReloadScript(path string) error {
	e.stats.RecordReload()
	return e.loadScriptsInternal([]string{path}, true)
}

// ReloadAllScripts hot-reloads all previously loaded scripts.
// ReloadAllScripts 热重载所有已加载脚本。
func (e *Engine) ReloadAllScripts() error {
	e.stats.RecordReload()
	current := e.getFunctionIndex()
	paths := make([]string, 0, len(current.Modules))
	for path := range current.Modules {
		paths = append(paths, path)
	}
	return e.loadScriptsInternal(paths, true)
}

func (e *Engine) loadScriptsInternal(paths []string, isReload bool) error {
	if len(paths) == 0 {
		return nil
	}

	// 检查引擎是否已关闭
	if e.closed.Load() {
		return zerrs.New(zscript.ErrTypeEngine, "engine is closed")
	}

	start := time.Now()
	defer func() {
		e.stats.RecordCall(time.Since(start), nil)
	}()

	// 1. 编译新脚本
	newModules := make(map[string]starlark.StringDict, len(paths))

	// 给加载过程设置超时（防止顶层代码死循环）
	timeout := e.config.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	for _, path := range paths {
		code, err := os.ReadFile(path) // #nosec G304 -- path is constrained to configured script roots
		if err != nil {
			return zerrs.Wrapf(err, zscript.ErrTypeScript, "read failed: %s", path)
		}

		// 每个脚本使用独立的 Thread，避免状态污染
		thread := &starlark.Thread{
			Name:  "loader",
			Print: e.printHandler,
		}
		thread.SetMaxExecutionSteps(5000000) // 防止加载时死循环

		// 超时保护：不再为每个脚本额外起 goroutine；用 timer 触发 thread.Cancel。
		// 若脚本顶层卡死，Cancel 会中断执行并让 ExecFile 返回错误。
		timer := time.AfterFunc(timeout, func() {
			thread.Cancel("load timeout")
		})

		globals, err := func() (starlark.StringDict, error) {
			defer func() {
				_ = timer.Stop()
			}()
			defer func() {
				if r := recover(); r != nil {
					err = zerrs.Newf(zscript.ErrTypeScript, "panic: %v", r)
				}
			}()
			return starlark.ExecFileOptions(&defaultStarlarkFileOptions, thread, path, code, nil)
		}()
		if err != nil {
			// 若是超时取消，统一成更清晰的错误文案
			return zerrs.Wrapf(err, zscript.ErrTypeScript, "exec failed: %s", path)
		}
		newModules[path] = globals
	}

	// 2. 更新索引 (Copy-On-Write)
	e.writeMu.Lock()
	defer e.writeMu.Unlock()

	// 二次检查 closed 状态（避免 Close 和 Load 竞态）
	if e.closed.Load() {
		return zerrs.New(zscript.ErrTypeEngine, "engine is closed")
	}

	oldIndex := e.getFunctionIndex()

	// 合并 Modules
	finalModules := make(map[string]starlark.StringDict, len(oldIndex.Modules)+len(newModules))
	for k, v := range oldIndex.Modules {
		finalModules[k] = v
	}
	for k, v := range newModules {
		finalModules[k] = v
	}

	// 按路径排序，确保函数冲突时覆盖顺序确定
	sortedPaths := make([]string, 0, len(finalModules))
	for path := range finalModules {
		sortedPaths = append(sortedPaths, path)
	}
	sort.Strings(sortedPaths)

	// 重建 Funcs 索引
	finalFuncs := make(map[string]starlark.Callable)
	for _, path := range sortedPaths {
		globals := finalModules[path]
		for name, val := range globals {
			// 只索引 Callable
			if f, ok := val.(starlark.Callable); ok {
				// 记录冲突的详细信息
				if oldFn, exists := finalFuncs[name]; exists {
					e.logger.Warn("Function name collision",
						zap.String("func", name),
						zap.String("old_source", e.findFunctionSource(oldFn, oldIndex.Modules)),
						zap.String("new_source", path))
				}
				finalFuncs[name] = f
			}
		}
	}

	e.functionIndex.Store(&FunctionIndex{
		Funcs:   finalFuncs,
		Modules: finalModules,
	})

	e.logger.Info("Starlark scripts loaded",
		zap.Int("count", len(paths)),
		zap.Int("total_funcs", len(finalFuncs)),
		zap.Bool("reload", isReload))
	return nil
}

// findFunctionSource finds module path that owns a function.
// findFunctionSource 查找函数所属的模块路径。
func (e *Engine) findFunctionSource(fn starlark.Callable, modules map[string]starlark.StringDict) string {
	for path, globals := range modules {
		for _, val := range globals {
			if val == fn {
				return path
			}
		}
	}
	return "unknown"
}

// Call executes a Starlark function.
// Call 执行一个 Starlark 函数。
// It is optimized for hot paths: O(1) lookup, synchronous execution, and low-allocation argument handling.
// 它面向热路径优化：O(1) 查找、同步执行、以及低分配的参数处理。
func (e *Engine) Call(callCtx context.Context, params *zscript.CallParams, function string, args ...interface{}) (interface{}, error) {
	// 检查引擎是否已关闭
	if e.closed.Load() {
		return nil, zerrs.New(zscript.ErrTypeEngine, "engine is closed")
	}
	if callCtx == nil {
		return nil, zerrs.New(zerrs.ErrTypeValidation, "zstarlark.Engine.Call: ctx is required")
	}
	if params == nil {
		return nil, zerrs.New(zerrs.ErrTypeValidation, "zstarlark.Engine.Call: params is nil")
	}

	start := time.Now()
	var err error

	ctx := zscript.GetContext(params.ActorID, params.ActorType)
	defer func() {
		e.stats.RecordCall(time.Since(start), err)
		zscript.PutContext(ctx)
	}()

	ctx.WithOwner(params.Owner).
		WithTraceID(params.TraceID).
		WithMessage(params.MsgID, params.AuthID, params.MsgData)
	if params.Metadata != nil {
		for k, v := range params.Metadata {
			ctx.WithMetadata(k, v)
		}
	}

	// 1. O(1) 查找函数
	index := e.getFunctionIndex()
	fn, ok := index.Funcs[function]
	if !ok {
		return nil, zscript.ErrFunctionNotFound
	}

	// 2. 准备 Thread
	thread := &starlark.Thread{
		Name:  fmt.Sprintf("actor_%d", ctx.ActorID),
		Print: e.printHandler,
	}

	// If caller cancels the context, proactively interrupt starlark execution.
	// This keeps ScriptEngine's ctx semantics consistent with other engines.
	if callCtx.Err() != nil {
		thread.Cancel(callCtx.Err().Error())
	} else {
		done := make(chan struct{})
		go func() {
			select {
			case <-callCtx.Done():
				thread.Cancel("call cancelled")
			case <-done:
			}
		}()
		defer close(done)
	}

	// ⚡️ 超时控制：零分配步数限制（Zero-Timer 策略）
	//
	// SetMaxExecutionSteps 是纯 CPU 指令计数器，开销极低（~1ns），零分配
	//
	// 【步数比例】1ms = 10000 步
	// - 理论依据：简单函数 ~100-200 步，1ms ≈ 2500-5000 步（实测）
	// - 安全余量：2 倍（考虑复杂业务场景）
	// - 生产经验：宁可多给，避免误杀正常业务
	//
	// 【超时配置】
	// - Timeout = 0:  不设置限制（极速模式，用于信任脚本）
	// - Timeout < 0:  使用默认 5s（50M 步）
	// - Timeout = 5s: 50M 步（足够复杂业务）
	timeout := e.config.Timeout
	if timeout > 0 {
		// 根据超时计算步数（1ms = 10000 步）
		steps := uint64(timeout.Milliseconds()) * 10000
		if steps < 10000 {
			steps = 10000 // 最少 1 万步（避免配置错误）
		}
		thread.SetMaxExecutionSteps(steps)
	} else if timeout < 0 {
		// 负数表示使用默认 5s 超时（50M 步）
		thread.SetMaxExecutionSteps(50000000)
	}
	// timeout == 0: 不设置限制，极速模式

	// 3. 参数转换（使用框架池化技术）
	// ✅ 使用 lazyContext 实现 Lazy Loading（只转换被访问的字段）
	starlarkCtx := &lazyContext{engine: e, ctx: ctx}
	starlarkArgs := e.argsPool.Get()[:0]
	starlarkArgs = append(starlarkArgs, starlarkCtx)
	for _, arg := range args {
		starlarkArgs = append(starlarkArgs, e.goToStarlark(arg))
	}
	defer func() {
		e.argsPool.Put(starlarkArgs[:0])
	}()

	// 4. 执行（无 Timer，使用步数限制）
	resultVal, evalErr := starlark.Call(thread, fn, starlarkArgs, nil)

	if evalErr != nil {
		// 检查步数超限错误（SetMaxExecutionSteps 触发）
		errMsg := evalErr.Error()
		if evalErrVal, ok := evalErr.(*starlark.EvalError); ok {
			errMsg = evalErrVal.Msg
		}

		// 步数超限或手动取消都视为超时
		// Starlark cancellation reason string may vary, so we use contains match.
		if errMsg == "maximum number of computation steps exceeded" ||
			strings.Contains(errMsg, "computation cancelled") {
			e.stats.RecordTimeout()
			return nil, zscript.ErrScriptTimeout
		}

		err = zerrs.Wrap(evalErr, zscript.ErrTypeScript, "starlark call failed")
		return nil, err
	}

	return e.starlarkToGo(resultVal), nil
}

func (e *Engine) getFunctionIndex() *FunctionIndex {
	return e.functionIndex.Load().(*FunctionIndex)
}

func (e *Engine) goToStarlark(val interface{}) starlark.Value {
	if val == nil {
		return starlark.None
	}
	switch v := val.(type) {
	case bool:
		return starlark.Bool(v)
	case string:
		return starlark.String(v)
	case []byte:
		return starlark.String(string(v))
	case float64:
		return starlark.Float(v)
	case float32:
		return starlark.Float(v)
	case int:
		return starlark.MakeInt(v)
	case int8:
		return starlark.MakeInt(int(v))
	case int16:
		return starlark.MakeInt(int(v))
	case int32:
		return starlark.MakeInt(int(v))
	case int64:
		return starlark.MakeInt64(v)
	case uint:
		return starlark.MakeUint(v)
	case uint8:
		return starlark.MakeUint(uint(v))
	case uint16:
		return starlark.MakeUint(uint(v))
	case uint32:
		return starlark.MakeUint(uint(v))
	case uint64:
		return starlark.MakeUint64(v)
	case map[string]interface{}:
		// ✅ 预分配精确容量
		dict := starlark.NewDict(len(v))
		for k, val := range v {
			// 忽略 SetKey 错误（字符串 key 不会失败）
			_ = dict.SetKey(starlark.String(k), e.goToStarlark(val))
		}
		return dict
	case []interface{}:
		// ✅ 预分配精确长度，避免扩容
		list := make([]starlark.Value, len(v))
		for i, val := range v {
			list[i] = e.goToStarlark(val)
		}
		return starlark.NewList(list)
	default:
		return starlark.None
	}
}

func (e *Engine) starlarkToGo(val starlark.Value) interface{} {
	if val == nil || val == starlark.None {
		return nil
	}
	switch v := val.(type) {
	case starlark.Bool:
		return bool(v)
	case starlark.Int:
		i, _ := v.Int64()
		return i
	case starlark.Float:
		return float64(v)
	case starlark.String:
		return string(v)
	case *starlark.List:
		result := make([]interface{}, v.Len())
		for i := 0; i < v.Len(); i++ {
			result[i] = e.starlarkToGo(v.Index(i))
		}
		return result
	case starlark.Tuple: // 支持 Tuple
		result := make([]interface{}, v.Len())
		for i := 0; i < v.Len(); i++ {
			result[i] = e.starlarkToGo(v.Index(i))
		}
		return result
	case *starlark.Dict:
		result := make(map[string]interface{})
		for _, item := range v.Items() {
			if keyStr, ok := item[0].(starlark.String); ok {
				result[string(keyStr)] = e.starlarkToGo(item[1])
			}
		}
		return result
	case *starlark.Set: // 支持 Set (转为 slice)
		result := make([]interface{}, 0, v.Len())
		iter := v.Iterate()
		defer iter.Done()
		var x starlark.Value
		for iter.Next(&x) {
			result = append(result, e.starlarkToGo(x))
		}
		return result
	default:
		return nil
	}
}

func (e *Engine) printHandler(_ *starlark.Thread, msg string) {
	e.logger.Info("STARLARK_PRINT", zap.String("msg", msg))
}

// GetStats returns a snapshot of engine statistics.
// GetStats 返回引擎统计信息快照。
func (e *Engine) GetStats() *zscript.EngineStats {
	stats := e.stats.GetStats()
	idx := e.getFunctionIndex()

	stats.ScriptFiles = make([]zscript.ScriptFileInfo, 0, len(idx.Modules))
	for path := range idx.Modules {
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
	if stats.Metadata == nil {
		stats.Metadata = make(map[string]interface{})
	}
	stats.Metadata["total_funcs"] = len(idx.Funcs)
	stats.Metadata["total_modules"] = len(idx.Modules)
	return stats
}

// GetType returns engine type identifier ("starlark").
// GetType 返回引擎类型标识（"starlark"）。
func (e *Engine) GetType() string { return string(zscript.EngineTypeStarlark) }

// Close marks engine as closed and clears loaded indices.
// Close 标记引擎关闭并清空已加载索引。
func (e *Engine) Close() {
	// 设置 closed 标记
	e.closed.Store(true)

	// 清空索引
	e.functionIndex.Store(&FunctionIndex{
		Funcs:   make(map[string]starlark.Callable),
		Modules: make(map[string]starlark.StringDict),
	})

	e.logger.Info("Starlark engine closed")
}
