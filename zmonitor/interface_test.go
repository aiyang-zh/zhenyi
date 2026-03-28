package zmonitor

import (
	"runtime"
	"testing"
	"time"
)

func TestActorStats(t *testing.T) {
	stats := NewActorStats()

	// Simulate processing 10 messages.
	// 模拟处理 10 条消息。
	for i := 0; i < 10; i++ {
		latency := int64(5 * time.Millisecond)
		isSlow := i%3 == 0 // One slow message every 3 messages / 每 3 条就有 1 条慢消息
		stats.RecordMessage(latency, isSlow)
		time.Sleep(time.Millisecond)
	}

	// Record 2 errors.
	// 记录 2 个错误。
	stats.RecordError()
	stats.RecordError()

	// Get snapshot.
	// 获取快照。
	processed, avgLatency, qps, slowCount, errorCount := stats.GetSnapshot()

	if processed != 10 {
		t.Errorf("Expected processed=10, got %d", processed)
	}

	if avgLatency != 5 {
		t.Errorf("Expected avgLatency=5ms, got %d", avgLatency)
	}

	if slowCount != 4 { // 0,3,6,9 -> 4 messages / 4 条
		t.Errorf("Expected slowCount=4, got %d", slowCount)
	}

	if errorCount != 2 {
		t.Errorf("Expected errorCount=2, got %d", errorCount)
	}

	if qps <= 0 {
		t.Errorf("Expected qps>0, got %f", qps)
	}

	t.Logf("Stats: processed=%d, avgLatency=%dms, qps=%.2f, slow=%d, error=%d",
		processed, avgLatency, qps, slowCount, errorCount)
}

func TestSessionStats(t *testing.T) {
	stats := NewSessionStats()
	time.Sleep(10 * time.Millisecond) // Ensure time passes / 确保时间流逝

	// Record sends.
	// 记录发送。
	stats.RecordSend(5, 1024)  // 5条消息，1024字节
	stats.RecordSend(10, 2048) // 10条消息，2048字节

	// Record receives.
	// 记录接收。
	for i := 0; i < 8; i++ {
		stats.RecordRecv(512) // 每条512字节
	}

	// Get snapshot.
	// 获取快照。
	sendCount, recvCount, sendBytes, recvBytes, connectedAt, lastActiveMs := stats.GetSnapshot()

	if sendCount != 15 {
		t.Errorf("Expected sendCount=15, got %d", sendCount)
	}

	if sendBytes != 3072 {
		t.Errorf("Expected sendBytes=3072, got %d", sendBytes)
	}

	if recvCount != 8 {
		t.Errorf("Expected recvCount=8, got %d", recvCount)
	}

	if recvBytes != 4096 {
		t.Errorf("Expected recvBytes=4096, got %d", recvBytes)
	}

	if connectedAt == 0 {
		t.Error("Expected connectedAt>0")
	}

	if lastActiveMs == 0 {
		t.Error("Expected lastActiveMs>0")
	}

	t.Logf("Stats: send=%d/%d bytes, recv=%d/%d bytes, connected=%d, lastActive=%d",
		sendCount, sendBytes, recvCount, recvBytes, connectedAt, lastActiveMs)
}

func TestCollectSystemMonitor(t *testing.T) {
	data := CollectSystemMonitor()

	if data.Type != "system" {
		t.Errorf("Expected type=system, got %s", data.Type)
	}

	if data.MemStats == nil {
		t.Fatal("Expected MemStats not nil")
	}

	if data.MemStats.AllocMB <= 0 {
		t.Error("Expected AllocMB>0")
	}

	if data.GCStats == nil {
		t.Fatal("Expected GCStats not nil")
	}

	if data.GoroutineNum <= 0 {
		t.Error("Expected GoroutineNum>0")
	}

	if data.CPUNum != runtime.NumCPU() {
		t.Errorf("Expected CPUNum=%d, got %d", runtime.NumCPU(), data.CPUNum)
	}

	t.Logf("System: Alloc=%.2fMB, Goroutines=%d, CPU=%d, GC=%d",
		data.MemStats.AllocMB,
		data.GoroutineNum,
		data.CPUNum,
		data.GCStats.NumGC)
}

func TestMonitorManager(t *testing.T) {
	mgr := NewManager()

	// Create mock components.
	// 创建模拟组件。
	mockComponent1 := &mockComponent{
		id:   "test_1",
		typ:  "mock",
		name: "Mock Component 1",
	}
	mockComponent2 := &mockComponent{
		id:   "test_2",
		typ:  "mock",
		name: "Mock Component 2",
	}

	// Register.
	// 注册。
	mgr.Register("test_1", mockComponent1)
	mgr.Register("test_2", mockComponent2)

	// Get all.
	// 获取所有。
	all := mgr.GetAll()
	if len(all) != 2 {
		t.Errorf("Expected 2 components, got %d", len(all))
	}

	// Get one.
	// 获取单个。
	data, ok := mgr.Get("test_1")
	if !ok {
		t.Error("Expected component test_1 to exist")
	}
	if data.ID != "test_1" {
		t.Errorf("Expected ID=test_1, got %s", data.ID)
	}

	// Get by type.
	// 按类型获取。
	mocks := mgr.GetByType("mock")
	if len(mocks) != 2 {
		t.Errorf("Expected 2 mock components, got %d", len(mocks))
	}

	// Unregister.
	// 注销。
	mgr.Unregister("test_1")
	all = mgr.GetAll()
	if len(all) != 1 {
		t.Errorf("Expected 1 component after unregister, got %d", len(all))
	}

	t.Log("Manager test passed")
}

// mockComponent is a minimal IMonitorable implementation used in tests.
// mockComponent 测试用的最小 IMonitorable 实现。
type mockComponent struct {
	id   string
	typ  string
	name string
}

func (m *mockComponent) GetMonitorData() MonitorData {
	return MonitorData{
		Type:      m.typ,
		ID:        m.id,
		Name:      m.name,
		Status:    "running",
		Timestamp: time.Now().UnixMilli(),
		Metrics: map[string]interface{}{
			"test_metric": 123,
		},
		Tags: map[string]string{
			"test_tag": "value",
		},
	}
}

func BenchmarkActorStats(b *testing.B) {
	stats := NewActorStats()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			stats.RecordMessage(5*int64(time.Millisecond), false)
		}
	})
}

func BenchmarkSessionStats(b *testing.B) {
	stats := NewSessionStats()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			stats.RecordSend(1, 100)
		}
	})
}

func BenchmarkCollectSystemMonitor(b *testing.B) {
	for i := 0; i < b.N; i++ {
		collectSystemMonitorFresh()
	}
}
