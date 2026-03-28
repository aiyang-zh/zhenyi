package zgate

import (
	"testing"
	"time"
)

// ============================================================
// EncodeReqKey 单元测试
// ============================================================

func TestEncodeReqKey_Basic(t *testing.T) {
	key := EncodeReqKey(1, 2)
	if key == 0 {
		t.Error("key should not be 0")
	}
}

func TestEncodeReqKey_Uniqueness(t *testing.T) {
	// 不同 (sessionId, seqId) 应产生不同的 key
	k1 := EncodeReqKey(1, 1)
	k2 := EncodeReqKey(1, 2)
	k3 := EncodeReqKey(2, 1)

	if k1 == k2 {
		t.Error("different seqId should produce different keys")
	}
	if k1 == k3 {
		t.Error("different sessionId should produce different keys")
	}
}

func TestEncodeReqKey_BitLayout(t *testing.T) {
	// 高32位 = sessionId低32位，低32位 = seqId
	key := EncodeReqKey(0x00000001, 0x00000002)
	high := key >> 32
	low := key & 0xFFFFFFFF
	if high != 1 {
		t.Errorf("high 32 bits should be 1, got %d", high)
	}
	if low != 2 {
		t.Errorf("low 32 bits should be 2, got %d", low)
	}
}

func TestEncodeReqKey_MaxValues(t *testing.T) {
	key := EncodeReqKey(0xFFFFFFFF, 0xFFFFFFFF)
	if key != 0x00000000FFFFFFFF<<32|0xFFFFFFFF {
		t.Errorf("unexpected key for max values: %x", key)
	}
}

// ============================================================
// CalculateStats 单元测试
// ============================================================

func TestCalculateStats_Empty(t *testing.T) {
	stats := CalculateStats(nil)
	if stats.Count != 0 {
		t.Error("empty samples should return count 0")
	}
}

func TestCalculateStats_Single(t *testing.T) {
	samples := []int64{1000000} // 1ms
	stats := CalculateStats(samples)

	if stats.Count != 1 {
		t.Errorf("expected count 1, got %d", stats.Count)
	}
	if stats.Min != time.Millisecond {
		t.Errorf("expected min 1ms, got %v", stats.Min)
	}
	if stats.Max != time.Millisecond {
		t.Errorf("expected max 1ms, got %v", stats.Max)
	}
	if stats.Avg != time.Millisecond {
		t.Errorf("expected avg 1ms, got %v", stats.Avg)
	}
}

func TestCalculateStats_Multiple(t *testing.T) {
	// 10 个样本: 1ms ~ 10ms
	samples := make([]int64, 10)
	for i := range samples {
		samples[i] = int64((i + 1) * 1000000) // ms -> ns
	}
	stats := CalculateStats(samples)

	if stats.Count != 10 {
		t.Errorf("expected 10, got %d", stats.Count)
	}
	if stats.Min != time.Millisecond {
		t.Errorf("expected min 1ms, got %v", stats.Min)
	}
	if stats.Max != 10*time.Millisecond {
		t.Errorf("expected max 10ms, got %v", stats.Max)
	}
	// Avg = (1+2+...+10)/10 = 5.5ms
	expectedAvg := time.Duration(5500000) // 5.5ms
	if stats.Avg != expectedAvg {
		t.Errorf("expected avg 5.5ms, got %v", stats.Avg)
	}
	// P50 should be around 5-6ms
	if stats.P50 < 4*time.Millisecond || stats.P50 > 7*time.Millisecond {
		t.Errorf("P50 unexpected: %v", stats.P50)
	}
}

func TestCalculateStats_Sorted(t *testing.T) {
	// 乱序输入应被排序
	samples := []int64{5000000, 1000000, 3000000, 2000000, 4000000}
	stats := CalculateStats(samples)

	if stats.Min != time.Millisecond {
		t.Errorf("expected min 1ms, got %v", stats.Min)
	}
	if stats.Max != 5*time.Millisecond {
		t.Errorf("expected max 5ms, got %v", stats.Max)
	}
}

// ============================================================
// ServerMetrics.RecordRTT 单元测试
// ============================================================

func TestServerMetrics_RecordRTT(t *testing.T) {
	m := &ServerMetrics{
		RTTTracker: NewLockFreeRTTTracker(1024, 10000),
	}

	m.RecordRTT(5 * time.Millisecond)
	m.RecordRTT(10 * time.Millisecond)

	totalNs := m.GlobalTotalRTT.Load()
	totalCount := m.GlobalCountRTT.Load()

	if totalCount != 2 {
		t.Errorf("expected count 2, got %d", totalCount)
	}
	expectedNs := int64(15 * time.Millisecond)
	if totalNs != expectedNs {
		t.Errorf("expected total %d ns, got %d", expectedNs, totalNs)
	}
}

// ============================================================
// 基准测试
// ============================================================

func BenchmarkEncodeReqKey(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		EncodeReqKey(uint64(i), uint32(i))
	}
}

func BenchmarkCalculateStats_100(b *testing.B) {
	samples := make([]int64, 100)
	for i := range samples {
		samples[i] = int64(i * 1000000)
	}
	s := make([]int64, len(samples))
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		// 需要拷贝因为 CalculateStats 会排序：复用 buffer，避免基准本身引入分配
		copy(s, samples)
		CalculateStats(s)
	}
}

func BenchmarkServerMetrics_RecordRTT(b *testing.B) {
	m := &ServerMetrics{
		RTTTracker: NewLockFreeRTTTracker(1024, 10000),
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		m.RecordRTT(time.Millisecond)
	}
}
