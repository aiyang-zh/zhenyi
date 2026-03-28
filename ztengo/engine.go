package ztengo

import (
	"context"
	"errors"
	"fmt"
	"github.com/aiyang-zh/zhenyi-base/zcoll"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aiyang-zh/zhenyi-base/zerrs"
	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi/zscript"
	"github.com/d5/tengo/v2"
	"github.com/d5/tengo/v2/stdlib"
	"go.uber.org/zap"
)

var (
	// Precompiled regex patterns for performance.
	// 预编译正则表达式（性能优化）。
	funcDefPattern    = regexp.MustCompile(`\bfunc\s+([a-zA-Z_][a-zA-Z0-9_]*)\s*\(`)
	assignPattern     = regexp.MustCompile(`\b([a-zA-Z_][a-zA-Z0-9_]*)\s*:=\s*func\s*\(`)
	identifierPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

	// Global variable detection patterns (risky-mode identification).
	// 全局变量检测（危险模式识别）。
	globalMapPattern   = regexp.MustCompile(`\b([a-zA-Z_][a-zA-Z0-9_]*)\s*:=\s*\{`) // name := {...}
	globalArrayPattern = regexp.MustCompile(`\b([a-zA-Z_][a-zA-Z0-9_]*)\s*:=\s*\[`) // name := [...]
)

// ==========================================
// 1. Zero-Copy Context
// ==========================================

// Context is a Tengo Object that exposes zscript.ScriptContext to scripts with lazy/zero-copy access.
// Context 是 Tengo 的 Object：将 zscript.ScriptContext 暴露给脚本，并尽量保持零拷贝/按需访问。
type Context struct {
	tengo.ObjectImpl
	ctx *zscript.ScriptContext
	eng *Engine
}

// TypeName returns Tengo type name.
// TypeName 返回 Tengo 类型名。
func (o *Context) TypeName() string { return "context" }

// String returns debug string.
// String 返回调试字符串。
func (o *Context) String() string { return fmt.Sprintf("Context{ActorID:%d}", o.ctx.ActorID) }

// IndexGet implements Tengo index access like ctx["ActorID"].
// IndexGet 实现 Tengo 的索引访问，例如 ctx["ActorID"]。
func (o *Context) IndexGet(index tengo.Object) (tengo.Object, error) {
	key, ok := index.(*tengo.String)
	if !ok {
		return tengo.UndefinedValue, nil
	}

	switch key.Value {
	case "ActorID":
		return &tengo.Int{Value: int64(o.ctx.ActorID)}, nil
	case "ActorType":
		return &tengo.Int{Value: int64(o.ctx.ActorType)}, nil
	case "MsgID":
		return &tengo.Int{Value: int64(o.ctx.MsgID)}, nil
	case "AuthID":
		return &tengo.Int{Value: o.ctx.AuthID}, nil
	case "TraceID":
		// TraceID 是 uint64；不能直接转 int64，否则会在超过 MaxInt64 时溢出回绕。
		// 这里在安全范围内返回数字，超出范围则降级为字符串（避免回绕导致错误判断）。
		if o.ctx.TraceID <= ^uint64(0)>>1 {
			// #nosec G115 -- conversion is guarded by TraceID <= MaxInt64.
			return &tengo.Int{Value: int64(o.ctx.TraceID)}, nil
		}
		return &tengo.String{Value: fmt.Sprintf("%d", o.ctx.TraceID)}, nil
	case "NowMillis":
		return &tengo.Int{Value: o.ctx.NowMillis}, nil
	case "Owner":
		if o.ctx.Owner != nil {
			return o.eng.goToTengo(o.ctx.Owner), nil
		}
		return tengo.UndefinedValue, nil
	case "MsgData":
		if o.ctx.MsgData != nil {
			return o.eng.goToTengo(o.ctx.MsgData), nil
		}
		return tengo.UndefinedValue, nil
	case "Metadata":
		if o.ctx.Metadata != nil {
			return o.eng.goToTengo(o.ctx.Metadata), nil
		}
		return tengo.UndefinedValue, nil
	}
	return tengo.UndefinedValue, nil
}

// ==========================================
// 2. Precompiled Stub Cache
// ==========================================

// funcStub stores precompiled code for a specific function + argument count.
// funcStub 存储针对特定函数+参数数量的预编译代码。
type funcStub struct {
	compiled *tengo.Compiled
}

// Engine is Tengo engine implementation of zscript.IScriptEngine.
// Engine 是 zscript.IScriptEngine 的 Tengo 引擎实现。
type Engine struct {
	config *zscript.EngineConfig
	logger *zlog.Logger
	stats  *zscript.StatsCollector

	// Stub cache: "functionName:argCount" -> *funcStub.
	// 存根缓存: "functionName:argCount" -> *funcStub。
	stubs *zcoll.SyncMap[string, *funcStub]

	// Security: function-name whitelist (anti-injection).
	// 安全：函数名白名单（防注入）。
	validFunctions atomic.Value // map[string]bool

	// Security: global variable detection (pollution-risk warning).
	// 安全：全局变量检测（污染风险警告）。
	globalVariables atomic.Value // []string

	// Source mappings.
	// 源码映射。
	sourceMap  atomic.Value // map[string][]byte
	sourceCode atomic.Value // string (预拼接的完整源码)

	closed  atomic.Bool
	writeMu sync.Mutex
}

// NewEngine creates a Tengo engine.
// NewEngine 创建 Tengo 引擎。
// If config is nil, it uses zscript.DefaultEngineConfig() and forces Type=EngineTypeTengo.
// 若 config 为 nil，则使用 zscript.DefaultEngineConfig() 并强制 Type=EngineTypeTengo。
func NewEngine(config *zscript.EngineConfig) (*Engine, error) {
	if config == nil {
		config = zscript.DefaultEngineConfig()
		config.Type = zscript.EngineTypeTengo
	}

	engine := &Engine{
		config: config,
		logger: zlog.GetDefaultLog(),
		stats:  zscript.NewStatsCollector("tengo"),
		stubs:  zcoll.NewSyncMap[string, *funcStub](),
	}

	engine.sourceMap.Store(make(map[string][]byte))
	engine.sourceCode.Store("")
	engine.validFunctions.Store(make(map[string]bool))
	engine.globalVariables.Store([]string{})

	engine.logger.Info("Tengo engine created", zap.String("scriptDir", config.ScriptDir))
	return engine, nil
}

// LoadScript loads one script file and updates internal source mappings (copy-on-write).
// LoadScript 加载单个脚本文件，并更新内部源码映射（copy-on-write）。
func (e *Engine) LoadScript(path string) error {
	return e.loadScriptsInternal([]string{path}, false)
}

// LoadScripts loads a batch of script files and updates internal source mappings (copy-on-write).
// LoadScripts 批量加载脚本文件，并更新内部源码映射（copy-on-write）。
func (e *Engine) LoadScripts(paths []string) error {
	return e.loadScriptsInternal(paths, false)
}

// ReloadScript hot-reloads one script file.
// ReloadScript 热重载单个脚本文件。
func (e *Engine) ReloadScript(path string) error {
	e.stats.RecordReload()
	return e.loadScriptsInternal([]string{path}, true)
}

// ReloadAllScripts hot-reloads all previously loaded scripts.
// ReloadAllScripts 热重载所有已加载脚本。
func (e *Engine) ReloadAllScripts() error {
	e.stats.RecordReload()
	current := e.getSourceMap()
	paths := make([]string, 0, len(current))
	for path := range current {
		paths = append(paths, path)
	}
	return e.loadScriptsInternal(paths, true)
}

func (e *Engine) loadScriptsInternal(paths []string, isReload bool) error {
	if len(paths) == 0 {
		return nil
	}
	if e.closed.Load() {
		return zerrs.New(zscript.ErrTypeEngine, "engine is closed")
	}

	start := time.Now()
	defer func() {
		e.stats.RecordCall(time.Since(start), nil)
	}()

	newSources := make(map[string][]byte, len(paths))
	for _, path := range paths {
		code, err := os.ReadFile(path) // #nosec G304 -- path comes from engine-controlled/prog map keys
		if err != nil {
			return zerrs.Wrapf(err, zscript.ErrTypeScript, "read failed: %s", path)
		}
		// 语法检查（启用标准库支持，采用白名单模块）
		s := tengo.NewScript(code)
		// ✅ 启用必要的 Tengo 标准库模块（白名单）
		// 如需扩展，可由业务在配置中显式声明。
		s.SetImports(stdlib.GetModuleMap(
			"fmt",
			"text",
			"math",
			"times",
			"rand",
			"json",
		))
		if _, err := s.Compile(); err != nil {
			return zerrs.Wrapf(err, zscript.ErrTypeScript, "compile check failed: %s", path)
		}
		newSources[path] = code
	}

	e.writeMu.Lock()
	defer e.writeMu.Unlock()

	if e.closed.Load() {
		return zerrs.New(zscript.ErrTypeEngine, "engine is closed")
	}

	oldSources := e.getSourceMap()
	finalSources := make(map[string][]byte, len(oldSources)+len(newSources))
	for k, v := range oldSources {
		finalSources[k] = v
	}
	for k, v := range newSources {
		finalSources[k] = v
	}

	// 预拼接完整源码（按文件名排序确保确定性）
	sortedKeys := make([]string, 0, len(finalSources))
	for k := range finalSources {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)
	var sb strings.Builder
	for _, k := range sortedKeys {
		sb.Write(finalSources[k])
		sb.WriteByte('\n')
	}
	fullCode := sb.String()

	// 🛡️ 安全：扫描函数定义，建立白名单（防注入攻击）
	validFuncs := e.extractFunctionNames(fullCode)

	// ⚠️ 安全检测：扫描全局可变变量（在函数扫描之后）
	globalVars := e.detectGlobalVariables(fullCode, validFuncs)

	// 原子更新所有状态
	e.validFunctions.Store(validFuncs)
	e.globalVariables.Store(globalVars)
	e.sourceMap.Store(finalSources)
	e.sourceCode.Store(fullCode)

	// 清空存根缓存（函数定义可能变了）
	e.stubs.Range(func(key string, value *funcStub) bool {
		e.stubs.Delete(key)
		return true
	})

	e.logger.Info("Tengo scripts loaded",
		zap.Int("total_files", len(finalSources)),
		zap.Int("valid_functions", len(validFuncs)),
		zap.Int("global_variables_detected", len(globalVars)),
		zap.Bool("reload", isReload))

	// 如果检测到全局变量，给出针对性警告
	if len(globalVars) > 0 {
		e.logger.Warn("Global variable pollution risk detected",
			zap.Strings("global_vars", globalVars),
			zap.String("risk", "Clone() may share Map/Array references between requests"),
			zap.String("impact", "Possible data leakage or state corruption in concurrent calls"),
			zap.String("fix", "Move these to function-local scope or pass via Context"))
	}

	return nil
}

// extractFunctionNames extracts all function definitions for security whitelist.
// extractFunctionNames 从源码中提取所有函数定义（安全白名单）。
// Match patterns: func name(...) or name := func(...).
// 匹配模式：func name(...) 或 name := func(...)。
func (e *Engine) extractFunctionNames(code string) map[string]bool {
	funcs := make(map[string]bool)

	// 模式1: func functionName(...)
	matches := funcDefPattern.FindAllStringSubmatch(code, -1)
	for _, match := range matches {
		if len(match) > 1 {
			funcs[match[1]] = true
		}
	}

	// 模式2: functionName := func(...)
	matches = assignPattern.FindAllStringSubmatch(code, -1)
	for _, match := range matches {
		if len(match) > 1 {
			funcs[match[1]] = true
		}
	}

	return funcs
}

// detectGlobalVariables detects global mutable variables (Map/Array) in scripts.
// detectGlobalVariables 检测脚本中的全局可变变量（Map/Array）。
// 返回检测到的全局变量名列表
// 参数 knownFuncs 用于排除函数定义
func (e *Engine) detectGlobalVariables(code string, knownFuncs map[string]bool) []string {
	globalVars := make([]string, 0)
	varMap := make(map[string]bool)

	// 检测全局 Map：name := {...}
	matches := globalMapPattern.FindAllStringSubmatch(code, -1)
	for _, match := range matches {
		if len(match) > 1 {
			varName := match[1]
			// 排除函数（函数已经在白名单中）
			if !knownFuncs[varName] {
				varMap[varName] = true
			}
		}
	}

	// 检测全局 Array：name := [...]
	matches = globalArrayPattern.FindAllStringSubmatch(code, -1)
	for _, match := range matches {
		if len(match) > 1 {
			varName := match[1]
			// 排除函数
			if !knownFuncs[varName] {
				varMap[varName] = true
			}
		}
	}

	// 转换为列表
	for varName := range varMap {
		globalVars = append(globalVars, varName)
	}

	return globalVars
}

// isValidFunctionName validates function name against whitelist (anti-injection).
// isValidFunctionName 验证函数名是否在白名单中（防注入）。
func (e *Engine) isValidFunctionName(function string) bool {
	// 1. 基础校验：必须是合法的标识符格式（使用预编译正则）
	if !identifierPattern.MatchString(function) {
		return false
	}

	// 2. 白名单校验
	validFuncs := e.getValidFunctions()
	return validFuncs[function]
}

// getOrCompileStub gets or creates precompiled stub.
// getOrCompileStub 获取或创建预编译存根。
func (e *Engine) getOrCompileStub(function string, argCount int) (*funcStub, error) {
	// 核心防御：函数名注入防护
	// 1. 白名单校验：确保函数确实存在于已加载的脚本中
	// 2. 格式校验：防止通过特殊字符进行代码注入
	if !e.isValidFunctionName(function) {
		e.logger.Warn("Function name validation failed (possible injection attempt)",
			zap.String("function", function))
		return nil, zscript.ErrFunctionNotFound
	}

	key := fmt.Sprintf("%s:%d", function, argCount)

	// 快速查找
	if v, ok := e.stubs.Load(key); ok {
		return v, nil
	}

	// 缓存未命中，编译存根
	baseCode := e.getSourceCode()
	if baseCode == "" {
		return nil, zscript.ErrFunctionNotFound
	}

	// 拼接调用代码
	var sb strings.Builder
	sb.WriteString(baseCode)
	sb.WriteString("\n__res := ")
	sb.WriteString(function)
	sb.WriteString("(__ctx")
	for i := 0; i < argCount; i++ {
		fmt.Fprintf(&sb, ", __arg%d", i)
	}
	sb.WriteString(")")

	// 编译（使用与编译检查相同的标准库白名单）
	s := tengo.NewScript([]byte(sb.String()))
	s.SetImports(stdlib.GetModuleMap(
		"fmt",
		"text",
		"math",
		"times",
		"rand",
		"json",
	))
	// 声明全局变量（编译时需要）
	_ = s.Add("__ctx", tengo.UndefinedValue)
	for i := 0; i < argCount; i++ {
		_ = s.Add(fmt.Sprintf("__arg%d", i), tengo.UndefinedValue)
	}

	compiled, err := s.Compile()
	if err != nil {
		return nil, err
	}

	stub := &funcStub{compiled: compiled}
	e.stubs.Store(key, stub)

	return stub, nil
}

// Call executes script function.
// Call 执行函数。
func (e *Engine) Call(callCtx context.Context, params *zscript.CallParams, function string, args ...interface{}) (interface{}, error) {
	if e.closed.Load() {
		return nil, zerrs.New(zscript.ErrTypeEngine, "engine is closed")
	}
	if callCtx == nil {
		return nil, zerrs.New(zerrs.ErrTypeValidation, "ztengo.Engine.Call: ctx is required")
	}
	if params == nil {
		return nil, zerrs.New(zerrs.ErrTypeValidation, "ztengo.Engine.Call: params is nil")
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

	// 1. 获取预编译存根（首次慢，后续O(1)查找）
	stub, err := e.getOrCompileStub(function, len(args))
	if err != nil {
		if strings.Contains(err.Error(), "unresolved reference") {
			return nil, zscript.ErrFunctionNotFound
		}
		return nil, zerrs.Wrap(err, zscript.ErrTypeScript, "prepare stub failed")
	}

	// 2. Clone VM（轻量级）
	vm := stub.compiled.Clone()

	// 3. 设置参数（运行时注入）
	if err := vm.Set("__ctx", &Context{ctx: ctx, eng: e}); err != nil {
		return nil, zerrs.Wrap(err, zscript.ErrTypeScript, "set ctx failed")
	}

	for i, arg := range args {
		if err := vm.Set(fmt.Sprintf("__arg%d", i), e.goToTengo(arg)); err != nil {
			return nil, zerrs.Wrap(err, zscript.ErrTypeScript, "set arg failed")
		}
	}

	// 4. 执行
	timeout := e.config.Timeout
	if timeout == 0 {
		// Fast Path
		if err = vm.RunContext(callCtx); err != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				e.stats.RecordTimeout()
				return nil, zscript.ErrScriptTimeout
			}
			return nil, zerrs.Wrap(err, zscript.ErrTypeScript, "run failed")
		}
	} else {
		// Safe Path
		if timeout < 0 {
			timeout = 5 * time.Second
		}
		// 基于调用方 ctx 派生执行超时，继承 trace/value。
		tCtx, cancel := context.WithTimeout(callCtx, timeout)
		defer cancel()

		if err = vm.RunContext(tCtx); err != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
				e.stats.RecordTimeout()
				return nil, zscript.ErrScriptTimeout
			}
			return nil, zerrs.Wrap(err, zscript.ErrTypeScript, "run failed")
		}
	}

	// 5. 获取结果
	resVar := vm.Get("__res")
	if resVar == nil {
		return nil, nil
	}

	return e.tengoToGo(resVar.Object()), nil
}

// GetStats returns a snapshot of engine statistics.
// GetStats 返回引擎统计信息快照。
func (e *Engine) GetStats() *zscript.EngineStats {
	stats := e.stats.GetStats()
	sourceMap := e.getSourceMap()
	validFuncs := e.getValidFunctions()
	globalVars := e.getGlobalVariables()

	stubCount := 0
	e.stubs.Range(func(_ string, _ *funcStub) bool {
		stubCount++
		return true
	})

	stats.ScriptFiles = make([]zscript.ScriptFileInfo, 0, len(sourceMap))
	for path := range sourceMap {
		info, err := os.Stat(path)
		if err == nil {
			stats.ScriptFiles = append(stats.ScriptFiles, zscript.ScriptFileInfo{
				Path: path, Size: info.Size(), LoadTime: info.ModTime(),
			})
		}
	}

	if stats.Metadata == nil {
		stats.Metadata = make(map[string]interface{})
	}
	stats.Metadata["total_scripts"] = len(sourceMap)
	stats.Metadata["valid_functions"] = len(validFuncs)
	stats.Metadata["cached_stubs"] = stubCount
	stats.Metadata["global_variables"] = len(globalVars)
	if len(globalVars) > 0 {
		stats.Metadata["global_variable_names"] = globalVars
	}

	return stats
}

// GetType returns engine type identifier ("tengo").
// GetType 返回引擎类型标识（"tengo"）。
func (e *Engine) GetType() string {
	return string(zscript.EngineTypeTengo)
}

// Close marks engine as closed and clears loaded sources and caches.
// Close 标记引擎关闭，并清理已加载源码与缓存。
func (e *Engine) Close() {
	e.closed.Store(true)
	e.sourceMap.Store(make(map[string][]byte))
	e.sourceCode.Store("")
	e.validFunctions.Store(make(map[string]bool))
	e.globalVariables.Store([]string{})
	e.stubs.Range(func(key string, value *funcStub) bool {
		e.stubs.Delete(key)
		return true
	})
	e.logger.Info("Tengo engine closed")
}

func (e *Engine) getSourceMap() map[string][]byte {
	v := e.sourceMap.Load()
	if v == nil {
		return map[string][]byte{}
	}
	return v.(map[string][]byte)
}

func (e *Engine) getSourceCode() string {
	v := e.sourceCode.Load()
	if v == nil {
		return ""
	}
	return v.(string)
}

func (e *Engine) getValidFunctions() map[string]bool {
	v := e.validFunctions.Load()
	if v == nil {
		return map[string]bool{}
	}
	return v.(map[string]bool)
}

func (e *Engine) getGlobalVariables() []string {
	v := e.globalVariables.Load()
	if v == nil {
		return []string{}
	}
	return v.([]string)
}

// ==========================================
// 3. Type Conversion Helpers
// ==========================================

func (e *Engine) goToTengo(val interface{}) tengo.Object {
	if val == nil {
		return tengo.UndefinedValue
	}

	switch v := val.(type) {
	case tengo.Object:
		return v
	case bool:
		if v {
			return tengo.TrueValue
		}
		return tengo.FalseValue
	case string:
		return &tengo.String{Value: v}
	case []byte:
		// 遵循 zscript 约定：MsgData 为只读 []byte 视图。
		// 这里保持零拷贝暴露给脚本，调用方需保证底层缓冲不会在脚本执行期间被复用/修改；
		// 脚本侧也必须把 MsgData 当作只读。
		return &tengo.Bytes{Value: v}
	case int:
		return &tengo.Int{Value: int64(v)}
	case int64:
		return &tengo.Int{Value: v}
	case int32:
		return &tengo.Int{Value: int64(v)}
	case uint:
		return &tengo.Int{Value: int64(v)}
	case uint32:
		return &tengo.Int{Value: int64(v)}
	case uint64:
		if v > 9223372036854775807 {
			return &tengo.String{Value: fmt.Sprintf("%d", v)}
		}
		return &tengo.Int{Value: int64(v)}
	case float64:
		return &tengo.Float{Value: v}
	case float32:
		return &tengo.Float{Value: float64(v)}
	case map[string]interface{}:
		m := &tengo.Map{Value: make(map[string]tengo.Object, len(v))}
		for k, val := range v {
			m.Value[k] = e.goToTengo(val)
		}
		return m
	case []interface{}:
		arr := &tengo.Array{Value: make([]tengo.Object, len(v))}
		for i, val := range v {
			arr.Value[i] = e.goToTengo(val)
		}
		return arr
	case *zscript.ScriptContext:
		return &Context{ctx: v, eng: e}
	default:
		return tengo.UndefinedValue
	}
}

func (e *Engine) tengoToGo(val tengo.Object) interface{} {
	if val == nil || val == tengo.UndefinedValue {
		return nil
	}

	switch v := val.(type) {
	case *tengo.Bool:
		return !v.IsFalsy()
	case *tengo.Int:
		return v.Value
	case *tengo.Float:
		return v.Value
	case *tengo.String:
		return v.Value
	case *tengo.Bytes:
		return v.Value
	case *tengo.Array:
		result := make([]interface{}, len(v.Value))
		for i, obj := range v.Value {
			result[i] = e.tengoToGo(obj)
		}
		return result
	case *tengo.Map:
		result := make(map[string]interface{}, len(v.Value))
		for k, obj := range v.Value {
			result[k] = e.tengoToGo(obj)
		}
		return result
	case *tengo.Error:
		return v.String()
	default:
		return v
	}
}
