package zmsg

import (
	"context"
	"sync"
	"testing"

	"github.com/aiyang-zh/zhenyi-base/zlog"
)

func init() {
	zlog.NewDefaultLogger()
}

// ============================================================
// Message Pool — GetMessage / Retain / Release
// ============================================================

func TestGetMessage_InitialState(t *testing.T) {
	msg := GetMessage()
	defer msg.Release()

	if msg == nil {
		t.Fatal("GetMessage returned nil")
	}
	if rc := msg.LoadRefCount(); rc != 1 {
		t.Fatalf("expected RefCount=1, got %d", rc)
	}
	if msg.MsgId != 0 {
		t.Fatalf("expected MsgId=0 after pool reset, got %d", msg.MsgId)
	}
	if len(msg.Data) != 0 {
		t.Fatalf("expected empty Data, got len=%d", len(msg.Data))
	}

}

func TestRetain_Nil(t *testing.T) {
	var m *Message
	got := m.Retain()
	if got != nil {
		t.Fatal("Retain(nil) should return nil")
	}
}

func TestRetain_Normal(t *testing.T) {
	msg := GetMessage()
	defer msg.Release()

	ret := msg.Retain()
	if ret != msg {
		t.Fatal("Retain should return same pointer")
	}
	if rc := msg.LoadRefCount(); rc != 2 {
		t.Fatalf("expected RefCount=2 after Retain, got %d", rc)
	}
	msg.Release() // balance the extra Retain
}

func TestRetain_Multiple(t *testing.T) {
	msg := GetMessage()
	msg.Retain()
	msg.Retain()
	if rc := msg.LoadRefCount(); rc != 3 {
		t.Fatalf("expected RefCount=3, got %d", rc)
	}
	msg.Release()
	msg.Release()
	if rc := msg.LoadRefCount(); rc != 1 {
		t.Fatalf("expected RefCount=1, got %d", rc)
	}
	msg.Release()
}

func TestRelease_Nil(t *testing.T) {
	var m *Message
	m.Release() // should not panic
}

func TestRelease_BackToPool(t *testing.T) {
	msg := GetMessage()
	msg.MsgId = 42
	msg.Release()

	msg2 := GetMessage()
	defer msg2.Release()
	if msg2.MsgId != 0 {
		t.Fatal("pooled message should have MsgId=0 after PoolReset")
	}
}

func TestRelease_LargeBufferCleanup(t *testing.T) {
	msg := GetMessage()
	msg.Data = make([]byte, 5000)
	msg.Release()

	msg2 := GetMessage()
	defer msg2.Release()
	// After release of large buffers, they should be nil'd and re-allocated from pool
	// The pool creates with cap=256 / cap=4 by default
	if cap(msg2.Data) > 5000 {
		t.Fatal("expected large Data buffer to be released")
	}
}

func TestRelease_DoubleRelease_NosPanic(t *testing.T) {
	msg := GetMessage()
	msg.Release()
	// Second release should not panic in production mode (DEBUG_LIFECYCLE=false)
	msg.Release()
}

func TestMustRelease_Nil(t *testing.T) {
	var m *Message
	m.MustRelease() // should not panic
}

func TestMustRelease_Normal(t *testing.T) {
	msg := GetMessage()
	msg.MustRelease()
	// should behave same as Release
}

func TestLoadRefCount_Nil(t *testing.T) {
	var m *Message
	if rc := m.LoadRefCount(); rc != 0 {
		t.Fatalf("expected LoadRefCount(nil)=0, got %d", rc)
	}
}

func TestLoadRefCount_AfterRetainRelease(t *testing.T) {
	msg := GetMessage()
	if msg.LoadRefCount() != 1 {
		t.Fatal("expected RefCount=1")
	}
	msg.Retain()
	if msg.LoadRefCount() != 2 {
		t.Fatal("expected RefCount=2")
	}
	msg.Release()
	if msg.LoadRefCount() != 1 {
		t.Fatal("expected RefCount=1")
	}
	msg.Release()
}

// ============================================================
// Message Pool — Concurrent Safety
// ============================================================

func TestMessagePool_ConcurrentGetRelease(t *testing.T) {
	const goroutines = 50
	const iterations = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				msg := GetMessage()
				msg.MsgId = int32(i)
				msg.Data = append(msg.Data, byte(i))
				msg.Release()
			}
		}()
	}
	wg.Wait()
}

func TestMessagePool_RetainReleaseConcurrent(t *testing.T) {
	msg := GetMessage()

	const retainers = 20
	var wg sync.WaitGroup
	wg.Add(retainers)
	for i := 0; i < retainers; i++ {
		msg.Retain()
	}
	for i := 0; i < retainers; i++ {
		go func() {
			defer wg.Done()
			msg.Release()
		}()
	}
	wg.Wait()
	if rc := msg.LoadRefCount(); rc != 1 {
		t.Fatalf("expected RefCount=1 after concurrent releases, got %d", rc)
	}
	msg.Release()
}

// ============================================================
// SmartReset vs PoolReset
// ============================================================

func TestSmartReset(t *testing.T) {
	msg := GetMessage()
	msg.MsgId = 100
	msg.Data = []byte("hello")
	msg.ToClient = true
	msg.IsResponse = true
	msg.RefCount = 5

	msg.SmartReset()

	if msg.MsgId != 0 || msg.ToClient || msg.IsResponse || msg.RefCount != 0 {
		t.Fatal("SmartReset didn't clear all fields")
	}
	if msg.Data != nil {
		t.Fatal("SmartReset should set Data to nil")
	}
}

func TestPoolReset(t *testing.T) {
	msg := &Message{
		MsgId:    100,
		Data:     make([]byte, 10, 256),
		ToClient: true,
	}
	origDataCap := cap(msg.Data)

	msg.PoolReset()

	if msg.MsgId != 0 || msg.ToClient {
		t.Fatal("PoolReset didn't clear fields")
	}
	if len(msg.Data) != 0 {
		t.Fatal("PoolReset should truncate Data to len=0")
	}
	if cap(msg.Data) != origDataCap {
		t.Fatal("PoolReset should preserve Data capacity")
	}
}

// ============================================================
// Marshal / Unmarshal
// ============================================================

func TestMarshalUnmarshal_Roundtrip(t *testing.T) {
	orig := &Message{
		MsgId:      42,
		SrcActor:   1,
		TarActor:   2,
		SessionId:  9999,
		RpcId:      8888,
		SeqId:      7,
		TraceIdHi:  5555,
		TraceIdLo:  6666,
		SpanId:     3333,
		ToClient:   true,
		FromClient: false,
		IsResponse: true,
		Data:       []byte("hello world"),
	}

	buf, err := orig.Marshal()
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	decoded := &Message{}
	if err := decoded.Unmarshal(buf); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.MsgId != orig.MsgId {
		t.Fatalf("MsgId mismatch: %d vs %d", decoded.MsgId, orig.MsgId)
	}

	if decoded.SrcActor != orig.SrcActor || decoded.TarActor != orig.TarActor {
		t.Fatal("Actor ID mismatch")
	}
	if decoded.SessionId != orig.SessionId || decoded.RpcId != orig.RpcId {
		t.Fatal("Session/RPC mismatch")
	}
	if decoded.SeqId != orig.SeqId || decoded.TraceIdHi != orig.TraceIdHi || decoded.TraceIdLo != orig.TraceIdLo || decoded.SpanId != orig.SpanId {
		t.Fatal("Seq/Trace/Span mismatch")
	}
	if !decoded.ToClient || decoded.FromClient || !decoded.IsResponse {
		t.Fatal("flags mismatch")
	}
	if string(decoded.Data) != "hello world" {
		t.Fatalf("Data mismatch: %q", decoded.Data)
	}

}

func TestMarshalUnmarshal_EmptyData(t *testing.T) {
	orig := &Message{MsgId: 1}
	buf, err := orig.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	decoded := &Message{}
	if err := decoded.Unmarshal(buf); err != nil {
		t.Fatal(err)
	}
	if decoded.Data != nil {
		t.Fatal("empty Data should unmarshal as nil")
	}

}

func TestMarshalTo_BufferTooSmall(t *testing.T) {
	msg := &Message{MsgId: 1, Data: []byte("data")}
	buf := make([]byte, 10)
	_, err := msg.MarshalTo(buf)
	if err != ErrBufferTooSmall {
		t.Fatalf("expected ErrBufferTooSmall, got %v", err)
	}
}

func TestUnmarshal_TooShort(t *testing.T) {
	msg := &Message{}
	err := msg.Unmarshal(make([]byte, 10))
	if err != ErrDataCorrupt {
		t.Fatalf("expected ErrDataCorrupt, got %v", err)
	}
}

func TestUnmarshal_TruncatedData(t *testing.T) {
	orig := &Message{MsgId: 1, Data: []byte("hello")}
	buf, _ := orig.Marshal()
	// Truncate buffer so Data is incomplete
	err := (&Message{}).Unmarshal(buf[:FixedHeaderSize+4+2])
	if err != ErrDataCorrupt {
		t.Fatalf("expected ErrDataCorrupt for truncated data, got %v", err)
	}
}

func TestUnmarshal_TruncatedAuthIds(t *testing.T) {
	orig := &Message{MsgId: 1}
	buf, _ := orig.Marshal()
	// Truncate buffer so AuthIds are incomplete
	err := (&Message{}).Unmarshal(buf[:len(buf)-4])
	if err != ErrDataCorrupt {
		t.Fatalf("expected ErrDataCorrupt for truncated AuthIds, got %v", err)
	}
}

func TestUnmarshal_DataCapReuse(t *testing.T) {
	msg := &Message{Data: make([]byte, 0, 128)}
	orig := &Message{Data: []byte("reuse me")}
	buf, _ := orig.Marshal()
	if err := msg.Unmarshal(buf); err != nil {
		t.Fatal(err)
	}
	if cap(msg.Data) < 128 {
		t.Fatal("Unmarshal should reuse existing Data capacity")
	}
}

func TestMarshalPooled(t *testing.T) {
	msg := &Message{
		MsgId: 42,
		Data:  []byte("pooled"),
	}
	buf, err := msg.MarshalPooled()
	if err != nil {
		t.Fatalf("MarshalPooled failed: %v", err)
	}
	defer buf.Release()

	decoded := &Message{}
	if err := decoded.Unmarshal(buf.B); err != nil {
		t.Fatalf("Unmarshal after MarshalPooled failed: %v", err)
	}
	if decoded.MsgId != 42 || string(decoded.Data) != "pooled" {
		t.Fatal("roundtrip mismatch")
	}
}

func TestSize(t *testing.T) {
	msg := &Message{}
	base := FixedHeaderSize + 4 // data len
	if msg.Size() != base {
		t.Fatalf("empty Size()=%d, want %d", msg.Size(), base)
	}

	msg.Data = []byte("test")
	want := base + 4 // 4 bytes data
	if msg.Size() != want {
		t.Fatalf("Size()=%d, want %d", msg.Size(), want)
	}
}

// ============================================================
// StartLeakDetector (DEBUG_LIFECYCLE=false path)
// ============================================================

func TestStartLeakDetector_DisabledMode(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	StartLeakDetector(ctx) // should return immediately when DEBUG_LIFECYCLE=false
}

// ============================================================
// MsgPool 监控已统一迁移到 zpool observer（日志/Prometheus），
// zmsg 只保留消息语义层的生命周期防御（RefCount/DoubleRelease 等）。

// ============================================================
// Benchmarks
// ============================================================

func BenchmarkGetMessageRelease(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		m := GetMessage()
		m.Release()
	}
}

func BenchmarkRetainRelease(b *testing.B) {
	msg := GetMessage()
	defer msg.Release()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		msg.Retain()
		msg.Release()
	}
}

func BenchmarkMarshalUnmarshal(b *testing.B) {
	msg := &Message{
		MsgId:    42,
		SrcActor: 1,
		TarActor: 2,
		Data:     make([]byte, 128),
	}
	buf := make([]byte, msg.Size())

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		msg.MarshalTo(buf)
	}
}

func BenchmarkUnmarshal(b *testing.B) {
	orig := &Message{
		MsgId:    42,
		SrcActor: 1,
		TarActor: 2,
		Data:     make([]byte, 128),
	}
	buf, _ := orig.Marshal()
	target := &Message{Data: make([]byte, 0, 256)}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		target.Unmarshal(buf)
	}
}

func BenchmarkMarshalPooled(b *testing.B) {
	msg := &Message{
		MsgId: 42,
		Data:  make([]byte, 128),
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf, _ := msg.MarshalPooled()
		buf.Release()
	}
}

func BenchmarkMessagePool_Parallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			m := GetMessage()
			m.MsgId = 1
			m.Data = append(m.Data, 1, 2, 3, 4)
			m.Release()
		}
	})
}

func BenchmarkLoadRefCount(b *testing.B) {
	msg := GetMessage()
	defer msg.Release()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = msg.LoadRefCount()
	}
}

func BenchmarkSize(b *testing.B) {
	msg := &Message{
		MsgId: 42,
		Data:  make([]byte, 256),
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = msg.Size()
	}
}

func BenchmarkSmartReset(b *testing.B) {
	msg := &Message{}
	data := []byte("test")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		msg.MsgId = 42
		msg.Data = data
		msg.SmartReset()
	}
}

func BenchmarkPoolReset(b *testing.B) {
	msg := &Message{
		Data: make([]byte, 0, 256),
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		msg.MsgId = 42
		msg.Data = msg.Data[:10]
		msg.PoolReset()
	}
}

func BenchmarkGetMessageRelease_WithStats(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		m := GetMessage()
		m.Release()
	}
}

func BenchmarkGetMessageRelease_WithStats_Parallel(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			m := GetMessage()
			m.Release()
		}
	})
}
