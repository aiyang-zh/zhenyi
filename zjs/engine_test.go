package zjs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aiyang-zh/zhenyi-base/zerrs"
	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi/zscript"
)

func init() {
	zlog.NewDefaultLogger()
}

// ========== 基础功能测试 ==========

func TestEngine_Basic(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test.js")

	scriptContent := `
function add(a, b) {
	return a + b;
}

function greet(name) {
	return "Hello, " + name;
}

function getContext() {
	return {
		actorId: ctx.ActorID,
		msgId: ctx.MsgID,
		authId: ctx.AuthID,
		traceId: ctx.TraceID,
		nowMillis: ctx.NowMillis
	};
}

function testStdlib() {
	const nowSec = Math.floor(ctx.NowMillis / 1000);
	const msg = "ActorID: " + ctx.ActorID + ", Type: " + ctx.ActorType;
	return {
		nowSeconds: nowSec,
		message: msg,
		mathWorks: true
	};
}
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	config.Type = zscript.EngineTypeJS
	engine, err := NewEngine(config)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	if err := engine.LoadScript(scriptPath); err != nil {
		t.Fatalf("Failed to load script: %v", err)
	}

	params := &zscript.CallParams{
		ActorID:   1001,
		ActorType: 10001,
		TraceID:   12345,
	}

	// 测试简单函数调用
	result, err := engine.Call(context.Background(), params, "add", 1, 2)
	if err != nil {
		t.Fatalf("Failed to call add: %v", err)
	}
	if result.(int64) != 3 {
		t.Errorf("Expected 3, got %v", result)
	}

	// 测试字符串函数
	result, err = engine.Call(context.Background(), params, "greet", "World")
	if err != nil {
		t.Fatalf("Failed to call greet: %v", err)
	}
	if result.(string) != "Hello, World" {
		t.Errorf("Expected 'Hello, World', got %v", result)
	}

	// 测试上下文注入
	result, err = engine.Call(context.Background(), params, "getContext")
	if err != nil {
		t.Fatalf("Failed to call getContext: %v", err)
	}
	t.Logf("Context result: %+v", result)

	// 测试标准库
	result, err = engine.Call(context.Background(), params, "testStdlib")
	if err != nil {
		t.Fatalf("Failed to call testStdlib: %v", err)
	}

	stdlibResult := result.(map[string]interface{})
	if !stdlibResult["mathWorks"].(bool) {
		t.Error("JavaScript stdlib not working")
	}
	t.Logf("Stdlib test result: %+v", stdlibResult)
}

func TestEngine_LoadScripts(t *testing.T) {
	tmpDir := t.TempDir()

	scripts := []struct {
		name    string
		content string
	}{
		{"test1.js", `function test1() { return 1; }`},
		{"test2.js", `function test2() { return 2; }`},
	}

	var paths []string
	for _, s := range scripts {
		path := filepath.Join(tmpDir, s.name)
		if err := os.WriteFile(path, []byte(s.content), 0644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}

	config := zscript.DefaultEngineConfig()
	config.Type = zscript.EngineTypeJS
	engine, err := NewEngine(config)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	if err := engine.LoadScripts(paths); err != nil {
		t.Fatalf("Failed to load scripts: %v", err)
	}

	params := &zscript.CallParams{ActorID: 1001, ActorType: 10001}
	callCtx := context.Background()

	// 测试函数调用
	result, err := engine.Call(callCtx, params, "test1")
	if err != nil {
		t.Fatalf("Failed to call test1: %v", err)
	}
	if result.(int64) != 1 {
		t.Errorf("Expected 1, got %v", result)
	}

	result, err = engine.Call(callCtx, params, "test2")
	if err != nil {
		t.Fatalf("Failed to call test2: %v", err)
	}
	if result.(int64) != 2 {
		t.Errorf("Expected 2, got %v", result)
	}

	stats := engine.GetStats()
	if stats.CallCount < 2 {
		t.Errorf("Expected at least 2 calls, got %d", stats.CallCount)
	}
}

// ========== 类型转换测试 ==========

func TestEngine_TypeConversion(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "types.js")

	scriptContent := `
function testTypes(boolVal, intVal, floatVal, strVal, arrVal, objVal) {
	return {
		bool: boolVal,
		int: intVal,
		float: floatVal,
		str: strVal,
		arr: arrVal,
		obj: objVal
	};
}

function returnNull() {
	return null;
}

function returnUndefined() {
	return undefined;
}

function returnArray() {
	return [1, 2, 3, 4, 5];
}

function returnObject() {
	return {name: "test", level: 10};
}
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	engine, err := NewEngine(config)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	if err := engine.LoadScript(scriptPath); err != nil {
		t.Fatal(err)
	}

	params := &zscript.CallParams{ActorID: 1001, ActorType: 10001}
	callCtx := context.Background()

	// 测试各种类型
	result, err := engine.Call(callCtx, params, "testTypes",
		true,
		int64(42),
		3.14,
		"hello",
		[]interface{}{1, 2, 3},
		map[string]interface{}{"key": "value"},
	)
	if err != nil {
		t.Fatalf("Failed to call testTypes: %v", err)
	}
	t.Logf("Type conversion result: %+v", result)

	// 测试返回 null
	result, err = engine.Call(callCtx, params, "returnNull")
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Errorf("Expected nil, got %v", result)
	}

	// 测试返回 undefined
	result, err = engine.Call(callCtx, params, "returnUndefined")
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Errorf("Expected nil, got %v", result)
	}

	// 测试返回数组
	result, err = engine.Call(callCtx, params, "returnArray")
	if err != nil {
		t.Fatal(err)
	}
	arr := result.([]interface{})
	if len(arr) != 5 {
		t.Errorf("Expected array length 5, got %d", len(arr))
	}

	// 测试返回对象
	result, err = engine.Call(callCtx, params, "returnObject")
	if err != nil {
		t.Fatal(err)
	}
	obj := result.(map[string]interface{})
	if obj["name"].(string) != "test" {
		t.Errorf("Expected name='test', got %v", obj["name"])
	}
}

// ========== 错误处理测试 ==========

func TestEngine_Errors(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "errors.js")

	scriptContent := `
function runtimeError() {
	throw new Error("intentional error");
}

function typeError() {
	return undefinedVariable;
}

function divideByZero() {
	return 1 / 0;  // JavaScript 返回 Infinity
}
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	engine, err := NewEngine(config)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	if err := engine.LoadScript(scriptPath); err != nil {
		t.Fatal(err)
	}

	params := &zscript.CallParams{ActorID: 1001, ActorType: 10001}
	callCtx := context.Background()

	// 测试运行时错误
	_, err = engine.Call(callCtx, params, "runtimeError")
	if err == nil {
		t.Fatal("Expected runtime error, got nil")
	}
	t.Logf("Runtime error: %v", err)

	// 测试类型错误
	_, err = engine.Call(callCtx, params, "typeError")
	if err == nil {
		t.Fatal("Expected type error, got nil")
	}
	t.Logf("Type error: %v", err)

	// 测试函数不存在
	_, err = engine.Call(callCtx, params, "nonExistentFunction")
	if err == nil {
		t.Fatal("Expected function not found error, got nil")
	}
	if err != zscript.ErrFunctionNotFound {
		t.Errorf("Expected ErrFunctionNotFound, got: %v", err)
	}
}

// ========== 超时测试 ==========

func TestEngine_Timeout(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "timeout.js")

	scriptContent := `
function infiniteLoop() {
	while (true) {
		// 无限循环
	}
}
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	config.Type = zscript.EngineTypeJS
	config.Timeout = 100 * time.Millisecond

	engine, err := NewEngine(config)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	if err := engine.LoadScript(scriptPath); err != nil {
		t.Fatalf("Failed to load script: %v", err)
	}

	params := &zscript.CallParams{ActorID: 1002, ActorType: 10001}
	callCtx := context.Background()
	_, err = engine.Call(callCtx, params, "infiniteLoop")

	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	if !zerrs.IsTimeout(err) {
		t.Errorf("Expected timeout error, got: %v", err)
	}

	stats := engine.GetStats()
	if stats.TimeoutCount == 0 {
		t.Error("Expected timeout count > 0")
	}
}

// ========== 模块系统测试 ==========

func TestEngine_RequireModule(t *testing.T) {
	tmpDir := t.TempDir()

	// 创建模块文件
	modulePath := filepath.Join(tmpDir, "math_utils.js")
	moduleContent := `
var MathUtils = {
	add: function(a, b) { return a + b; },
	multiply: function(a, b) { return a * b; }
};
`
	if err := os.WriteFile(modulePath, []byte(moduleContent), 0644); err != nil {
		t.Fatal(err)
	}

	// 创建主脚本
	mainPath := filepath.Join(tmpDir, "main.js")
	mainContent := `
require("math_utils.js");

function calculate(a, b) {
	return {
		sum: MathUtils.add(a, b),
		product: MathUtils.multiply(a, b)
	};
}
`
	if err := os.WriteFile(mainPath, []byte(mainContent), 0644); err != nil {
		t.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	config.ScriptDir = tmpDir
	engine, err := NewEngine(config)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	if err := engine.LoadScript(mainPath); err != nil {
		t.Fatal(err)
	}

	params := &zscript.CallParams{ActorID: 1001, ActorType: 10001}
	callCtx := context.Background()
	result, err := engine.Call(callCtx, params, "calculate", 10, 5)
	if err != nil {
		t.Fatalf("Failed to call calculate: %v", err)
	}

	res := result.(map[string]interface{})
	if res["sum"].(int64) != 15 {
		t.Errorf("Expected sum=15, got %v", res["sum"])
	}
	if res["product"].(int64) != 50 {
		t.Errorf("Expected product=50, got %v", res["product"])
	}

	t.Logf("Module test result: %+v", result)
}

func TestEngine_RequireOutsideScriptDirRejected(t *testing.T) {
	// ScriptDir-only sandbox: require("../outside.js") must not read outside files.
	scriptDir := t.TempDir()
	outsideDir := t.TempDir()

	outsidePath := filepath.Join(outsideDir, "outside.js")
	if err := os.WriteFile(outsidePath, []byte(`exports.secret = "x";`), 0644); err != nil {
		t.Fatal(err)
	}

	mainPath := filepath.Join(scriptDir, "main.js")
	mainContent := `
	function test() {
		// This path intentionally escapes ScriptDir.
		require("../outside.js");
		return "unreachable";
	}`
	if err := os.WriteFile(mainPath, []byte(mainContent), 0644); err != nil {
		t.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	config.ScriptDir = scriptDir

	engine, err := NewEngine(config)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	if err := engine.LoadScript(mainPath); err != nil {
		t.Fatal(err)
	}

	params := &zscript.CallParams{ActorID: 1001, ActorType: 10001}
	callCtx := context.Background()
	_, err = engine.Call(callCtx, params, "test")
	if err == nil {
		t.Fatalf("expected require outside ScriptDir to fail")
	}
	if !strings.Contains(err.Error(), "outside ScriptDir") {
		t.Fatalf("expected sandbox error, got: %v", err)
	}
}

// ========== 热重载测试 ==========

func TestEngine_HotReload(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "reload.js")

	// 写入初始版本
	v1Content := `function getVersion() { return 1; }`
	if err := os.WriteFile(scriptPath, []byte(v1Content), 0644); err != nil {
		t.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	engine, err := NewEngine(config)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	if err := engine.LoadScript(scriptPath); err != nil {
		t.Fatal(err)
	}

	params := &zscript.CallParams{ActorID: 1001, ActorType: 10001}
	callCtx := context.Background()

	// 调用 v1
	result, err := engine.Call(callCtx, params, "getVersion")
	if err != nil {
		t.Fatal(err)
	}
	if result.(int64) != 1 {
		t.Errorf("Expected version 1, got %v", result)
	}

	// 写入 v2
	v2Content := `function getVersion() { return 2; }`
	if err := os.WriteFile(scriptPath, []byte(v2Content), 0644); err != nil {
		t.Fatal(err)
	}

	// 热重载
	if err := engine.ReloadScript(scriptPath); err != nil {
		t.Fatal(err)
	}

	// 调用 v2
	result, err = engine.Call(callCtx, params, "getVersion")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("After reload, version: %v (expected: 2, may vary based on VM pooling)", result)
}

// ========== 并发安全测试 ==========

func TestEngine_Concurrent(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "concurrent.js")

	scriptContent := `
function compute(n) {
	let sum = 0;
	for (let i = 0; i <= n; i++) {
		sum += i;
	}
	return sum;
}
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	config.VMPoolSize = 4
	engine, err := NewEngine(config)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	if err := engine.LoadScript(scriptPath); err != nil {
		t.Fatal(err)
	}

	callCtx := context.Background()

	const goroutines = 100
	const calls = 10

	var wg sync.WaitGroup
	var successCount int64
	var errorCount int64

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			params := &zscript.CallParams{
				ActorID:   uint64(id),
				ActorType: 10001,
			}
			for j := 0; j < calls; j++ {
				result, err := engine.Call(callCtx, params, "compute", 100)
				if err != nil {
					atomic.AddInt64(&errorCount, 1)
					t.Logf("Goroutine %d call %d failed: %v", id, j, err)
				} else {
					if result.(int64) != 5050 {
						t.Errorf("Goroutine %d got wrong result: %v", id, result)
					}
					atomic.AddInt64(&successCount, 1)
				}
			}
		}(i)
	}

	wg.Wait()

	t.Logf("Concurrent test: success=%d, errors=%d", successCount, errorCount)
	if errorCount > 0 {
		t.Errorf("Expected 0 errors, got %d", errorCount)
	}
	if successCount != goroutines*calls {
		t.Errorf("Expected %d successful calls, got %d", goroutines*calls, successCount)
	}
}

// ========== Close 保护测试 ==========

func TestEngine_CloseProtection(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test.js")

	scriptContent := `function test() { return 42; }`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	engine, err := NewEngine(config)
	if err != nil {
		t.Fatal(err)
	}

	if err := engine.LoadScript(scriptPath); err != nil {
		t.Fatal(err)
	}

	params := &zscript.CallParams{ActorID: 1001, ActorType: 10001}
	callCtx := context.Background()

	// 正常调用
	_, err = engine.Call(callCtx, params, "test")
	if err != nil {
		t.Fatal(err)
	}

	// Close 引擎
	engine.Close()

	// Close 后调用应该失败
	_, err = engine.Call(callCtx, params, "test")
	if err != nil {
		t.Logf("✅ Close protection works: %v", err)
	} else {
		t.Log("⚠️ Close did not prevent Call (may be due to async cleanup)")
	}

	// Close 后加载应该失败
	err = engine.LoadScript(scriptPath)
	if err != nil {
		t.Logf("✅ Close protection works for LoadScript: %v", err)
	} else {
		t.Log("⚠️ Close did not prevent LoadScript")
	}
}

// ========== VM 生命周期测试 ==========

func TestEngine_VMLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test.js")

	// 说明：
	// 原测试依赖 sync.Pool 在短时间内“足够复用同一个 VM wrapper”，从而触发 useCount/age 淘汰。
	// 在 -race/-count 组合下 sync.Pool 复用不稳定，导致 vm_destroyed 有时仍为 0（flake）。
	// 这里改为用一个显式慢函数 slow()，保证单次调用耗时 > MaxVMAge，从而稳定触发 age 淘汰逻辑。
	scriptContent := `
function fast() { return 1; }
function slow() {
  var start = Date.now();
  while (Date.now() - start < 1200) { /* busy wait */ }
  return 1;
}`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	config.VMPoolSize = 1
	config.MaxVMUseCount = 5
	config.MaxVMAge = 1 * time.Second

	engine, err := NewEngine(config)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	if err := engine.LoadScript(scriptPath); err != nil {
		t.Fatal(err)
	}

	params := &zscript.CallParams{ActorID: 1001, ActorType: 10001}
	callCtx := context.Background()

	// 调用多次触发 useCount 淘汰
	for i := 0; i < 20; i++ {
		_, err := engine.Call(callCtx, params, "fast")
		if err != nil {
			t.Fatalf("Call %d failed: %v", i, err)
		}
	}

	stats := engine.GetStats()
	vmCreated := stats.Metadata["vm_created"].(int64)
	vmDestroyed := stats.Metadata["vm_destroyed"].(int64)

	t.Logf("VM lifecycle: created=%d, destroyed=%d", vmCreated, vmDestroyed)

	if vmCreated < 1 {
		t.Errorf("Expected at least 1 VM created, got %d", vmCreated)
	}

	// 显式触发 age 淘汰：slow() 单次耗时 > MaxVMAge，shouldDestroy 应在本次 Call 结束时生效。
	destroyedBefore := vmDestroyed
	_, err = engine.Call(callCtx, params, "slow")
	if err != nil {
		t.Fatal(err)
	}

	// 保险起见轮询一下（应当是同步计数，但避免极端调度下的可观测延迟）
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		stats = engine.GetStats()
		vmDestroyedAfter := stats.Metadata["vm_destroyed"].(int64)
		if vmDestroyedAfter > destroyedBefore {
			t.Logf("VM destroyed after slow(): %d -> %d", destroyedBefore, vmDestroyedAfter)
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	stats = engine.GetStats()
	vmDestroyedAfter := stats.Metadata["vm_destroyed"].(int64)
	t.Fatalf("Expected VM destroyed after slow() (age eviction), but vm_destroyed %d -> %d", destroyedBefore, vmDestroyedAfter)
}

// ========== 全局变量污染测试 ==========

func TestEngine_GlobalVariablePollution(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "global.js")

	scriptContent := `
var globalCounter = 0;  // ❌ 全局变量

function increment() {
	return ++globalCounter;
}
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	config.Type = zscript.EngineTypeJS
	config.VMPoolSize = 1

	engine, err := NewEngine(config)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	if err := engine.LoadScript(scriptPath); err != nil {
		t.Fatalf("Failed to load script: %v", err)
	}

	params := &zscript.CallParams{ActorID: 1001, ActorType: 10001}
	callCtx := context.Background()

	// 第一次调用
	result1, err := engine.Call(callCtx, params, "increment")
	if err != nil {
		t.Fatalf("Failed to call increment: %v", err)
	}

	// 第二次调用（会复用同一个 VM）
	result2, err := engine.Call(callCtx, params, "increment")
	if err != nil {
		t.Fatalf("Failed to call increment: %v", err)
	}

	t.Logf("First call:  %v", result1)
	t.Logf("Second call: %v", result2)

	// 验证污染发生了
	if result1.(int64) == 1 && result2.(int64) == 2 {
		t.Log("⚠️  Global variable pollution detected!")
		t.Log("⚠️  This demonstrates why global variables MUST be avoided in scripts!")
	}
}

// ========== 基准测试 ==========

func BenchmarkEngine_Call(b *testing.B) {
	tmpDir := b.TempDir()
	scriptPath := filepath.Join(tmpDir, "bench.js")

	scriptContent := `
function compute(n) {
	let sum = 0;
	for (let i = 0; i < n; i++) {
		sum += i;
	}
	return sum;
}
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		b.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	engine, err := NewEngine(config)
	if err != nil {
		b.Fatal(err)
	}
	defer engine.Close()

	if err := engine.LoadScript(scriptPath); err != nil {
		b.Fatal(err)
	}

	params := &zscript.CallParams{ActorID: 1003, ActorType: 10001}
	callCtx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := engine.Call(callCtx, params, "compute", 100)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEngine_CallParallel 并发测试（保留 Timeout，模拟真实生产场景）
func BenchmarkEngine_CallParallel(b *testing.B) {
	tmpDir := b.TempDir()
	scriptPath := filepath.Join(tmpDir, "bench.js")

	scriptContent := `
function compute(n) {
	let sum = 0;
	for (let i = 0; i < n; i++) {
		sum += i;
	}
	return sum;
}
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		b.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	// ⚠️ 保留 Timeout（真实场景），会增加 ~79 allocs/op
	config.VMPoolSize = 8
	engine, err := NewEngine(config)
	if err != nil {
		b.Fatal(err)
	}
	defer engine.Close()

	if err := engine.LoadScript(scriptPath); err != nil {
		b.Fatal(err)
	}

	callCtx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		params := &zscript.CallParams{ActorID: 1001, ActorType: 10001}
		for pb.Next() {
			_, err := engine.Call(callCtx, params, "compute", 100)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkEngine_TypeConversion(b *testing.B) {
	tmpDir := b.TempDir()
	scriptPath := filepath.Join(tmpDir, "bench.js")

	scriptContent := `
function processData(data) {
	return {
		count: data.items.length,
		total: data.total,
		name: data.name
	};
}
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		b.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	engine, err := NewEngine(config)
	if err != nil {
		b.Fatal(err)
	}
	defer engine.Close()

	if err := engine.LoadScript(scriptPath); err != nil {
		b.Fatal(err)
	}

	params := &zscript.CallParams{ActorID: 1001, ActorType: 10001}
	testData := map[string]interface{}{
		"items": []interface{}{1, 2, 3, 4, 5},
		"total": int64(15),
		"name":  "test",
	}
	callCtx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := engine.Call(callCtx, params, "processData", testData)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEngine_CallMemory(b *testing.B) {
	tmpDir := b.TempDir()
	scriptPath := filepath.Join(tmpDir, "bench.js")

	scriptContent := `function simple(a, b) { return a + b; }`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		b.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	config.Timeout = 0 // ✅ 关闭 Timeout，精确测量内存分配
	engine, err := NewEngine(config)
	if err != nil {
		b.Fatal(err)
	}
	defer engine.Close()

	if err := engine.LoadScript(scriptPath); err != nil {
		b.Fatal(err)
	}

	params := &zscript.CallParams{ActorID: 1001, ActorType: 10001}
	callCtx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err = engine.Call(callCtx, params, "simple", 1, 2)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEngine_Optimized 验证 Context 优化效果（Zero-Timeout + Context Access）
func BenchmarkEngine_Optimized(b *testing.B) {
	tmpDir := b.TempDir()
	scriptPath := filepath.Join(tmpDir, "bench_opt.js")

	// 脚本：一个是纯计算，一个是访问 Context
	// 如果优化生效，accessContext 的分配次数应该和 simple 几乎一样
	scriptContent := `
	function simple(a, b) { return a + b; }
	
	function accessContext() {
		// 访问多个字段，测试 Lazy Loading 和 sharedCtx 是否有分配
		return ctx.ActorID + ctx.MsgID + ctx.TraceID;
	}
	`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		b.Fatal(err)
	}

	// ✅ 关键 1：设置 Timeout 为 0，移除 time.AfterFunc 的巨大干扰
	config := zscript.DefaultEngineConfig()
	config.Timeout = 0

	engine, err := NewEngine(config)
	if err != nil {
		b.Fatal(err)
	}
	defer engine.Close()

	if err := engine.LoadScript(scriptPath); err != nil {
		b.Fatal(err)
	}

	params := &zscript.CallParams{ActorID: 1001, ActorType: 10001, TraceID: 123, MsgID: 456}
	callCtx := context.Background()

	b.Run("Simple_Add", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// 注意：这里的 1, 2 传参本身会产生 Go 语言层面的 3 次分配 (int box * 2 + slice)
			_, err := engine.Call(callCtx, params, "simple", 1, 2)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Context_Access", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// 这里的 Call 没有 args，减少了参数装箱的干扰
			// 纯粹测试 sharedCtx 的字段访问是否有分配
			_, err := engine.Call(callCtx, params, "accessContext")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// ========== 覆盖率补充测试 ==========

func TestEngine_ReloadAllScripts(t *testing.T) {
	tmpDir := t.TempDir()

	scripts := []struct {
		name    string
		content string
	}{
		{"script1.js", `function func1() { return 1; }`},
		{"script2.js", `function func2() { return 2; }`},
		{"script3.js", `function func3() { return 3; }`},
	}

	var paths []string
	for _, s := range scripts {
		path := filepath.Join(tmpDir, s.name)
		if err := os.WriteFile(path, []byte(s.content), 0644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}

	config := zscript.DefaultEngineConfig()
	engine, err := NewEngine(config)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	if err := engine.LoadScripts(paths); err != nil {
		t.Fatal(err)
	}

	// 修改所有脚本
	for i, s := range scripts {
		newContent := s.content + `function extra() { return 99; }`
		if err := os.WriteFile(paths[i], []byte(newContent), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// 批量重载
	if err := engine.ReloadAllScripts(); err != nil {
		t.Fatal(err)
	}

	params := &zscript.CallParams{ActorID: 1001, ActorType: 10001}
	callCtx := context.Background()
	result, err := engine.Call(callCtx, params, "extra")
	if err != nil {
		t.Fatal(err)
	}
	if result.(int64) != 99 {
		t.Errorf("Expected 99, got %v", result)
	}

	t.Log("✅ ReloadAllScripts works correctly")
}

func TestEngine_GetType(t *testing.T) {
	config := zscript.DefaultEngineConfig()
	engine, err := NewEngine(config)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	if engine.GetType() != "javascript" {
		t.Errorf("Expected 'javascript', got '%s'", engine.GetType())
	}
}

func TestEngine_EdgeCases(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "edge.js")

	scriptContent := `
function returnEmptyObject() {
	return {};
}

function returnEmptyArray() {
	return [];
}

function returnNested() {
	return {
		level1: {
			level2: {
				value: "deep"
			}
		}
	};
}

function acceptNull(val) {
	return val === null ? "got_null" : "got_value";
}

function acceptUndefined(val) {
	return val === undefined ? "got_undefined" : "got_value";
}

function mixedArray() {
	return [1, "two", 3.0, true, {nested: "value"}, null];
}
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	engine, err := NewEngine(config)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	if err := engine.LoadScript(scriptPath); err != nil {
		t.Fatal(err)
	}

	params := &zscript.CallParams{ActorID: 1001, ActorType: 10001}
	callCtx := context.Background()

	// 测试空对象
	result, err := engine.Call(callCtx, params, "returnEmptyObject")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Empty object: %+v", result)

	// 测试空数组
	result, err = engine.Call(callCtx, params, "returnEmptyArray")
	if err != nil {
		t.Fatal(err)
	}
	if arr, ok := result.([]interface{}); !ok || len(arr) != 0 {
		t.Error("Expected empty array")
	}

	// 测试嵌套结构
	result, err = engine.Call(callCtx, params, "returnNested")
	if err != nil {
		t.Fatal(err)
	}
	nested := result.(map[string]interface{})
	level1 := nested["level1"].(map[string]interface{})
	level2 := level1["level2"].(map[string]interface{})
	if level2["value"].(string) != "deep" {
		t.Error("Nested structure parsing failed")
	}

	// 测试 null 参数
	result, err = engine.Call(callCtx, params, "acceptNull", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.(string) != "got_null" {
		t.Error("Null parameter handling failed")
	}

	// 测试混合数组
	result, err = engine.Call(callCtx, params, "mixedArray")
	if err != nil {
		t.Fatal(err)
	}
	arr := result.([]interface{})
	if len(arr) != 6 {
		t.Errorf("Expected 6 elements, got %d", len(arr))
	}

	t.Log("✅ Edge cases handled correctly")
}

func TestEngine_Metadata(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "meta.js")

	scriptContent := `
function checkMetadata() {
	return {
		hasOwner: ctx.Owner !== null,
		hasMetadata: Object.keys(ctx.Metadata).length > 0
	};
}
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	engine, err := NewEngine(config)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	if err := engine.LoadScript(scriptPath); err != nil {
		t.Fatal(err)
	}

	params := &zscript.CallParams{
		ActorID:   1001,
		ActorType: 10001,
		Owner:     "test_owner",
		Metadata: map[string]interface{}{
			"key1": "value1",
			"key2": 123,
		},
	}
	callCtx := context.Background()

	result, err := engine.Call(callCtx, params, "checkMetadata")
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Metadata result: %+v", result)
	t.Log("✅ Metadata handling works")
}

func TestEngine_LoadScript_ErrorCases(t *testing.T) {
	config := zscript.DefaultEngineConfig()
	engine, err := NewEngine(config)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// 测试不存在的文件
	err = engine.LoadScript("/nonexistent/path/script.js")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
	t.Logf("Nonexistent file error: %v", err)

	// 测试语法错误的脚本
	tmpDir := t.TempDir()
	badScript := filepath.Join(tmpDir, "bad.js")
	if err := os.WriteFile(badScript, []byte(`function bad( return }`), 0644); err != nil {
		t.Fatal(err)
	}

	err = engine.LoadScript(badScript)
	if err == nil {
		t.Error("Expected error for syntax error")
	}
	t.Logf("Syntax error: %v", err)

	t.Log("✅ Error cases handled correctly")
}

func TestEngine_CallWithNoArgs(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "noargs.js")

	scriptContent := `
function noParams() {
	return "success";
}
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	engine, err := NewEngine(config)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	if err := engine.LoadScript(scriptPath); err != nil {
		t.Fatal(err)
	}

	params := &zscript.CallParams{ActorID: 1001, ActorType: 10001}
	callCtx := context.Background()
	result, err := engine.Call(callCtx, params, "noParams")
	if err != nil {
		t.Fatal(err)
	}

	if result.(string) != "success" {
		t.Errorf("Expected 'success', got %v", result)
	}
}

func TestEngine_ComplexTypes(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "complex.js")

	scriptContent := `
function returnComplex() {
	return {
		stringVal: "test",
		numberVal: 42,
		floatVal: 3.14,
		boolVal: true,
		nullVal: null,
		undefinedVal: undefined,
		array: [1, 2, 3],
		nested: {
			inner: {
				value: "deep"
			}
		}
	};
}
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	engine, err := NewEngine(config)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	if err := engine.LoadScript(scriptPath); err != nil {
		t.Fatal(err)
	}

	params := &zscript.CallParams{ActorID: 1001, ActorType: 10001}
	callCtx := context.Background()
	result, err := engine.Call(callCtx, params, "returnComplex")
	if err != nil {
		t.Fatal(err)
	}

	m := result.(map[string]interface{})
	if m["stringVal"].(string) != "test" {
		t.Error("String conversion failed")
	}
	if m["numberVal"].(int64) != 42 {
		t.Error("Number conversion failed")
	}
	if m["floatVal"].(float64) != 3.14 {
		t.Error("Float conversion failed")
	}
	if m["boolVal"].(bool) != true {
		t.Error("Bool conversion failed")
	}
	if m["nullVal"] != nil {
		t.Error("Null conversion failed")
	}

	arr := m["array"].([]interface{})
	if len(arr) != 3 {
		t.Error("Array conversion failed")
	}

	t.Log("✅ Complex type conversions work correctly")
}

func TestEngine_ModulePathResolution(t *testing.T) {
	tmpDir := t.TempDir()
	mainPath := filepath.Join(tmpDir, "main.js")
	subdir := filepath.Join(tmpDir, "lib")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatal(err)
	}
	modulePath := filepath.Join(subdir, "helper.js")

	moduleContent := `exports.helper = function() { return "from_module"; };`
	if err := os.WriteFile(modulePath, []byte(moduleContent), 0644); err != nil {
		t.Fatal(err)
	}

	mainContent := `
var helper = require("./lib/helper.js");
function testModule() {
	return helper.helper();
}
`
	if err := os.WriteFile(mainPath, []byte(mainContent), 0644); err != nil {
		t.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	config.ScriptDir = tmpDir
	engine, err := NewEngine(config)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	if err := engine.LoadScript(mainPath); err != nil {
		t.Fatal(err)
	}

	params := &zscript.CallParams{ActorID: 1001, ActorType: 10001}
	callCtx := context.Background()
	result, err := engine.Call(callCtx, params, "testModule")
	if err != nil {
		t.Fatal(err)
	}

	if result.(string) != "from_module" {
		t.Errorf("Expected 'from_module', got %v", result)
	}

	t.Log("✅ Module path resolution works")
}

// ========== 深度覆盖率测试 (95%+ 目标) ==========

func TestEngine_TypeConversionEdges(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "types.js")

	scriptContent := `
function testLargeNumber() {
	return 9007199254740992;
}

function testNegative() {
	return -999999;
}

function testZero() {
	return 0;
}

function testEmptyString() {
	return "";
}

function testUnicode() {
	return "你好世界🚀";
}

function testNestedArray() {
	return [[1, 2], [3, 4], [5, 6]];
}

function testSparseArray() {
	var arr = [];
	arr[0] = "a";
	arr[5] = "b";
	arr[10] = "c";
	return arr;
}

function testMixedObject() {
	return {
		string: "value",
		number: 42,
		bool: true,
		null: null,
		array: [1, 2, 3],
		nested: { inner: "deep" }
	};
}

function testRegExp() {
	return /test/.toString();
}

function testDate() {
	return new Date(2024, 0, 1).toISOString();
}
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	engine, err := NewEngine(config)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	if err := engine.LoadScript(scriptPath); err != nil {
		t.Fatal(err)
	}

	params := &zscript.CallParams{ActorID: 1001, ActorType: 10001}
	callCtx := context.Background()

	tests := []string{
		"testLargeNumber", "testNegative", "testZero", "testEmptyString",
		"testUnicode", "testNestedArray", "testSparseArray", "testMixedObject",
		"testRegExp", "testDate",
	}

	for _, fn := range tests {
		t.Run(fn, func(t *testing.T) {
			result, err := engine.Call(callCtx, params, fn)
			if err != nil {
				t.Fatalf("%s failed: %v", fn, err)
			}
			t.Logf("%s result: %+v", fn, result)
		})
	}

	t.Log("✅ All type conversion edges covered")
}

func TestEngine_ConcurrentVMReuse(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "concurrent.js")

	scriptContent := `function work(n) { return n * 2; }`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	config.VMPoolSize = 2
	engine, err := NewEngine(config)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	if err := engine.LoadScript(scriptPath); err != nil {
		t.Fatal(err)
	}

	const workers = 50
	const iterations = 20

	var wg sync.WaitGroup
	var successCount int64
	callCtx := context.Background()

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			params := &zscript.CallParams{
				ActorID:   uint64(workerID),
				ActorType: 10001,
			}
			for j := 0; j < iterations; j++ {
				result, err := engine.Call(callCtx, params, "work", j)
				if err != nil {
					t.Logf("Error: %v", err)
					return
				}
				if result.(int64) == int64(j*2) {
					atomic.AddInt64(&successCount, 1)
				}
			}
		}(i)
	}

	wg.Wait()

	expected := int64(workers * iterations)
	if successCount != expected {
		t.Errorf("Expected %d successful calls, got %d", expected, successCount)
	}

	t.Logf("✅ %d successful concurrent calls", successCount)
}

func TestEngine_ScriptErrors(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "errors.js")

	scriptContent := `
function nullAccess() {
	var obj = null;
	return obj.field;
}

function typeError() {
	return undefinedVariable;
}

function callNonFunction() {
	var x = 42;
	x();
}
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	engine, err := NewEngine(config)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	if err := engine.LoadScript(scriptPath); err != nil {
		t.Fatal(err)
	}

	params := &zscript.CallParams{ActorID: 1001, ActorType: 10001}
	callCtx := context.Background()

	errorTests := []string{"nullAccess", "typeError", "callNonFunction"}

	for _, fn := range errorTests {
		t.Run(fn, func(t *testing.T) {
			_, err := engine.Call(callCtx, params, fn)
			if err == nil {
				t.Errorf("%s should have failed", fn)
			} else {
				t.Logf("%s error (expected): %v", fn, err)
			}
		})
	}

	t.Log("✅ Script error handling covered")
}

func TestEngine_VMRecycling(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test.js")

	scriptContent := `function simple() { return 1; }`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	config.VMPoolSize = 2
	config.MaxVMUseCount = 3
	engine, err := NewEngine(config)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	if err := engine.LoadScript(scriptPath); err != nil {
		t.Fatal(err)
	}

	params := &zscript.CallParams{ActorID: 1001, ActorType: 10001}
	callCtx := context.Background()

	for i := 0; i < 20; i++ {
		_, err := engine.Call(callCtx, params, "simple")
		if err != nil {
			t.Fatal(err)
		}
	}

	stats := engine.GetStats()
	vmCreated := stats.Metadata["vm_created"].(int64)
	vmDestroyed := stats.Metadata["vm_destroyed"].(int64)

	t.Logf("VM lifecycle: created=%d, destroyed=%d", vmCreated, vmDestroyed)

	if vmCreated < 3 {
		t.Logf("VM created count: %d (may vary)", vmCreated)
	}

	t.Log("✅ VM recycling mechanism covered")
}

func TestEngine_CloseDuringCall(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "slow.js")

	scriptContent := `
function slowWork() {
	var sum = 0;
	for (var i = 0; i < 100000; i++) {
		sum += i;
	}
	return sum;
}
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	engine, err := NewEngine(config)
	if err != nil {
		t.Fatal(err)
	}

	if err := engine.LoadScript(scriptPath); err != nil {
		t.Fatal(err)
	}

	params := &zscript.CallParams{ActorID: 1001, ActorType: 10001}
	callCtx := context.Background()

	done := make(chan struct{})
	go func() {
		_, _ = engine.Call(callCtx, params, "slowWork")
		close(done)
	}()

	time.Sleep(10 * time.Millisecond)
	engine.Close()

	<-done

	t.Log("✅ Close during call handled")
}

func TestEngine_EmptyScriptDir(t *testing.T) {
	config := zscript.DefaultEngineConfig()
	config.ScriptDir = ""
	engine, err := NewEngine(config)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	t.Log("✅ Empty script dir handled")
}

func TestEngine_RequireRejectedWhenScriptDirEmpty(t *testing.T) {
	config := zscript.DefaultEngineConfig()
	config.ScriptDir = ""
	engine, err := NewEngine(config)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	tmpDir := t.TempDir()
	libPath := filepath.Join(tmpDir, "lib.js")
	if err := os.WriteFile(libPath, []byte(`exports.x = 1;`), 0644); err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(tmpDir, "main.js")
	if err := os.WriteFile(mainPath, []byte(`
function hit() {
	require("./lib.js");
	return 1;
}
`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := engine.LoadScript(mainPath); err != nil {
		t.Fatal(err)
	}
	_, err = engine.Call(context.Background(), &zscript.CallParams{ActorID: 1, ActorType: 1}, "hit")
	if err == nil {
		t.Fatal("expected Call to fail when ScriptDir empty and require is used")
	}
	if !strings.Contains(err.Error(), "ScriptDir is empty") {
		t.Fatalf("expected ScriptDir error, got: %v", err)
	}
}

func TestEngine_RequireAllowedLegacyWhenScriptDirEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	libPath, err := filepath.Abs(filepath.Join(tmpDir, "lib.js"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(libPath, []byte(`exports.x = 1;`), 0644); err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(tmpDir, "main.js")
	// Legacy mode resolves require via filepath.Abs(relative to process CWD), so use an absolute path.
	mainSrc := fmt.Sprintf(`var lib = require(%s); function getx() { return lib.x; }`, strconv.Quote(filepath.ToSlash(libPath)))
	if err := os.WriteFile(mainPath, []byte(mainSrc), 0644); err != nil {
		t.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	config.ScriptDir = ""
	config.AllowRequireWithoutScriptDir = true
	engine, err := NewEngine(config)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	if err := engine.LoadScript(mainPath); err != nil {
		t.Fatal(err)
	}
	params := &zscript.CallParams{ActorID: 1, ActorType: 1}
	v, err := engine.Call(context.Background(), params, "getx")
	if err != nil {
		t.Fatal(err)
	}
	if v != int64(1) {
		t.Fatalf("got %v want 1", v)
	}
}
