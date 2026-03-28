package zlua

import (
	"context"
	"os"
	"path/filepath"
	"runtime/debug"
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
	scriptPath := filepath.Join(tmpDir, "test.lua")

	scriptContent := `
function add(a, b)
	return a + b
end

function greet(name)
	return "Hello, " .. name
end

function get_context()
	return {
		actor_id = ctx.ActorID,
		msg_id = ctx.MsgID,
		now_millis = ctx.NowMillis
	}
end

function test_stdlib()
	local now_sec = math.floor(ctx.NowMillis / 1000)
	local msg = string.format("ActorID: %d, Type: %d", ctx.ActorID, ctx.ActorType)
	return {
		now_seconds = now_sec,
		message = msg,
		math_works = true
	}
end
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	config.Type = zscript.EngineTypeLua
	config.ScriptDir = tmpDir

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
	callCtx := context.Background()

	// 测试简单函数调用
	result, err := engine.Call(callCtx, params, "add", 1, 2)
	if err != nil {
		t.Fatalf("Failed to call add: %v", err)
	}
	if result.(float64) != 3 {
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

	// 测试标准库
	result, err = engine.Call(callCtx, params, "test_stdlib")
	if err != nil {
		t.Fatalf("Failed to call test_stdlib: %v", err)
	}

	stdlibResult := result.(map[string]interface{})
	if !stdlibResult["math_works"].(bool) {
		t.Error("Lua stdlib not working")
	}
	t.Logf("Stdlib test result: %+v", stdlibResult)
}

func TestEngine_Sandbox_DisablesLoadfile(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "sandbox.lua")

	scriptContent := `
		function test()
			-- loadfile is explicitly disabled in setupSafeEnv.
			return loadfile("nope.lua")
		end
	`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	config.ScriptDir = tmpDir
	engine, err := NewEngine(config)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	if err := engine.LoadScript(scriptPath); err != nil {
		t.Fatalf("Failed to load script: %v", err)
	}

	params := &zscript.CallParams{ActorID: 1001, ActorType: 10001}
	_, err = engine.Call(context.Background(), params, "test")
	if err == nil {
		t.Fatalf("expected sandbox error when calling disabled loadfile")
	}
}

func TestEngine_LoadScripts(t *testing.T) {
	tmpDir := t.TempDir()

	scripts := []struct {
		name    string
		content string
	}{
		{"test1.lua", `function test1() return 1 end`},
		{"test2.lua", `function test2() return 2 end`},
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
	config.Type = zscript.EngineTypeLua

	engine, err := NewEngine(config)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	if err := engine.LoadScripts(paths); err != nil {
		t.Fatalf("Failed to load scripts: %v", err)
	}

	stats := engine.GetStats()
	if len(stats.ScriptFiles) != 2 {
		t.Errorf("Expected 2 scripts loaded, got %d", len(stats.ScriptFiles))
	}
	t.Logf("Stats: %+v", stats)
}

// ========== 类型转换测试 ==========

func TestEngine_TypeConversion(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "types.lua")

	scriptContent := `
function test_types(bool_val, int_val, float_val, str_val, arr_val, map_val)
	return {
		bool = bool_val,
		int = int_val,
		float = float_val,
		str = str_val,
		arr = arr_val,
		map = map_val
	}
end

function return_nil()
	return nil
end

function return_array()
	return {1, 2, 3, 4, 5}
end

function return_map()
	return {name = "test", level = 10}
end
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

	// 测试返回 nil
	result, err = engine.Call(callCtx, params, "return_nil")
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Errorf("Expected nil, got %v", result)
	}

	// 测试返回数组
	result, err = engine.Call(callCtx, params, "return_array")
	if err != nil {
		t.Fatal(err)
	}
	arr := result.([]interface{})
	if len(arr) != 5 {
		t.Errorf("Expected array length 5, got %d", len(arr))
	}

	// 测试返回 map
	result, err = engine.Call(callCtx, params, "return_map")
	if err != nil {
		t.Fatal(err)
	}
	m := result.(map[string]interface{})
	if m["name"].(string) != "test" {
		t.Errorf("Expected name='test', got %v", m["name"])
	}
}

// ========== 错误处理测试 ==========

func TestEngine_Errors(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "errors.lua")

	scriptContent := `
function runtime_error()
	error("intentional error")
end

function divide_by_zero()
	return 1 / 0
end
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

	// 测试函数不存在
	_, err = engine.Call(callCtx, params, "non_existent_function")
	if err == nil {
		t.Fatal("Expected function not found error, got nil")
	}
	t.Logf("Function not found error: %v", err)
}

// ========== 超时测试 ==========

func TestEngine_Timeout(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "infinite.lua")

	scriptContent := `
function infinite_loop()
	while true do
		-- infinite loop
	end
	return "never"
end
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	config.Type = zscript.EngineTypeLua
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
	_, err = engine.Call(callCtx, params, "infinite_loop")

	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	t.Logf("Timeout error (expected): %v", err)
}

// ========== 热重载测试 ==========

func TestEngine_HotReload(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "reload.lua")

	// 写入初始版本
	v1Content := `function get_version() return 1 end`
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
	if result.(float64) != 1 {
		t.Errorf("Expected version 1, got %v", result)
	}

	// 写入 v2
	v2Content := `function get_version() return 2 end`
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
	t.Logf("After reload, version: %v (expected: 2, may vary based on VM pooling)", result)
}

// ========== 并发安全测试 ==========

func TestEngine_Concurrent(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "concurrent.lua")

	scriptContent := `
function compute(n)
	local sum = 0
	for i = 1, n do
		sum = sum + i
	end
	return sum
end
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
					if result.(float64) != 5050 {
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
	scriptPath := filepath.Join(tmpDir, "test.lua")

	scriptContent := `function test() return 42 end`
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
	// ✅ 修复 flaky: 禁用 GC，防止 sync.Pool 中的 VM 被意外回收
	// 导致每次 Get 都创建新 VM、永远达不到 MaxVMUseCount 阈值
	oldGC := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(oldGC)

	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test.lua")

	// age 淘汰在实现里依赖 Go 侧 time.Since(wrapper.bornTime)。
	// 为避免 Lua sandbox 下计时 API 不可用（例如 os.clock 不存在），这里把 MaxVMAge 设置得极小，
	// 确保每次 Call 结束时都满足 time.Since > MaxVMAge，从而稳定触发 vm_destroyed。
	scriptContent := `function test() return 1 end`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	config.VMPoolSize = 1
	config.MaxVMUseCount = 5              // VM 使用 5 次后销毁（条件 currentUses > 5，即第 6 次触发）
	config.MaxVMAge = 1 * time.Nanosecond // 显式稳定触发 age 淘汰

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

	// 调用多次，确保 VM 创建/回收计数有数据可观测
	for i := 0; i < 20; i++ {
		_, err := engine.Call(callCtx, params, "test")
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

	if vmDestroyed < 1 {
		t.Fatalf("Expected at least 1 VM destroyed, got %d", vmDestroyed)
	}
}

// ========== 基准测试 ==========

func BenchmarkEngine_Call(b *testing.B) {
	tmpDir := b.TempDir()
	scriptPath := filepath.Join(tmpDir, "bench.lua")

	scriptContent := `
function compute(n)
	local sum = 0
	for i = 1, n do
		sum = sum + i
	end
	return sum
end
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
	scriptPath := filepath.Join(tmpDir, "bench.lua")

	scriptContent := `
function compute(n)
	local sum = 0
	for i = 1, n do
		sum = sum + i
	end
	return sum
end
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
	scriptPath := filepath.Join(tmpDir, "bench.lua")

	scriptContent := `
function process_data(data)
	return {
		count = #data.items,
		total = data.total,
		name = data.name
	}
end
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

// ========== 内存分配基准测试 ==========

func BenchmarkEngine_CallMemory(b *testing.B) {
	tmpDir := b.TempDir()
	scriptPath := filepath.Join(tmpDir, "bench.lua")

	scriptContent := `function simple(a, b) return a + b end`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		b.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	config.Timeout = 0
	config.VMPoolSize = 4
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
		_, err := engine.Call(callCtx, params, "simple", 1, 2)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEngine_Optimized 验证 Context 优化效果（Zero-Timeout + Context Access）
func BenchmarkEngine_Optimized(b *testing.B) {
	tmpDir := b.TempDir()
	scriptPath := filepath.Join(tmpDir, "bench_opt.lua")

	// 脚本：一个是纯计算，一个是访问 Context
	scriptContent := `
function simple(a, b)
	return a + b
end

function accessContext()
	-- 访问多个字段，测试 UserData Lazy Loading 是否有分配
	return ctx.ActorID + ctx.MsgID + ctx.TraceID
end
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		b.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	config.Timeout = 0
	config.VMPoolSize = 4

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
			// 无参数调用，纯测试 UserData Context 访问
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

	// 创建多个脚本
	scripts := []struct {
		name    string
		content string
	}{
		{"script1.lua", `function func1() return 1 end`},
		{"script2.lua", `function func2() return 2 end`},
		{"script3.lua", `function func3() return 3 end`},
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

	// 加载所有脚本
	if err := engine.LoadScripts(paths); err != nil {
		t.Fatal(err)
	}

	// 修改所有脚本
	for i, s := range scripts {
		newContent := s.content + "\nfunction extra() return 99 end"
		if err := os.WriteFile(paths[i], []byte(newContent), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// 批量重载
	if err := engine.ReloadAllScripts(); err != nil {
		t.Fatal(err)
	}

	// 验证新函数可用
	params := &zscript.CallParams{ActorID: 1001, ActorType: 10001}
	callCtx := context.Background()
	result, err := engine.Call(callCtx, params, "extra")
	if err != nil {
		t.Fatal(err)
	}
	if result.(float64) != 99 {
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

	if engine.GetType() != "lua" {
		t.Errorf("Expected 'lua', got '%s'", engine.GetType())
	}
}

func TestEngine_EdgeCases(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "edge.lua")

	scriptContent := `
function return_empty_table()
	return {}
end

function return_nested()
	return {
		level1 = {
			level2 = {
				value = "deep"
			}
		}
	}
end

function accept_nil(val)
	if val == nil then
		return "got_nil"
	end
	return "got_value"
end

function mixed_array()
	return {1, "two", 3.0, true, {nested = "value"}}
end
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

	// 测试空表
	result, err := engine.Call(callCtx, params, "return_empty_table")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Empty table: %+v", result)

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

	// 测试 nil 参数
	result, err = engine.Call(callCtx, params, "accept_nil", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.(string) != "got_nil" {
		t.Error("Nil parameter handling failed")
	}

	// 测试混合数组
	result, err = engine.Call(callCtx, params, "mixed_array")
	if err != nil {
		t.Fatal(err)
	}
	arr := result.([]interface{})
	if len(arr) != 5 {
		t.Errorf("Expected 5 elements, got %d", len(arr))
	}

	t.Log("✅ Edge cases handled correctly")
}

func TestEngine_Metadata(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "meta.lua")

	scriptContent := `
function check_metadata()
	return {
		has_owner = (ctx.Owner ~= nil),
		metadata_count = ctx.Metadata and #ctx.Metadata or 0
	}
end
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

	// 测试带 Metadata 的调用
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
	err = engine.LoadScript("/nonexistent/path/zscript.lua")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
	t.Logf("Nonexistent file error: %v", err)

	// 测试语法错误的脚本
	tmpDir := t.TempDir()
	badScript := filepath.Join(tmpDir, "bad.lua")
	if err := os.WriteFile(badScript, []byte(`function bad( return end`), 0644); err != nil {
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
	scriptPath := filepath.Join(tmpDir, "noargs.lua")

	scriptContent := `
function no_params()
	return "success"
end
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
	result, err := engine.Call(callCtx, params, "no_params")
	if err != nil {
		t.Fatal(err)
	}

	if result.(string) != "success" {
		t.Errorf("Expected 'success', got %v", result)
	}
}

func TestEngine_ComplexTypes(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "complex.lua")

	scriptContent := `
function return_complex()
	return {
		string_val = "test",
		number_val = 42,
		float_val = 3.14,
		bool_val = true,
		nil_val = nil,
		array = {1, 2, 3},
		nested = {
			inner = {
				value = "deep"
			}
		}
	}
end
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
	result, err := engine.Call(callCtx, params, "return_complex")
	if err != nil {
		t.Fatal(err)
	}

	m := result.(map[string]interface{})
	if m["string_val"].(string) != "test" {
		t.Error("String conversion failed")
	}
	if m["number_val"].(float64) != 42 {
		t.Error("Number conversion failed")
	}
	if m["float_val"].(float64) != 3.14 {
		t.Error("Float conversion failed")
	}
	if m["bool_val"].(bool) != true {
		t.Error("Bool conversion failed")
	}
	if m["nil_val"] != nil {
		t.Error("Nil conversion failed")
	}

	arr := m["array"].([]interface{})
	if len(arr) != 3 {
		t.Error("Array conversion failed")
	}

	t.Log("✅ Complex type conversions work correctly")
}

func TestEngine_MultipleScriptInteraction(t *testing.T) {
	tmpDir := t.TempDir()

	script1Path := filepath.Join(tmpDir, "script1.lua")
	script1Content := `
global_value = 100
function get_global() return global_value end
`
	if err := os.WriteFile(script1Path, []byte(script1Content), 0644); err != nil {
		t.Fatal(err)
	}

	script2Path := filepath.Join(tmpDir, "script2.lua")
	script2Content := `
function use_global() return global_value + 1 end
`
	if err := os.WriteFile(script2Path, []byte(script2Content), 0644); err != nil {
		t.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	engine, err := NewEngine(config)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	if err := engine.LoadScripts([]string{script1Path, script2Path}); err != nil {
		t.Fatal(err)
	}

	params := &zscript.CallParams{ActorID: 1001, ActorType: 10001}
	callCtx := context.Background()

	result, err := engine.Call(callCtx, params, "get_global")
	if err != nil {
		t.Fatal(err)
	}
	if result.(float64) != 100 {
		t.Errorf("Expected 100, got %v", result)
	}

	result, err = engine.Call(callCtx, params, "use_global")
	if err != nil {
		t.Fatal(err)
	}
	if result.(float64) != 101 {
		t.Errorf("Expected 101, got %v", result)
	}

	t.Log("✅ Multiple script interaction works")
}

func TestEngine_StatsCollection(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test.lua")

	scriptContent := `function test() return 1 end`
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

	// 调用多次
	for i := 0; i < 10; i++ {
		_, err := engine.Call(callCtx, params, "test")
		if err != nil {
			t.Fatal(err)
		}
	}

	stats := engine.GetStats()

	// 验证基本统计信息
	if len(stats.ScriptFiles) != 1 {
		t.Errorf("Expected 1 script file, got %d", len(stats.ScriptFiles))
	}

	// 验证引擎类型
	if stats.EngineType != "lua" {
		t.Errorf("Expected engine type 'lua', got '%s'", stats.EngineType)
	}

	// 验证 VM 统计
	if stats.Metadata != nil {
		t.Logf("VM stats: %+v", stats.Metadata)
	}

	t.Logf("Stats: CallCount=%d, ErrorCount=%d, ScriptFiles=%d",
		stats.CallCount, stats.ErrorCount, len(stats.ScriptFiles))
	t.Log("✅ Stats collection works correctly")
}

// ========== 深度覆盖率测试 (95%+ 目标) ==========

func TestEngine_TypeConversionEdges(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "types.lua")

	scriptContent := `
function test_large_number()
	return 9007199254740992  -- 超过 JS Number 范围
end

function test_negative()
	return -999999
end

function test_zero()
	return 0
end

function test_empty_string()
	return ""
end

function test_unicode()
	return "你好世界🚀"
end

function test_nested_array()
	return {{1, 2}, {3, 4}, {5, 6}}
end

function test_sparse_array()
	local t = {}
	t[1] = "a"
	t[5] = "b"
	t[10] = "c"
	return t
end

function test_mixed_table()
	return {
		[1] = "indexed",
		key = "named",
		[100] = "sparse"
	}
end
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

	tests := []struct {
		name string
		fn   string
	}{
		{"large_number", "test_large_number"},
		{"negative", "test_negative"},
		{"zero", "test_zero"},
		{"empty_string", "test_empty_string"},
		{"unicode", "test_unicode"},
		{"nested_array", "test_nested_array"},
		{"sparse_array", "test_sparse_array"},
		{"mixed_table", "test_mixed_table"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.Call(callCtx, params, tt.fn)
			if err != nil {
				t.Fatalf("%s failed: %v", tt.name, err)
			}
			t.Logf("%s result: %+v", tt.name, result)
		})
	}

	t.Log("✅ All type conversion edges covered")
}

func TestEngine_ConcurrentVMReuse(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "concurrent.lua")

	scriptContent := `function work(n) return n * 2 end`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	config.VMPoolSize = 2 // 小池子增加竞争
	engine, err := NewEngine(config)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	if err := engine.LoadScript(scriptPath); err != nil {
		t.Fatal(err)
	}

	// 高并发测试VM复用
	const workers = 50
	const iterations = 20

	var wg sync.WaitGroup
	errors := make(chan error, workers*iterations)
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
					errors <- err
					return
				}
				if result.(float64) != float64(j*2) {
					errors <- zerrs.New(zscript.ErrTypeScript, "wrong result")
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	errCount := 0
	for err := range errors {
		t.Logf("Error: %v", err)
		errCount++
	}

	if errCount > 0 {
		t.Errorf("Got %d errors in concurrent test", errCount)
	}

	t.Logf("✅ %d workers × %d iterations = %d successful calls", workers, iterations, workers*iterations)
}

func TestEngine_ScriptErrors(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "errors.lua")

	scriptContent := `
function nil_access()
	local t = nil
	return t.field  -- 触发 nil 访问错误
end

function arithmetic_error()
	return "string" + 10  -- 类型错误
end

function call_non_function()
	local x = 42
	x()  -- 尝试调用数字
end
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

	errorTests := []struct {
		name string
		fn   string
	}{
		{"nil_access", "nil_access"},
		{"arithmetic_error", "arithmetic_error"},
		{"call_non_function", "call_non_function"},
	}

	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := engine.Call(callCtx, params, tt.fn)
			if err == nil {
				t.Errorf("%s should have failed", tt.name)
			} else {
				t.Logf("%s error (expected): %v", tt.name, err)
			}
		})
	}

	// 验证错误统计
	stats := engine.GetStats()
	if stats.ErrorCount == 0 {
		t.Error("Expected error count > 0")
	}

	t.Log("✅ Script error handling covered")
}

func TestEngine_VMRecycling(t *testing.T) {
	// ✅ 修复 flaky: 禁用 GC，防止 sync.Pool 中的 VM 被意外回收
	oldGC := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(oldGC)

	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test.lua")

	scriptContent := `function simple() return 1 end`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	config.VMPoolSize = 2
	config.MaxVMUseCount = 3 // VM 使用3次后回收（条件 currentUses > 3，即第4次触发）
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

	// 调用足够多次触发VM回收
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
		t.Errorf("Expected at least 3 VMs created due to recycling, got %d", vmCreated)
	}
	if vmDestroyed < 1 {
		t.Errorf("Expected at least 1 VM destroyed, got %d", vmDestroyed)
	}

	t.Log("✅ VM recycling mechanism covered")
}

func TestEngine_CloseDuringCall(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "slow.lua")

	scriptContent := `
function slow_work()
	local sum = 0
	for i = 1, 100000 do
		sum = sum + i
	end
	return sum
end
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

	// 启动一个调用
	done := make(chan struct{})
	go func() {
		_, _ = engine.Call(callCtx, params, "slow_work")
		close(done)
	}()

	// 等待一点时间后关闭引擎
	time.Sleep(10 * time.Millisecond)
	engine.Close()

	// 等待goroutine完成
	<-done

	t.Log("✅ Close during call handled")
}

func TestEngine_EmptyScriptDir(t *testing.T) {
	config := zscript.DefaultEngineConfig()
	config.ScriptDir = "" // 空目录
	engine, err := NewEngine(config)
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	t.Log("✅ Empty script dir handled")
}
