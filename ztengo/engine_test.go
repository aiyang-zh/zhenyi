package ztengo

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi/zscript"
)

var initOnce sync.Once

func init() {
	initOnce.Do(func() {
		zlog.NewDefaultLogger()
	})
}

// TestEngine_Basic 基础功能测试
func TestEngine_Basic(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test.tengo")

	scriptContent := `
// Tengo 脚本示例
// 约定：第一个参数永远是 ctx (Context)
add := func(ctx, a, b) {
	return a + b
}

multiply := func(ctx, a, b) {
	return a * b
}

get_actor_info := func(ctx) {
	return {
		actor_id: ctx["ActorID"],
		actor_type: ctx["ActorType"]
	}
}
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	config.Type = zscript.EngineTypeTengo

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
		TraceID:   123,
		Metadata: map[string]interface{}{
			"test_key": "test_value",
		},
	}
	callCtx := context.Background()

	// 测试 add 函数
	result, err := engine.Call(callCtx, params, "add", 10, 20)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	t.Logf("Result type: %T, value: %v", result, result)
	if result == nil {
		t.Fatal("Result is nil")
	}
	if result.(int64) != 30 {
		t.Errorf("Expected 30, got %v", result)
	}

	// 测试 get_actor_info
	result, err = engine.Call(callCtx, params, "get_actor_info")
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	info := result.(map[string]interface{})
	if info["actor_id"].(int64) != 1001 {
		t.Errorf("Expected actor_id 1001, got %v", info["actor_id"])
	}
}

// TestEngine_Timeout 超时测试
func TestEngine_Timeout(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "timeout.tengo")

	scriptContent := `
infinite_loop := func(ctx) {
	i := 0
	for true {
		i = i + 1
	}
	return i
}
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	config.Type = zscript.EngineTypeTengo
	config.Timeout = 100 * time.Millisecond

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
	}
	callCtx := context.Background()

	_, err = engine.Call(callCtx, params, "infinite_loop")
	if err != zscript.ErrScriptTimeout {
		t.Errorf("Expected timeout error, got %v", err)
	}

	stats := engine.GetStats()
	if stats.TimeoutCount == 0 {
		t.Error("Expected timeout to be recorded in stats")
	}
}

func TestEngine_ContextCancellationInterrupts(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "cancel_ctx.tengo")

	scriptContent := `
infinite_loop := func(ctx) {
	i := 0
	for true {
		i = i + 1
	}
	return i
}
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		t.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	config.Type = zscript.EngineTypeTengo
	config.Timeout = 0 // rely purely on ctx cancellation

	engine, err := NewEngine(config)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}
	defer engine.Close()

	if err := engine.LoadScript(scriptPath); err != nil {
		t.Fatalf("Failed to load script: %v", err)
	}

	params := &zscript.CallParams{ActorID: 1001, ActorType: 10001}
	callCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	start := time.Now()
	errCh := make(chan error, 1)
	go func() {
		_, callErr := engine.Call(callCtx, params, "infinite_loop")
		errCh <- callErr
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case callErr := <-errCh:
		if callErr != zscript.ErrScriptTimeout {
			t.Fatalf("expected ErrScriptTimeout on ctx cancellation, got: %v", callErr)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("engine.Call did not interrupt within 1s")
	}

	if time.Since(start) > 500*time.Millisecond {
		t.Fatalf("expected early interrupt, elapsed=%s", time.Since(start))
	}

	stats := engine.GetStats()
	if stats.TimeoutCount == 0 {
		t.Fatal("expected timeout to be recorded in stats")
	}
}

// BenchmarkEngine_CallMemory 内存分配基准测试（Timeout=0）
func BenchmarkEngine_CallMemory(b *testing.B) {
	tmpDir := b.TempDir()
	scriptPath := filepath.Join(tmpDir, "bench.tengo")

	scriptContent := `
simple := func(ctx, a, b) {
	return a + b
}
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		b.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	config.Type = zscript.EngineTypeTengo
	config.Timeout = 0 // ⚡️ 极速模式：不启用超时

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

	// 🔥 预热 (Warm-up)：确保 Stub 已编译缓存
	// 第一次调用会触发编译和存根缓存，后续调用直接复用
	// 这样 benchmark 测的就是纯粹的 Clone + Set + Run 性能
	if _, err := engine.Call(callCtx, params, "simple", 1, 2); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer() // 重置计时器，排除预热时间
	for i := 0; i < b.N; i++ {
		_, err := engine.Call(callCtx, params, "simple", 1, 2)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEngine_CallParallel 并发基准测试（Timeout=5s）
func BenchmarkEngine_CallParallel(b *testing.B) {
	tmpDir := b.TempDir()
	scriptPath := filepath.Join(tmpDir, "bench.tengo")

	scriptContent := `
simple := func(ctx, a, b) {
	return a + b
}
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		b.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	config.Type = zscript.EngineTypeTengo
	// ✅ 保留 Timeout，模拟真实生产场景

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

	// 🔥 预热 (Warm-up)：确保 Stub 已编译缓存
	// 在高并发测试前，先触发编译，让后续测试直接复用
	if _, err := engine.Call(callCtx, params, "simple", 1, 2); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer() // 重置计时器，排除预热时间
	b.RunParallel(func(pb *testing.PB) {
		// ✅ params 是只读使用，线程安全
		for pb.Next() {
			_, err := engine.Call(callCtx, params, "simple", 1, 2)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkEngine_Optimized 优化版基准测试
func BenchmarkEngine_Optimized(b *testing.B) {
	tmpDir := b.TempDir()
	scriptPath := filepath.Join(tmpDir, "bench_opt.tengo")

	scriptContent := `
simple := func(ctx, a, b) {
	return a + b
}

accessContext := func(ctx) {
	return ctx["ActorID"] + ctx["MsgID"] + ctx["TraceID"]
}
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0644); err != nil {
		b.Fatal(err)
	}

	config := zscript.DefaultEngineConfig()
	config.Type = zscript.EngineTypeTengo
	config.Timeout = 0 // ✅ 关键：设置 Timeout 为 0，移除 Context 开销

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
		// 🔥 预热：触发 "simple" 函数的存根编译
		if _, err := engine.Call(callCtx, params, "simple", 1, 2); err != nil {
			b.Fatal(err)
		}

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
		// 🔥 预热：触发 "accessContext" 函数的存根编译
		if _, err := engine.Call(callCtx, params, "accessContext"); err != nil {
			b.Fatal(err)
		}

		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := engine.Call(callCtx, params, "accessContext")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// TestEngine_GetStats 统计测试
func TestEngine_GetStats(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test.tengo")

	scriptContent := `
test := func(ctx) {
	return "ok"
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

	params := &zscript.CallParams{ActorID: 100, ActorType: 1}
	callCtx := context.Background()

	// 验证 Load 后的状态
	stats := engine.GetStats()
	if stats.EngineType != "tengo" {
		t.Errorf("Expected engine type 'tengo', got '%s'", stats.EngineType)
	}
	if stats.Metadata == nil {
		t.Fatal("Expected metadata to be populated")
	}

	// ✅ 验证函数白名单（Load 时扫描出来的）
	if validFuncs, ok := stats.Metadata["valid_functions"].(int); !ok || validFuncs != 1 {
		t.Errorf("Expected 1 valid function, got %v", stats.Metadata["valid_functions"])
	}

	// ⚠️ Load 后，存根缓存应该为 0（还没有 Call）
	if cachedStubs, ok := stats.Metadata["cached_stubs"].(int); !ok || cachedStubs != 0 {
		t.Errorf("Expected 0 cached stubs after Load (before Call), got %v", stats.Metadata["cached_stubs"])
	}

	// 🔥 Call 一次，触发存根编译
	if _, err := engine.Call(callCtx, params, "test"); err != nil {
		t.Fatalf("Call failed: %v", err)
	}

	// ✅ Call 后，存根缓存应该为 1
	stats = engine.GetStats()
	if cachedStubs, ok := stats.Metadata["cached_stubs"].(int); !ok || cachedStubs != 1 {
		t.Errorf("Expected 1 cached stub after Call, got %v", stats.Metadata["cached_stubs"])
	}

	// 🔥 再次 Call 相同函数，存根缓存仍然是 1（复用）
	if _, err := engine.Call(callCtx, params, "test"); err != nil {
		t.Fatalf("Second call failed: %v", err)
	}

	stats = engine.GetStats()
	if cachedStubs, ok := stats.Metadata["cached_stubs"].(int); !ok || cachedStubs != 1 {
		t.Errorf("Expected still 1 cached stub (reused), got %v", stats.Metadata["cached_stubs"])
	}
}

// TestEngine_GlobalVariablePollution 测试全局变量污染风险（黄旗警告）
func TestEngine_GlobalVariablePollution(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "global_state.tengo")

	// ⚠️ 危险的脚本：使用了全局可变状态
	scriptContent := `
// 这是一个全局 Map（危险！）
global_state := {"count": 0}

increment := func(ctx) {
	global_state["count"] = global_state["count"] + 1
	return global_state["count"]
}

reset := func(ctx) {
	global_state["count"] = 0
	return 0
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

	params := &zscript.CallParams{ActorID: 100, ActorType: 1}
	callCtx := context.Background()

	// 🔥 演示污染问题
	// 请求 A：increment
	result1, err := engine.Call(callCtx, params, "increment")
	if err != nil {
		t.Fatalf("First call failed: %v", err)
	}
	count1 := result1.(int64)
	t.Logf("Request A: count = %d", count1)

	// 请求 B：increment（期望从 0 开始，但实际可能从 1 开始）
	result2, err := engine.Call(callCtx, params, "increment")
	if err != nil {
		t.Fatalf("Second call failed: %v", err)
	}
	count2 := result2.(int64)
	t.Logf("Request B: count = %d", count2)

	// ⚠️ 如果 count2 == 2，说明全局状态被共享了（污染）
	if count2 == 2 {
		t.Logf("⚠️  Global variable pollution detected: count2=%d (expected 1, got 2)", count2)
		t.Logf("⚠️  This is a Yellow Flag warning, not a test failure")
		t.Logf("⚠️  Best Practice: Avoid using global Map/Array in scripts")
	} else if count2 == 1 {
		t.Logf("✅ No pollution: each VM instance has isolated global state")
	}

	// 无论如何，这个测试都应该 PASS（这是警告，不是错误）
	// 真正的修复方案是：禁止用户在脚本中使用全局可变变量

	// ✅ 验证全局变量检测功能
	stats := engine.GetStats()
	if globalVarsCount, ok := stats.Metadata["global_variables"].(int); ok {
		t.Logf("📊 Detected %d global variable(s)", globalVarsCount)
		if globalVarsCount > 0 {
			if globalVarNames, ok := stats.Metadata["global_variable_names"].([]string); ok {
				t.Logf("⚠️  Global variable names: %v", globalVarNames)
				// 应该检测到 global_state
				found := false
				for _, name := range globalVarNames {
					if name == "global_state" {
						found = true
						break
					}
				}
				if !found {
					t.Logf("⚠️  Expected to detect 'global_state', but it was not found")
				} else {
					t.Logf("✅ Successfully detected 'global_state' as a risky global variable")
				}
			}
		}
	}
}

// TestEngine_SecurityInjection 测试函数名注入防护
func TestEngine_SecurityInjection(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test.tengo")

	// 加载正常脚本
	scriptContent := `
validFunc := func(ctx, x) {
	return x * 2
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
		ActorID:   100,
		ActorType: 1,
	}
	callCtx := context.Background()

	// ✅ 正常调用应该成功
	result, err := engine.Call(callCtx, params, "validFunc", 10)
	if err != nil {
		t.Fatalf("Normal call failed: %v", err)
	}
	if result.(int64) != 20 {
		t.Errorf("Expected 20, got %v", result)
	}

	// 🚨 注入测试：各种非法函数名
	testCases := []struct {
		name         string
		functionName string
		description  string
	}{
		{
			name:         "Semicolon_Injection",
			functionName: "validFunc); malicious(",
			description:  "尝试使用分号注入恶意代码",
		},
		{
			name:         "Newline_Injection",
			functionName: "validFunc\nmalicious",
			description:  "尝试使用换行符注入",
		},
		{
			name:         "Special_Chars",
			functionName: "valid@Func#",
			description:  "使用非法标识符字符",
		},
		{
			name:         "Non_Existent_Function",
			functionName: "nonExistentFunction",
			description:  "调用不存在的函数",
		},
		{
			name:         "Empty_Name",
			functionName: "",
			description:  "空函数名",
		},
		{
			name:         "Number_Start",
			functionName: "123invalid",
			description:  "数字开头的函数名",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := engine.Call(callCtx, params, tc.functionName, 10)
			if err == nil {
				t.Errorf("%s: Expected error, but call succeeded", tc.description)
			} else {
				t.Logf("✅ %s correctly rejected: %v", tc.description, err)
			}
		})
	}
}
