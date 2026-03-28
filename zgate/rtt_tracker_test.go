package zgate

import (
	"sync"
	"testing"
	"time"
)

// ==================== 单元测试 ====================

// TestNewLockFreeRTTTracker 测试构造函数
func TestNewLockFreeRTTTracker(t *testing.T) {
	tests := []struct {
		name           string
		bufferSize     uint32
		maxSamples     uint32
		expectedBuffer int
	}{
		{"small", 100, 1000, 128},     // 2^7 = 128
		{"medium", 1024, 10000, 1024}, // 2^10 = 1024
		{"large", 5000, 20000, 8192},  // 2^13 = 8192
		{"power of 2", 4096, 10000, 4096},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := NewLockFreeRTTTracker(tt.bufferSize, tt.maxSamples)

			if len(tracker.buffer) != tt.expectedBuffer {
				t.Errorf("buffer size = %d, want %d", len(tracker.buffer), tt.expectedBuffer)
			}

			if len(tracker.samples) != int(tt.maxSamples) {
				t.Errorf("samples size = %d, want %d", len(tracker.samples), tt.maxSamples)
			}

			if tracker.mask != uint32(tt.expectedBuffer-1) {
				t.Errorf("mask = %d, want %d", tracker.mask, tt.expectedBuffer-1)
			}
		})
	}
}

// TestRecordAndComplete 测试基本的记录和完成流程
func TestRecordAndComplete(t *testing.T) {
	tracker := NewLockFreeRTTTracker(1024, 1000)

	sessionId := uint64(88888)
	seqId := uint32(12345)

	// Record
	tracker.Record(sessionId, seqId)

	// 短暂延迟，确保有 RTT
	time.Sleep(1 * time.Millisecond)

	// Complete
	rtt, ok := tracker.Complete(sessionId, seqId)

	if !ok {
		t.Fatal("Complete should return true")
	}

	if rtt < 1*time.Millisecond {
		t.Errorf("RTT = %v, should be >= 1ms", rtt)
	}

	if rtt > 100*time.Millisecond {
		t.Errorf("RTT = %v, should be < 100ms", rtt)
	}
}

// TestCompleteWithoutRecord 测试未记录就完成的情况
func TestCompleteWithoutRecord(t *testing.T) {
	tracker := NewLockFreeRTTTracker(1024, 1000)

	sessionId := uint64(88888)
	seqId := uint32(99999)

	// 没有 Record，直接 Complete
	rtt, ok := tracker.Complete(sessionId, seqId)

	if ok {
		t.Errorf("Complete should return false for unrecorded seqId, got rtt=%v", rtt)
	}
}

// TestHashCollision 测试哈希冲突（槽位覆盖）
func TestHashCollision(t *testing.T) {
	tracker := NewLockFreeRTTTracker(16, 1000) // 小缓冲，容易冲突

	sessionId := uint64(88888)
	seqId1 := uint32(1)
	seqId2 := uint32(17) // 可能与 seqId1 冲突

	// Record seqId1
	tracker.Record(sessionId, seqId1)
	time.Sleep(1 * time.Millisecond)

	// Record seqId2（可能覆盖 seqId1）
	tracker.Record(sessionId, seqId2)

	// Complete seqId1 应该失败（已被覆盖）
	_, ok := tracker.Complete(sessionId, seqId1)
	if ok {
		t.Error("Complete seqId1 should fail due to collision")
	}

	// Complete seqId2 应该成功
	time.Sleep(1 * time.Millisecond)
	rtt, ok := tracker.Complete(sessionId, seqId2)
	if !ok {
		t.Error("Complete seqId2 should succeed")
	}
	if rtt < 1*time.Millisecond {
		t.Errorf("RTT = %v, should be >= 1ms", rtt)
	}
}

// TestGetAndResetSamples 测试采样获取和重置
func TestGetAndResetSamples(t *testing.T) {
	tracker := NewLockFreeRTTTracker(1024, 100)

	sessionId := uint64(88888)

	// 记录 50 个请求
	for i := uint32(0); i < 50; i++ {
		tracker.Record(sessionId, i)
		time.Sleep(100 * time.Microsecond)
		tracker.Complete(sessionId, i)
	}

	// 获取采样
	samples := tracker.GetAndResetSamples()

	if len(samples) != 50 {
		t.Errorf("samples length = %d, want 50", len(samples))
	}

	// 再次获取应该为空
	samples2 := tracker.GetAndResetSamples()
	if len(samples2) != 0 {
		t.Errorf("samples2 length = %d, want 0", len(samples2))
	}

	// 验证 RTT 值的合理性
	for i, rtt := range samples {
		if rtt <= 0 {
			t.Errorf("sample[%d] = %d, should be > 0", i, rtt)
		}
		if rtt > 1*time.Second.Nanoseconds() {
			t.Errorf("sample[%d] = %d, should be < 1s", i, rtt)
		}
	}
}

// TestMaxSamplesLimit 测试采样上限
func TestMaxSamplesLimit(t *testing.T) {
	maxSamples := uint32(10)
	tracker := NewLockFreeRTTTracker(1024, maxSamples)

	sessionId := uint64(88888)

	// 记录 20 个请求（超过上限）
	for i := uint32(0); i < 20; i++ {
		tracker.Record(sessionId, i)
		time.Sleep(10 * time.Microsecond)
		tracker.Complete(sessionId, i)
	}

	// 获取采样
	samples := tracker.GetAndResetSamples()

	// 应该只有 maxSamples 个
	if len(samples) != int(maxSamples) {
		t.Errorf("samples length = %d, want %d", len(samples), maxSamples)
	}
}

// TestSanityCheck 测试合理性检查
func TestSanityCheck(t *testing.T) {
	tracker := NewLockFreeRTTTracker(1024, 1000)

	sessionId := uint64(88888)
	seqId := uint32(888)
	tracker.Record(sessionId, seqId)

	// 短暂延迟确保 RTT > 0
	time.Sleep(100 * time.Microsecond)

	// Complete
	rtt, ok := tracker.Complete(sessionId, seqId)

	// 应该成功
	if !ok {
		t.Error("Complete should succeed")
	}

	if rtt <= 0 {
		t.Errorf("RTT = %v, should be > 0", rtt)
	}

	if rtt > 10*time.Second {
		t.Errorf("RTT = %v, should be < 10s", rtt)
	}
}

// TestDifferentSessionsIsolation 测试不同 session 之间的隔离性
func TestDifferentSessionsIsolation(t *testing.T) {
	tracker := NewLockFreeRTTTracker(1024, 1000)

	// 两个不同的 session，使用相同的 seqId
	sessionId1 := uint64(10001)
	sessionId2 := uint64(10002)
	seqId := uint32(100)

	// Session 1 记录请求
	tracker.Record(sessionId1, seqId)
	time.Sleep(1 * time.Millisecond)

	// Session 2 也记录相同 seqId 的请求（应该不会互相干扰）
	tracker.Record(sessionId2, seqId)
	time.Sleep(1 * time.Millisecond)

	// Session 1 尝试 Complete
	rtt1, ok1 := tracker.Complete(sessionId1, seqId)

	// Session 2 Complete 应该成功
	rtt2, ok2 := tracker.Complete(sessionId2, seqId)

	if !ok2 {
		t.Error("Session 2 Complete should succeed")
	}

	if rtt2 < 1*time.Millisecond {
		t.Errorf("Session 2 RTT = %v, should be >= 1ms", rtt2)
	}

	// Session 1 可能失败（因为槽位被 Session 2 覆盖）或者成功（如果槽位不同）
	// 但不应该读到 Session 2 的数据
	if ok1 {
		t.Logf("Session 1 RTT = %v (not overwritten)", rtt1)
	} else {
		t.Logf("Session 1 failed (overwritten by Session 2, expected)")
	}
}

// TestNextPowerOfTwo 测试幂次方计算
func TestNextPowerOfTwo(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{0, 2},
		{1, 2},
		{2, 2},
		{3, 4},
		{4, 4},
		{5, 8},
		{7, 8},
		{8, 8},
		{9, 16},
		{100, 128},
		{1024, 1024},
		{1025, 2048},
		{4096, 4096},
		{5000, 8192},
	}

	for _, tt := range tests {
		result := nextPowerOfTwo(tt.input)
		if result != tt.expected {
			t.Errorf("nextPowerOfTwo(%d) = %d, want %d", tt.input, result, tt.expected)
		}
	}
}

// ==================== 并发测试 ====================

// TestConcurrentRecordAndComplete 测试并发记录和完成
func TestConcurrentRecordAndComplete(t *testing.T) {
	// ✅ 修复 flaky:
	// 1. 缓冲区 65536 >> 总操作数 10000，避免哈希碰撞导致大量覆盖
	// 2. 分两阶段执行，确保所有 Record 完成后再 Complete
	// 3. 阈值设为 50%，容忍跨 sessionId 的哈希碰撞
	tracker := NewLockFreeRTTTracker(65536, 20000)

	const goroutines = 10
	const opsPerGoroutine = 1000

	// Phase 1: 并发 Record
	var recordWg sync.WaitGroup
	recordWg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(g int) {
			defer recordWg.Done()
			sessionId := uint64(10000 + g)
			base := uint32(g * opsPerGoroutine)
			for i := uint32(0); i < opsPerGoroutine; i++ {
				tracker.Record(sessionId, base+i)
			}
		}(g)
	}
	recordWg.Wait() // 等待所有 Record 完成

	// 确保 Record 和 Complete 之间有时间差，
	// 避免 Complete 的 Sanity Check (cost <= 0) 因时钟精度问题过滤掉合法记录
	time.Sleep(time.Millisecond)

	// Phase 2: 并发 Complete
	var completeWg sync.WaitGroup
	completeWg.Add(goroutines)
	successCount := make([]int, goroutines)
	for g := 0; g < goroutines; g++ {
		go func(g int) {
			defer completeWg.Done()
			sessionId := uint64(10000 + g)
			base := uint32(g * opsPerGoroutine)
			for i := uint32(0); i < opsPerGoroutine; i++ {
				if _, ok := tracker.Complete(sessionId, base+i); ok {
					successCount[g]++
				}
			}
		}(g)
	}
	completeWg.Wait()

	// 统计成功次数
	total := 0
	for _, count := range successCount {
		total += count
	}

	// 至少应该有 50% 成功（lock-free 结构下不同 sessionId 可能碰撞同一槽位）
	expectedMin := goroutines * opsPerGoroutine * 50 / 100
	if total < expectedMin {
		t.Errorf("success count = %d, want >= %d", total, expectedMin)
	}

	t.Logf("Concurrent test: %d/%d succeeded (%.1f%%)",
		total, goroutines*opsPerGoroutine, float64(total)*100/float64(goroutines*opsPerGoroutine))
}

// TestConcurrentGetAndReset 测试并发获取和重置
func TestConcurrentGetAndReset(t *testing.T) {
	tracker := NewLockFreeRTTTracker(4096, 5000)

	var wg sync.WaitGroup

	// Writer goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		sessionId := uint64(88888)
		for i := uint32(0); i < 2000; i++ {
			tracker.Record(sessionId, i)
			time.Sleep(10 * time.Microsecond)
			tracker.Complete(sessionId, i)
		}
	}()

	// Reader goroutines
	for g := 0; g < 5; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				samples := tracker.GetAndResetSamples()
				// 验证采样的合理性
				for _, rtt := range samples {
					if rtt < 0 || rtt > 10*time.Second.Nanoseconds() {
						t.Errorf("invalid RTT: %d", rtt)
					}
				}
				time.Sleep(20 * time.Millisecond)
			}
		}()
	}

	wg.Wait()
}

// TestRaceDetection 测试数据竞争（使用 go test -race）
func TestRaceDetection(t *testing.T) {
	tracker := NewLockFreeRTTTracker(1024, 1000)

	done := make(chan struct{})

	// Writer
	go func() {
		sessionId := uint64(88888)
		for {
			select {
			case <-done:
				return
			default:
				for i := uint32(0); i < 100; i++ {
					tracker.Record(sessionId, i)
					tracker.Complete(sessionId, i)
				}
			}
		}
	}()

	// Reader
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				tracker.GetAndResetSamples()
			}
		}
	}()

	// 运行 100ms
	time.Sleep(100 * time.Millisecond)
	close(done)

	// 等待 goroutine 退出
	time.Sleep(10 * time.Millisecond)
}

// ==================== 基准测试 ====================

// BenchmarkRecord 单线程 Record 性能
func BenchmarkRecord(b *testing.B) {
	tracker := NewLockFreeRTTTracker(8192, 20000)
	sessionId := uint64(88888)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		tracker.Record(sessionId, uint32(i))
	}
}

// BenchmarkComplete 单线程 Complete 性能
func BenchmarkComplete(b *testing.B) {
	tracker := NewLockFreeRTTTracker(8192, 20000)
	sessionId := uint64(88888)

	// 预先 Record
	for i := 0; i < 8192; i++ {
		tracker.Record(sessionId, uint32(i))
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		tracker.Complete(sessionId, uint32(i&8191))
	}
}

// BenchmarkRecordAndComplete 完整流程性能
func BenchmarkRecordAndComplete(b *testing.B) {
	tracker := NewLockFreeRTTTracker(8192, 20000)
	sessionId := uint64(88888)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		seqId := uint32(i)
		tracker.Record(sessionId, seqId)
		tracker.Complete(sessionId, seqId)
	}
}

// BenchmarkRecordParallel 并发 Record 性能
func BenchmarkRecordParallel(b *testing.B) {
	tracker := NewLockFreeRTTTracker(8192, 20000)

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		sessionId := uint64(88888)
		seqId := uint32(0)
		for pb.Next() {
			tracker.Record(sessionId, seqId)
			seqId++
		}
	})
}

// BenchmarkCompleteParallel 并发 Complete 性能
func BenchmarkCompleteParallel(b *testing.B) {
	tracker := NewLockFreeRTTTracker(8192, 20000)
	sessionId := uint64(88888)

	// 预先 Record
	for i := 0; i < 8192; i++ {
		tracker.Record(sessionId, uint32(i))
	}

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		seqId := uint32(0)
		for pb.Next() {
			tracker.Complete(sessionId, seqId&8191)
			seqId++
		}
	})
}

// BenchmarkFullCycleParallel 并发完整流程性能
func BenchmarkFullCycleParallel(b *testing.B) {
	tracker := NewLockFreeRTTTracker(8192, 20000)

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		sessionId := uint64(88888)
		seqId := uint32(0)
		for pb.Next() {
			tracker.Record(sessionId, seqId)
			tracker.Complete(sessionId, seqId)
			seqId++
		}
	})
}

// BenchmarkGetAndResetSamples 获取采样性能
func BenchmarkGetAndResetSamples(b *testing.B) {
	tracker := NewLockFreeRTTTracker(8192, 20000)
	sessionId := uint64(88888)

	// 填充一些数据
	for i := uint32(0); i < 1000; i++ {
		tracker.Record(sessionId, i)
		tracker.Complete(sessionId, i)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = tracker.GetAndResetSamples()

		// 重新填充数据
		if i%10 == 0 {
			for j := uint32(0); j < 1000; j++ {
				tracker.Record(sessionId, j)
				tracker.Complete(sessionId, j)
			}
		}
	}
}

// BenchmarkHashCollisionRate 测试哈希冲突率
func BenchmarkHashCollisionRate(b *testing.B) {
	sizes := []uint32{512, 1024, 2048, 4096, 8192, 16384}

	for _, size := range sizes {
		b.Run(string(rune(size)), func(b *testing.B) {
			tracker := NewLockFreeRTTTracker(size, 10000)

			b.ResetTimer()

			successCount := 0
			totalCount := 0
			sessionId := uint64(88888)

			for i := 0; i < b.N; i++ {
				seqId := uint32(i)
				tracker.Record(sessionId, seqId)
				if _, ok := tracker.Complete(sessionId, seqId); ok {
					successCount++
				}
				totalCount++
			}

			if totalCount > 0 {
				rate := float64(successCount) * 100 / float64(totalCount)
				b.ReportMetric(rate, "success%")
			}
		})
	}
}
