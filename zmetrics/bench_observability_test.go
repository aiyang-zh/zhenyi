package zmetrics

import (
	"testing"
	"time"
)

// BenchmarkHandlerMetric_RecordCall measures the hot-path cost of per-handler metrics.
// Target: < 10 ns/op, 0 allocs/op
func BenchmarkHandlerMetric_RecordCall(b *testing.B) {
	resetHandlerMetrics()
	m := GetHandlerMetric(1, 100, 1001)
	cost := 5 * time.Millisecond

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.RecordCall(cost)
	}
}

// BenchmarkHandlerMetric_RecordCall_Parallel measures concurrent recording.
func BenchmarkHandlerMetric_RecordCall_Parallel(b *testing.B) {
	resetHandlerMetrics()
	m := GetHandlerMetric(1, 100, 1001)
	cost := 5 * time.Millisecond

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			m.RecordCall(cost)
		}
	})
}
