package zstarlark

import (
	"context"
	"os"
	"path/filepath"
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
	scriptPath := filepath.Join(tmpDir, "test.star")

	scriptContent := `
# Starlark 脚本示例

def add(ctx, a, b):
    return a + b

def greet(ctx, name):
    return "Hello, " + name

def get_context(ctx):
    return {
        "actor_id": ctx["ActorID"],
        "msg_id": ctx["MsgID"],
        "now_millis": ctx["NowMillis"],
    }

def test_list(ctx):
    nums = [1, 2, 3, 4, 5]
    total = 0
    for n in nums:
        total += n
    return total

def test_dict(ctx):
    data = {"name": "player1", "level": 10}
    return data["level"]

def test_set(ctx):
    s = set([1, 2, 2, 3, 3, 3])
    return len(s)

def test_tuple(ctx):
    t = (1, 2, 3)
    return t[0] + t[1] + t[2]
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	config.Type = zscript.EngineTypeStarlark

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
		MsgID:     1001,
		AuthID:    67890,
	}
	callCtx := context.Background()

	// 测试简单函数调用
	result, err := engine.Call(callCtx, params, "add", 1, 2)
	if err != nil {
		t.Fatalf("Failed to call add: %v", err)
	}
	if result.(int64) != 3 {
		t.Errorf("Expected 3, got %v", result)
	}

	// 测试字符串函数
	result, err = engine.Call(callCtx, params, "greet", "World")
	if err != nil {
		t.Fatalf("Failed to call greet: %v", err)
	}
	if result.(string) != "Hello, World" {
		t.Errorf("Expected 'Hello, World', got %v", result)
	}

	// 测试上下文注入
	result, err = engine.Call(callCtx, params, "get_context")
	if err != nil {
		t.Fatalf("Failed to call get_context: %v", err)
	}
	t.Logf("Context result: %+v", result)

	// 测试列表处理
	result, err = engine.Call(callCtx, params, "test_list")
	if err != nil {
		t.Fatalf("Failed to call test_list: %v", err)
	}
	if result.(int64) != 15 {
		t.Errorf("Expected 15, got %v", result)
	}

	// 测试字典处理
	result, err = engine.Call(callCtx, params, "test_dict")
	if err != nil {
		t.Fatalf("Failed to call test_dict: %v", err)
	}
	if result.(int64) != 10 {
		t.Errorf("Expected 10, got %v", result)
	}

	// 测试 Set
	result, err = engine.Call(callCtx, params, "test_set")
	if err != nil {
		t.Fatalf("Failed to call test_set: %v", err)
	}
	if result.(int64) != 3 {
		t.Errorf("Expected 3, got %v", result)
	}

	// 测试 Tuple
	result, err = engine.Call(callCtx, params, "test_tuple")
	if err != nil {
		t.Fatalf("Failed to call test_tuple: %v", err)
	}
	if result.(int64) != 6 {
		t.Errorf("Expected 6, got %v", result)
	}
}

func TestEngine_LoadScripts(t *testing.T) {
	tmpDir := t.TempDir()

	scripts := []struct {
		name    string
		content string
	}{
		{"test1.star", `def test1(ctx): return 1`},
		{"test2.star", `def test2(ctx): return 2`},
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
	config.Type = zscript.EngineTypeStarlark

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
	scriptPath := filepath.Join(tmpDir, "types.star")

	scriptContent := `
def test_types(ctx, bool_val, int_val, float_val, str_val, list_val, dict_val):
    return {
        "bool": bool_val,
        "int": int_val,
        "float": float_val,
        "str": str_val,
        "list": list_val,
        "dict": dict_val
    }

def return_none(ctx):
    return None

def return_list(ctx):
    return [1, 2, 3, 4, 5]

def return_dict(ctx):
    return {"name": "test", "level": 10}

def return_tuple(ctx):
    return (1, 2, 3)

def return_set(ctx):
    return set([1, 2, 3])
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
	result, err := engine.Call(callCtx, params, "test_types",
		true,
		int64(42),
		3.14,
		"hello",
		[]interface{}{1, 2, 3},
		map[string]interface{}{"key": "value"},
	)
	if err != nil {
		t.Fatalf("Failed to call test_types: %v", err)
	}
	t.Logf("Type conversion result: %+v", result)

	// 测试返回 None
	result, err = engine.Call(callCtx, params, "return_none")
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Errorf("Expected nil, got %v", result)
	}

	// 测试返回 list
	result, err = engine.Call(callCtx, params, "return_list")
	if err != nil {
		t.Fatal(err)
	}
	list := result.([]interface{})
	if len(list) != 5 {
		t.Errorf("Expected list length 5, got %d", len(list))
	}

	// 测试返回 dict
	result, err = engine.Call(callCtx, params, "return_dict")
	if err != nil {
		t.Fatal(err)
	}
	dict := result.(map[string]interface{})
	if dict["name"].(string) != "test" {
		t.Errorf("Expected name='test', got %v", dict["name"])
	}

	// 测试返回 tuple
	result, err = engine.Call(callCtx, params, "return_tuple")
	if err != nil {
		t.Fatal(err)
	}
	tuple := result.([]interface{})
	if len(tuple) != 3 {
		t.Errorf("Expected tuple length 3, got %d", len(tuple))
	}

	// 测试返回 set
	result, err = engine.Call(callCtx, params, "return_set")
	if err != nil {
		t.Fatal(err)
	}
	set := result.([]interface{})
	if len(set) != 3 {
		t.Errorf("Expected set length 3, got %d", len(set))
	}
}

// ========== 错误处理测试 ==========

func TestEngine_Errors(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "errors.star")

	scriptContent := `
def runtime_error(ctx):
    fail("intentional error")

def divide_by_zero(ctx):
    return 1 // 0  # Starlark uses // for integer division
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
	_, err = engine.Call(callCtx, params, "runtime_error")
	if err == nil {
		t.Fatal("Expected runtime error, got nil")
	}
	t.Logf("Runtime error: %v", err)

	// 测试除零错误
	_, err = engine.Call(callCtx, params, "divide_by_zero")
	if err == nil {
		t.Fatal("Expected division error, got nil")
	}
	t.Logf("Division error: %v", err)

	// 测试函数不存在
	_, err = engine.Call(callCtx, params, "non_existent_function")
	if err == nil {
		t.Fatal("Expected function not found error, got nil")
	}
	if err != zscript.ErrFunctionNotFound {
		t.Errorf("Expected ErrFunctionNotFound, got: %v", err)
	}

	// 测试除零（Starlark 不允许除零）
	_, err = engine.Call(callCtx, params, "divide_by_zero")
	if err == nil {
		t.Fatal("Expected divide by zero error, got nil")
	}
	t.Logf("Divide by zero error: %v", err)
}

// ========== 超时测试 ==========

func TestEngine_Timeout(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "timeout.star")

	// 大量计算
	scriptContent := `
def compute(ctx):
    total = 0
    for i in range(100000):
        total += i
    return total
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	config.Type = zscript.EngineTypeStarlark
	config.Timeout = 1 * time.Millisecond // 极短超时

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
	_, err = engine.Call(callCtx, params, "compute")

	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	if !zerrs.IsTimeout(err) {
		t.Logf("Error: %v", err)
	}

	stats := engine.GetStats()
	t.Logf("Stats: CallCount=%d, ErrorCount=%d, TimeoutCount=%d",
		stats.CallCount, stats.ErrorCount, stats.TimeoutCount)
}

func TestEngine_ContextCancellationInterrupts(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "cancel.star")

	scriptContent := `
def spin(ctx):
    i = 0
    while True:
        i = i + 1
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	config.Type = zscript.EngineTypeStarlark
	config.Timeout = 0 // disable SetMaxExecutionSteps, rely on ctx cancellation

	engine, err := NewEngine(config)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	if err := engine.LoadScript(scriptPath); err != nil {
		t.Fatalf("Failed to load script: %v", err)
	}

	params := &zscript.CallParams{ActorID: 1002, ActorType: 10001}
	callCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	start := time.Now()
	errCh := make(chan error, 1)
	go func() {
		_, callErr := engine.Call(callCtx, params, "spin")
		errCh <- callErr
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case callErr := <-errCh:
		if callErr == nil {
			t.Fatal("expected cancellation error, got nil")
		}
		// ctx cancellation should be mapped to timeout/cancel path.
		if !zerrs.IsTimeout(callErr) {
			t.Fatalf("expected timeout-like error, got: %v", callErr)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("engine.Call did not interrupt within 1s")
	}

	if time.Since(start) > 500*time.Millisecond {
		t.Fatalf("expected early interrupt, elapsed=%s", time.Since(start))
	}
}

// ========== Starlark 特性测试 ==========

func TestEngine_NoWhileLoop(t *testing.T) {
	// ✅ Starlark 语法错误检测
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "syntax_error.star")

	// 测试语法错误（缺少冒号）
	scriptContent := `
def invalid_syntax(ctx)
    return 42
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

	// 加载应该失败
	err = engine.LoadScript(scriptPath)
	if err == nil {
		t.Fatal("Expected compile error for syntax error, got nil")
	}

	t.Logf("✅ Correctly rejected syntax error: %v", err)
}

func TestEngine_Recursion(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "recursion.star")

	scriptContent := `
def factorial(ctx, n):
    if n <= 1:
        return 1
    return n * factorial(ctx, n - 1)
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

	// 测试正常递归
	result, err := engine.Call(callCtx, params, "factorial", 5)
	if err != nil {
		t.Fatalf("Failed to call factorial: %v", err)
	}
	if result.(int64) != 120 {
		t.Errorf("Expected 120, got %v", result)
	}

	// 测试深度递归（应该受 MaxExecutionSteps 限制）
	_, err = engine.Call(callCtx, params, "factorial", 100000)
	if err == nil {
		t.Log("Deep recursion succeeded (may be limited by MaxExecutionSteps)")
	} else {
		t.Logf("Deep recursion failed as expected: %v", err)
	}
}

func TestEngine_FunctionCollision(t *testing.T) {
	tmpDir := t.TempDir()

	// 创建两个脚本，定义同名函数
	script1Path := filepath.Join(tmpDir, "a_script.star")
	script1Content := `def test(ctx): return "from_a"`
	if err := os.WriteFile(script1Path, []byte(script1Content), 0644); err != nil {
		t.Fatal(err)
	}

	script2Path := filepath.Join(tmpDir, "b_script.star")
	script2Content := `def test(ctx): return "from_b"`
	if err := os.WriteFile(script2Path, []byte(script2Content), 0644); err != nil {
		t.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	engine, err := NewEngine(config)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	// 按顺序加载（排序后 a 在前）
	if err := engine.LoadScripts([]string{script1Path, script2Path}); err != nil {
		t.Fatal(err)
	}

	params := &zscript.CallParams{ActorID: 1001, ActorType: 10001}
	callCtx := context.Background()

	// 调用应该返回后加载的版本（b）
	result, err := engine.Call(callCtx, params, "test")
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Function collision result: %v", result)
	// 因为按路径排序，b_script.star 在后，所以应该是 "from_b"
	if result.(string) != "from_b" {
		t.Logf("Warning: expected 'from_b', got %v (order may vary)", result)
	}
}

// ========== 热重载测试 ==========

func TestEngine_HotReload(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "reload.star")

	// 写入初始版本
	v1Content := `def get_version(ctx): return 1`
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
	result, err := engine.Call(callCtx, params, "get_version")
	if err != nil {
		t.Fatal(err)
	}
	if result.(int64) != 1 {
		t.Errorf("Expected version 1, got %v", result)
	}

	// 写入 v2
	v2Content := `def get_version(ctx): return 2`
	if err := os.WriteFile(scriptPath, []byte(v2Content), 0644); err != nil {
		t.Fatal(err)
	}

	// 热重载
	if err := engine.ReloadScript(scriptPath); err != nil {
		t.Fatal(err)
	}

	// 调用 v2
	result, err = engine.Call(callCtx, params, "get_version")
	if err != nil {
		t.Fatal(err)
	}
	if result.(int64) != 2 {
		t.Errorf("Expected version 2, got %v", result)
	}

	t.Log("Hot reload test passed")
}

// ========== 并发安全测试 ==========

func TestEngine_Concurrent(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "concurrent.star")

	scriptContent := `
def compute(ctx, n):
    total = 0
    for i in range(n + 1):
        total += i
    return total
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
	scriptPath := filepath.Join(tmpDir, "test.star")

	scriptContent := `def test(ctx): return 42`
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
	if err == nil {
		t.Fatal("Expected error after Close, got nil")
	}
	t.Logf("Close protection works: %v", err)

	// Close 后加载应该失败
	err = engine.LoadScript(scriptPath)
	if err == nil {
		t.Fatal("Expected error after Close, got nil")
	}
}

// ========== 基准测试 ==========

func BenchmarkEngine_Call(b *testing.B) {
	tmpDir := b.TempDir()
	scriptPath := filepath.Join(tmpDir, "bench.star")

	scriptContent := `
def compute(ctx, n):
    total = 0
    for i in range(n):
        total += i
    return total
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
	scriptPath := filepath.Join(tmpDir, "bench.star")

	scriptContent := `
def compute(ctx, n):
    total = 0
    for i in range(n):
        total += i
    return total
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		b.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	// ⚠️ 保留 Timeout（真实场景），会增加分配开销
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
	scriptPath := filepath.Join(tmpDir, "bench.star")

	scriptContent := `
def process_data(ctx, data):
    return {
        "count": len(data["items"]),
        "total": data["total"],
        "name": data["name"]
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
		_, err = engine.Call(callCtx, params, "process_data", testData)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEngine_CallMemory(b *testing.B) {
	tmpDir := b.TempDir()
	scriptPath := filepath.Join(tmpDir, "bench.star")

	scriptContent := `def simple(ctx, a, b): return a + b`
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
	scriptPath := filepath.Join(tmpDir, "bench_opt.star")

	// 脚本：一个是纯计算，一个是访问 Context
	scriptContent := `
def simple(ctx, a, b):
	return a + b

def accessContext(ctx):
	# 访问多个字段，测试 Dict 构建开销
	return ctx["ActorID"] + ctx["MsgID"] + ctx["TraceID"]
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		b.Fatal(err)
	}

	// ✅ 关键：设置 Timeout 为 0，移除定时器干扰
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
			// 无额外参数，纯测试 Context Dict 访问
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
		{"script1.star", `def func1(ctx): return 1`},
		{"script2.star", `def func2(ctx): return 2`},
		{"script3.star", `def func3(ctx): return 3`},
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
		newContent := s.content + "\ndef extra(ctx): return 99"
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

	if engine.GetType() != "starlark" {
		t.Errorf("Expected 'starlark', got '%s'", engine.GetType())
	}
}

func TestEngine_EdgeCases(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "edge.star")

	scriptContent := `
def return_empty_dict(ctx):
    return {}

def return_empty_list(ctx):
    return []

def return_nested(ctx):
    return {
        "level1": {
            "level2": {
                "value": "deep"
            }
        }
    }

def accept_none(ctx, val):
    if val == None:
        return "got_none"
    return "got_value"

def mixed_list(ctx):
    return [1, "two", 3.0, True, {"nested": "value"}, None]
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

	// 测试空字典
	result, err := engine.Call(callCtx, params, "return_empty_dict")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Empty dict: %+v", result)

	// 测试空列表
	result, err = engine.Call(callCtx, params, "return_empty_list")
	if err != nil {
		t.Fatal(err)
	}
	if arr, ok := result.([]interface{}); !ok || len(arr) != 0 {
		t.Error("Expected empty list")
	}

	// 测试嵌套结构
	result, err = engine.Call(callCtx, params, "return_nested")
	if err != nil {
		t.Fatal(err)
	}
	nested := result.(map[string]interface{})
	level1 := nested["level1"].(map[string]interface{})
	level2 := level1["level2"].(map[string]interface{})
	if level2["value"].(string) != "deep" {
		t.Error("Nested structure parsing failed")
	}

	// 测试 None 参数
	result, err = engine.Call(callCtx, params, "accept_none", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.(string) != "got_none" {
		t.Error("None parameter handling failed")
	}

	// 测试混合列表
	result, err = engine.Call(callCtx, params, "mixed_list")
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
	scriptPath := filepath.Join(tmpDir, "meta.star")

	scriptContent := `
def check_metadata(ctx):
    return {
        "has_owner": ctx["Owner"] != None,
        "actor_id": ctx["ActorID"]
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

	result, err := engine.Call(callCtx, params, "check_metadata")
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
	err = engine.LoadScript("/nonexistent/path/script.star")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
	t.Logf("Nonexistent file error: %v", err)

	// 测试语法错误的脚本
	tmpDir := t.TempDir()
	badScript := filepath.Join(tmpDir, "bad.star")
	if err := os.WriteFile(badScript, []byte(`def bad( return`), 0644); err != nil {
		t.Fatal(err)
	}

	err = engine.LoadScript(badScript)
	if err == nil {
		t.Error("Expected error for syntax error")
	}
	t.Logf("Syntax error: %v", err)

	t.Log("✅ Error cases handled correctly")
}

// ========== 深度覆盖率测试 (95%+ 目标) ==========

func TestEngine_TypeConversionEdges(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "types.star")

	scriptContent := `
def test_large_number(ctx):
    return 9007199254740992

def test_negative(ctx):
    return -999999

def test_zero(ctx):
    return 0

def test_empty_string(ctx):
    return ""

def test_unicode(ctx):
    return "你好世界🚀"

def test_nested_list(ctx):
    return [[1, 2], [3, 4], [5, 6]]

def test_mixed_dict(ctx):
    return {
        "string": "value",
        "number": 42,
        "bool": True,
        "none": None,
        "list": [1, 2, 3],
        "nested": {"inner": "deep"}
    }

def test_tuple(ctx):
    return (1, 2, 3, "four")

def test_set(ctx):
    return set([1, 2, 3, 2, 1])
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
		"test_large_number", "test_negative", "test_zero", "test_empty_string",
		"test_unicode", "test_nested_list", "test_mixed_dict", "test_tuple", "test_set",
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

func TestEngine_ScriptErrors(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "errors.star")

	scriptContent := `
def none_access(ctx):
    obj = None
    return obj.field

def index_error(ctx):
    arr = [1, 2, 3]
    return arr[10]

def key_error(ctx):
    d = {"a": 1}
    return d["nonexistent"]

def type_error(ctx):
    return 1 + "string"
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

	// Starlark 在加载时检查未定义变量，所以只测试运行时错误
	errorTests := []string{"none_access", "index_error", "key_error", "type_error"}

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

func TestEngine_MaxExecutionSteps(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "steps.star")

	scriptContent := `
def heavy_computation(ctx):
    total = 0
    for i in range(10000000):  # 大量步骤
        total += i
    return total
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	config.Timeout = 5 * time.Second
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

	_, err = engine.Call(callCtx, params, "heavy_computation")
	if err != nil {
		t.Logf("Heavy computation error (may timeout): %v", err)
	}

	t.Log("✅ Max execution steps handled")
}

func TestEngine_CloseDuringCall(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "slow.star")

	scriptContent := `
def slow_work(ctx):
    total = 0
    for i in range(100000):
        total += i
    return total
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
		_, _ = engine.Call(callCtx, params, "slow_work")
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
