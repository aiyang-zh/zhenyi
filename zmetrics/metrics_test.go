package zmetrics

import (
	"math"
	"strings"
	"sync"
	"testing"
	"time"
)

// ==================== Counter Tests ====================

func TestCounter_Inc(t *testing.T) {
	var c Counter
	c.Inc()
	c.Inc()
	c.Inc()
	if got := c.Load(); got != 3 {
		t.Errorf("Counter.Inc: want 3, got %d", got)
	}
}

func TestCounter_Add(t *testing.T) {
	var c Counter
	c.Add(10)
	c.Add(20)
	if got := c.Load(); got != 30 {
		t.Errorf("Counter.Add: want 30, got %d", got)
	}
}

func TestCounter_Swap(t *testing.T) {
	var c Counter
	c.Add(42)
	old := c.Swap(0)
	if old != 42 {
		t.Errorf("Counter.Swap return: want 42, got %d", old)
	}
	if got := c.Load(); got != 0 {
		t.Errorf("Counter after Swap: want 0, got %d", got)
	}
}

func TestCounter_Concurrent(t *testing.T) {
	var c Counter
	var wg sync.WaitGroup
	n := 1000
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Inc()
		}()
	}
	wg.Wait()
	if got := c.Load(); got != int64(n) {
		t.Errorf("Counter concurrent Inc: want %d, got %d", n, got)
	}
}

// ==================== Gauge Tests ====================

func TestGauge_SetAndLoad(t *testing.T) {
	var g Gauge
	g.Set(100)
	if got := g.Load(); got != 100 {
		t.Errorf("Gauge.Set/Load: want 100, got %d", got)
	}
}

func TestGauge_Inc(t *testing.T) {
	var g Gauge
	g.Inc()
	g.Inc()
	if got := g.Load(); got != 2 {
		t.Errorf("Gauge.Inc: want 2, got %d", got)
	}
}

func TestGauge_Dec(t *testing.T) {
	var g Gauge
	g.Set(5)
	g.Dec()
	g.Dec()
	if got := g.Load(); got != 3 {
		t.Errorf("Gauge.Dec: want 3, got %d", got)
	}
}

func TestGauge_Add(t *testing.T) {
	var g Gauge
	g.Add(10)
	g.Add(-3)
	if got := g.Load(); got != 7 {
		t.Errorf("Gauge.Add: want 7, got %d", got)
	}
}

func TestGauge_SetFloat_LoadFloat(t *testing.T) {
	var g Gauge
	g.SetFloat(3.14159)
	got := g.LoadFloat()
	if math.Abs(got-3.14159) > 1e-10 {
		t.Errorf("Gauge.SetFloat/LoadFloat: want 3.14159, got %f", got)
	}
}

func TestGauge_SetFloat_Negative(t *testing.T) {
	var g Gauge
	g.SetFloat(-273.15)
	got := g.LoadFloat()
	if math.Abs(got-(-273.15)) > 1e-10 {
		t.Errorf("Gauge.SetFloat negative: want -273.15, got %f", got)
	}
}

func TestGauge_Concurrent(t *testing.T) {
	var g Gauge
	var wg sync.WaitGroup
	n := 500
	for i := 0; i < n; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); g.Inc() }()
		go func() { defer wg.Done(); g.Dec() }()
	}
	wg.Wait()
	if got := g.Load(); got != 0 {
		t.Errorf("Gauge concurrent Inc+Dec: want 0, got %d", got)
	}
}

// ==================== Histogram Tests ====================

func TestNewHistogram(t *testing.T) {
	bounds := []float64{1, 5, 10}
	h := NewHistogram(bounds)
	if h == nil {
		t.Fatal("NewHistogram returned nil")
	}
	if len(h.buckets) != 4 {
		t.Errorf("bucket count: want 4 (3 bounds + Inf), got %d", len(h.buckets))
	}
}

func TestNewLatencyHistogram(t *testing.T) {
	h := NewLatencyHistogram()
	if h == nil {
		t.Fatal("NewLatencyHistogram returned nil")
	}
	if len(h.bounds) != len(DefaultLatencyBounds) {
		t.Errorf("bounds count: want %d, got %d", len(DefaultLatencyBounds), len(h.bounds))
	}
}

func TestHistogram_Observe_BucketPlacement(t *testing.T) {
	h := NewHistogram([]float64{1, 5, 10})

	h.Observe(0.5) // bucket 0: [0, 1)
	h.Observe(3)   // bucket 1: [1, 5)
	h.Observe(5)   // bucket 1: le=5 includes exact boundary (Prometheus convention)
	h.Observe(7)   // bucket 2: (5, 10]
	h.Observe(100) // bucket 3: +Inf

	bounds, cumulative, _, count := h.Snapshot()
	if count != 5 {
		t.Errorf("count: want 5, got %d", count)
	}

	expected := map[float64]int64{
		1:  1, // cumulative: le=1 → 1
		5:  3, // cumulative: le=5 → 1+1+1=3 (includes exact 5)
		10: 4, // cumulative: le=10 → 3+1=4
	}
	for i, bound := range bounds {
		if want, ok := expected[bound]; ok {
			if cumulative[i] != want {
				t.Errorf("cumulative[le=%.0f]: want %d, got %d", bound, want, cumulative[i])
			}
		}
	}
	// +Inf bucket
	if cumulative[len(cumulative)-1] != 5 {
		t.Errorf("cumulative[+Inf]: want 5, got %d", cumulative[len(cumulative)-1])
	}
}

func TestHistogram_Observe_ExactBoundary(t *testing.T) {
	h := NewHistogram([]float64{10})
	h.Observe(10)
	_, cumulative, _, _ := h.Snapshot()
	// Prometheus convention: le=10 includes exact value 10
	if cumulative[0] != 1 {
		t.Errorf("le=10 cumulative: want 1 (exact match included per Prometheus le convention), got %d", cumulative[0])
	}
	if cumulative[1] != 1 {
		t.Errorf("le=+Inf cumulative: want 1, got %d", cumulative[1])
	}
}

func TestHistogram_ObserveDuration(t *testing.T) {
	h := NewHistogram([]float64{1, 5, 10})
	h.ObserveDuration(3 * time.Millisecond)
	_, _, _, count := h.Snapshot()
	if count != 1 {
		t.Errorf("ObserveDuration count: want 1, got %d", count)
	}
}

func TestHistogram_Snapshot_Sum(t *testing.T) {
	h := NewHistogram([]float64{10, 100})
	h.Observe(5)
	h.Observe(20)
	_, _, sum, _ := h.Snapshot()
	if math.Abs(sum-25) > 0.01 {
		t.Errorf("snapshot sum: want 25, got %f", sum)
	}
}

func TestHistogram_Concurrent(t *testing.T) {
	h := NewHistogram([]float64{1, 5, 10})
	var wg sync.WaitGroup
	n := 1000
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(v int) {
			defer wg.Done()
			h.Observe(float64(v % 15))
		}(i)
	}
	wg.Wait()
	_, _, _, count := h.Snapshot()
	if count != int64(n) {
		t.Errorf("Histogram concurrent: want count %d, got %d", n, count)
	}
}

// ==================== Registry Tests ====================

func newTestRegistry() *Registry {
	return &Registry{
		counters:   make(map[string]*counterEntry),
		gauges:     make(map[string]*gaugeEntry),
		histograms: make(map[string]*histogramEntry),
	}
}

func TestGlobal(t *testing.T) {
	r := Global()
	if r == nil {
		t.Fatal("Global() returned nil")
	}
	if r != globalRegistry {
		t.Error("Global() should return the globalRegistry singleton")
	}
}

func TestRegistry_Counter(t *testing.T) {
	r := newTestRegistry()
	c1 := r.Counter("test_counter", "a test counter")
	if c1 == nil {
		t.Fatal("Registry.Counter returned nil")
	}
	c2 := r.Counter("test_counter", "different help")
	if c1 != c2 {
		t.Error("expected same Counter for same name")
	}
	c3 := r.Counter("other_counter", "another")
	if c1 == c3 {
		t.Error("expected different Counter for different name")
	}
}

func TestRegistry_Gauge(t *testing.T) {
	r := newTestRegistry()
	g1 := r.Gauge("test_gauge", "a test gauge")
	if g1 == nil {
		t.Fatal("Registry.Gauge returned nil")
	}
	g2 := r.Gauge("test_gauge", "")
	if g1 != g2 {
		t.Error("expected same Gauge for same name")
	}
}

func TestRegistry_Histogram(t *testing.T) {
	r := newTestRegistry()
	h1 := r.Histogram("test_hist", "a test histogram", []float64{1, 5, 10})
	if h1 == nil {
		t.Fatal("Registry.Histogram returned nil")
	}
	h2 := r.Histogram("test_hist", "", nil)
	if h1 != h2 {
		t.Error("expected same Histogram for same name")
	}
}

func resetAllGlobalState() {
	resetHandlerMetrics()
}

func TestRegistry_WritePrometheus_Counters(t *testing.T) {
	resetAllGlobalState()
	r := newTestRegistry()
	c := r.Counter("zhenyi_test_total", "Test counter help")
	c.Inc()
	c.Inc()

	var b strings.Builder
	r.WritePrometheus(&b)
	output := b.String()

	mustContain := []string{
		"# HELP zhenyi_test_total Test counter help",
		"# TYPE zhenyi_test_total counter",
		"zhenyi_test_total 2",
	}
	for _, s := range mustContain {
		if !strings.Contains(output, s) {
			t.Errorf("counter output missing %q\n\nFull:\n%s", s, output)
		}
	}
}

func TestRegistry_WritePrometheus_Gauges(t *testing.T) {
	resetAllGlobalState()
	r := newTestRegistry()
	g := r.Gauge("zhenyi_test_connections", "Active connections")
	g.Set(42)

	var b strings.Builder
	r.WritePrometheus(&b)
	output := b.String()

	mustContain := []string{
		"# HELP zhenyi_test_connections Active connections",
		"# TYPE zhenyi_test_connections gauge",
		"zhenyi_test_connections 42",
	}
	for _, s := range mustContain {
		if !strings.Contains(output, s) {
			t.Errorf("gauge output missing %q\n\nFull:\n%s", s, output)
		}
	}
}

func TestRegistry_WritePrometheus_Histograms(t *testing.T) {
	resetAllGlobalState()
	r := newTestRegistry()
	h := r.Histogram("zhenyi_test_latency_ms", "Latency", []float64{1, 5, 10})
	h.Observe(3)
	h.Observe(7)

	var b strings.Builder
	r.WritePrometheus(&b)
	output := b.String()

	mustContain := []string{
		"# HELP zhenyi_test_latency_ms Latency",
		"# TYPE zhenyi_test_latency_ms histogram",
		`zhenyi_test_latency_ms_bucket{le="1"} 0`,
		`zhenyi_test_latency_ms_bucket{le="5"} 1`,
		`zhenyi_test_latency_ms_bucket{le="10"} 2`,
		`zhenyi_test_latency_ms_bucket{le="+Inf"} 2`,
		"zhenyi_test_latency_ms_count 2",
	}
	for _, s := range mustContain {
		if !strings.Contains(output, s) {
			t.Errorf("histogram output missing %q\n\nFull:\n%s", s, output)
		}
	}
}

func TestRegistry_WritePrometheus_NoHelp(t *testing.T) {
	resetAllGlobalState()
	r := newTestRegistry()
	c := r.Counter("no_help_counter", "")
	c.Inc()

	var b strings.Builder
	r.WritePrometheus(&b)
	output := b.String()

	if strings.Contains(output, "# HELP no_help_counter") {
		t.Error("should not emit HELP line for empty help string")
	}
	if !strings.Contains(output, "# TYPE no_help_counter counter") {
		t.Error("should still emit TYPE line")
	}
}

func TestRegistry_WritePrometheus_Sorted(t *testing.T) {
	resetAllGlobalState()
	r := newTestRegistry()
	r.Counter("zzz_counter", "").Inc()
	r.Counter("aaa_counter", "").Inc()
	r.Counter("mmm_counter", "").Inc()

	var b strings.Builder
	r.WritePrometheus(&b)
	output := b.String()

	idxA := strings.Index(output, "aaa_counter")
	idxM := strings.Index(output, "mmm_counter")
	idxZ := strings.Index(output, "zzz_counter")

	if idxA > idxM || idxM > idxZ {
		t.Errorf("counters should be alphabetically sorted: aaa=%d, mmm=%d, zzz=%d", idxA, idxM, idxZ)
	}
}

func TestRegistry_WritePrometheus_Full(t *testing.T) {
	resetAllGlobalState()
	r := newTestRegistry()
	r.Counter("test_c", "counter help").Add(10)
	r.Gauge("test_g", "gauge help").Set(99)
	h := r.Histogram("test_h", "hist help", []float64{5})
	h.Observe(3)

	var b strings.Builder
	r.WritePrometheus(&b)
	output := b.String()

	sections := []string{
		"# TYPE test_c counter",
		"test_c 10",
		"# TYPE test_g gauge",
		"test_g 99",
		"# TYPE test_h histogram",
		"test_h_count 1",
	}
	for _, s := range sections {
		if !strings.Contains(output, s) {
			t.Errorf("Full WritePrometheus missing %q\n\nFull:\n%s", s, output)
		}
	}
}

// ==================== Format Function Tests ====================

func TestAppendInt_Positive(t *testing.T) {
	var b strings.Builder
	appendInt(&b, 12345)
	if got := b.String(); got != "12345" {
		t.Errorf("appendInt(12345): want \"12345\", got %q", got)
	}
}

func TestAppendInt_Zero(t *testing.T) {
	var b strings.Builder
	appendInt(&b, 0)
	if got := b.String(); got != "0" {
		t.Errorf("appendInt(0): want \"0\", got %q", got)
	}
}

func TestAppendInt_Negative(t *testing.T) {
	var b strings.Builder
	appendInt(&b, -42)
	if got := b.String(); got != "-42" {
		t.Errorf("appendInt(-42): want \"-42\", got %q", got)
	}
}

func TestAppendInt_Large(t *testing.T) {
	var b strings.Builder
	appendInt(&b, 9999999999)
	if got := b.String(); got != "9999999999" {
		t.Errorf("appendInt(9999999999): want \"9999999999\", got %q", got)
	}
}

func TestAppendFloat_Integer(t *testing.T) {
	var b strings.Builder
	appendFloat(&b, 100.0)
	if got := b.String(); got != "100" {
		t.Errorf("appendFloat(100.0): want \"100\", got %q", got)
	}
}

func TestAppendFloat_Decimal(t *testing.T) {
	var b strings.Builder
	appendFloat(&b, 3.5)
	got := b.String()
	if got != "3.5" {
		t.Errorf("appendFloat(3.5): want \"3.5\", got %q", got)
	}
}

func TestAppendFloat_SmallFraction(t *testing.T) {
	var b strings.Builder
	appendFloat(&b, 0.001)
	got := b.String()
	if !strings.HasPrefix(got, "0.001") {
		t.Errorf("appendFloat(0.001): want prefix \"0.001\", got %q", got)
	}
}

func TestFormatInt_SingleDigit(t *testing.T) {
	var buf [20]byte
	n := formatInt(buf[:], 7)
	got := string(buf[n:])
	if got != "7" {
		t.Errorf("formatInt(7): want \"7\", got %q", got)
	}
}

func TestFormatInt_NegativeSingleDigit(t *testing.T) {
	var buf [20]byte
	n := formatInt(buf[:], -3)
	got := string(buf[n:])
	if got != "-3" {
		t.Errorf("formatInt(-3): want \"-3\", got %q", got)
	}
}

// ==================== sortedKeys Tests ====================

func TestSortedKeys(t *testing.T) {
	m := map[string]*counterEntry{
		"zzz": {},
		"aaa": {},
		"mmm": {},
	}
	keys := sortedKeys(m)
	if len(keys) != 3 {
		t.Fatalf("want 3 keys, got %d", len(keys))
	}
	if keys[0] != "aaa" || keys[1] != "mmm" || keys[2] != "zzz" {
		t.Errorf("expected sorted order [aaa mmm zzz], got %v", keys)
	}
}

func TestSortedKeys_Empty(t *testing.T) {
	m := map[string]*counterEntry{}
	keys := sortedKeys(m)
	if len(keys) != 0 {
		t.Errorf("expected empty keys, got %v", keys)
	}
}

func TestSortedGaugeKeys(t *testing.T) {
	m := map[string]*gaugeEntry{
		"beta":  {},
		"alpha": {},
	}
	keys := sortedGaugeKeys(m)
	if len(keys) != 2 || keys[0] != "alpha" || keys[1] != "beta" {
		t.Errorf("expected [alpha beta], got %v", keys)
	}
}

func TestSortedHistKeys(t *testing.T) {
	m := map[string]*histogramEntry{
		"z_hist": {},
		"a_hist": {},
	}
	keys := sortedHistKeys(m)
	if len(keys) != 2 || keys[0] != "a_hist" || keys[1] != "z_hist" {
		t.Errorf("expected [a_hist z_hist], got %v", keys)
	}
}

// ==================== Benchmarks ====================

func BenchmarkCounter_Inc(b *testing.B) {
	var c Counter
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		c.Inc()
	}
}

func BenchmarkCounter_Inc_Parallel(b *testing.B) {
	var c Counter
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c.Inc()
		}
	})
}

func BenchmarkGauge_SetFloat(b *testing.B) {
	var g Gauge
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		g.SetFloat(3.14)
	}
}

func BenchmarkHistogram_Observe(b *testing.B) {
	h := NewLatencyHistogram()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		h.Observe(5.0)
	}
}

func BenchmarkHistogram_Observe_Parallel(b *testing.B) {
	h := NewLatencyHistogram()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			h.Observe(5.0)
		}
	})
}

func BenchmarkAppendInt(b *testing.B) {
	var sb strings.Builder
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sb.Reset()
		appendInt(&sb, 1234567890)
	}
}

func BenchmarkAppendFloat(b *testing.B) {
	var sb strings.Builder
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sb.Reset()
		appendFloat(&sb, 3.14159)
	}
}

func BenchmarkRegistry_WritePrometheus(b *testing.B) {
	r := newTestRegistry()
	for i := 0; i < 10; i++ {
		r.Counter("counter_"+string(rune('a'+i)), "help").Add(int64(i * 100))
	}
	for i := 0; i < 5; i++ {
		r.Gauge("gauge_"+string(rune('a'+i)), "help").Set(int64(i * 10))
	}
	for i := 0; i < 5; i++ {
		h := r.Histogram("hist_"+string(rune('a'+i)), "help", DefaultLatencyBounds)
		h.Observe(float64(i))
	}

	var sb strings.Builder
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sb.Reset()
		r.WritePrometheus(&sb)
	}
}
