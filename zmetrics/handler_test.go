package zmetrics

import (
	"strings"
	"sync"
	"testing"
	"time"
)

func resetHandlerMetrics() {
	handlerMu.Lock()
	handlerMap = make(map[handlerKey]*HandlerMetric)
	handlerEntries = nil
	handlerSorted = false
	handlerMu.Unlock()
}

func TestGetHandlerMetric_Basic(t *testing.T) {
	resetHandlerMetrics()

	m := GetHandlerMetric(1, 100, 1001)
	if m == nil {
		t.Fatal("expected non-nil HandlerMetric")
	}

	m2 := GetHandlerMetric(1, 100, 1001)
	if m != m2 {
		t.Error("expected same pointer for same (actorId, msgId)")
	}

	m3 := GetHandlerMetric(2, 100, 1001)
	if m == m3 {
		t.Error("expected different pointer for different actorId")
	}

	m4 := GetHandlerMetric(1, 100, 1002)
	if m == m4 {
		t.Error("expected different pointer for different msgId")
	}
}

func TestGetHandlerMetric_SameActorIdDifferentType(t *testing.T) {
	resetHandlerMetrics()

	m1 := GetHandlerMetric(1, 100, 1001)
	m2 := GetHandlerMetric(1, 200, 1001)

	if m1 != m2 {
		t.Error("actorType should not affect key lookup — same (actorId, msgId) returns same metric")
	}
}

func TestHandlerMetric_RecordCall(t *testing.T) {
	resetHandlerMetrics()

	m := GetHandlerMetric(1, 100, 1001)

	m.RecordCall(5 * time.Millisecond)
	m.RecordCall(15 * time.Millisecond)
	m.RecordCall(3 * time.Millisecond)

	if got := m.total.Load(); got != 3 {
		t.Errorf("total: want 3, got %d", got)
	}

	if got := m.slow.Load(); got != 1 {
		t.Errorf("slow: want 1 (only 15ms > 10ms), got %d", got)
	}
}

func TestHandlerMetric_RecordCall_SlowThreshold(t *testing.T) {
	resetHandlerMetrics()
	m := GetHandlerMetric(1, 100, 2001)

	m.RecordCall(10 * time.Millisecond)
	if got := m.slow.Load(); got != 0 {
		t.Errorf("10ms should NOT be slow (threshold is >10ms), got slow=%d", got)
	}

	m.RecordCall(10*time.Millisecond + time.Microsecond)
	if got := m.slow.Load(); got != 1 {
		t.Errorf("10.001ms should be slow, got slow=%d", got)
	}
}

func TestWriteHandlerPrometheus_Empty(t *testing.T) {
	resetHandlerMetrics()
	var b strings.Builder
	WriteHandlerPrometheus(&b)
	if b.Len() != 0 {
		t.Errorf("expected empty output for no metrics, got %q", b.String())
	}
}

func TestWriteHandlerPrometheus_Format(t *testing.T) {
	resetHandlerMetrics()

	m := GetHandlerMetric(1, 100, 1001)
	m.RecordCall(5 * time.Millisecond)
	m.RecordCall(50 * time.Millisecond)

	var b strings.Builder
	WriteHandlerPrometheus(&b)
	output := b.String()

	mustContain := []string{
		"# HELP zhenyi_handler_total",
		"# TYPE zhenyi_handler_total counter",
		`zhenyi_handler_total{handler="1001",actor_id="1",actor_type="100"}`,
		"# HELP zhenyi_handler_slow_total",
		`zhenyi_handler_slow_total{handler="1001",actor_id="1",actor_type="100"}`,
		"# HELP zhenyi_handler_latency_ms",
		"# TYPE zhenyi_handler_latency_ms histogram",
		"zhenyi_handler_latency_ms_bucket",
		`le="+Inf"`,
		"zhenyi_handler_latency_ms_sum",
		"zhenyi_handler_latency_ms_count",
	}

	for _, s := range mustContain {
		if !strings.Contains(output, s) {
			t.Errorf("output missing %q\n\nFull output:\n%s", s, output)
		}
	}
}

func TestWriteHandlerPrometheus_Sorted(t *testing.T) {
	resetHandlerMetrics()

	GetHandlerMetric(3, 100, 2001).RecordCall(time.Millisecond)
	GetHandlerMetric(1, 100, 1001).RecordCall(time.Millisecond)
	GetHandlerMetric(2, 100, 1001).RecordCall(time.Millisecond)

	var b strings.Builder
	WriteHandlerPrometheus(&b)
	output := b.String()

	idx1 := strings.Index(output, `actor_id="1"`)
	idx2 := strings.Index(output, `actor_id="2"`)
	idx3 := strings.Index(output, `actor_id="3"`)

	if idx1 > idx2 || idx2 > idx3 {
		t.Errorf("entries should be sorted by actorId: idx1=%d, idx2=%d, idx3=%d", idx1, idx2, idx3)
	}
}

func TestWriteHandlerPrometheus_SkipZeroCounters(t *testing.T) {
	resetHandlerMetrics()

	_ = GetHandlerMetric(1, 100, 1001)

	var b strings.Builder
	WriteHandlerPrometheus(&b)
	output := b.String()

	if strings.Contains(output, `zhenyi_handler_total{handler="1001"`) {
		t.Error("zero-count handler should not appear in counter output")
	}
}

func TestGetHandlerMetric_ConcurrentAccess(t *testing.T) {
	resetHandlerMetrics()

	var wg sync.WaitGroup
	results := make([]*HandlerMetric, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx] = GetHandlerMetric(1, 100, 1001)
		}(i)
	}
	wg.Wait()

	for i := 1; i < 100; i++ {
		if results[i] != results[0] {
			t.Fatalf("concurrent GetHandlerMetric returned different pointers at index %d", i)
		}
	}
}
