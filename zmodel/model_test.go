package zmodel

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aiyang-zh/zhenyi/zmsg"
)

// ============================================================
// ActorConfig
// ============================================================

func TestActorConfig_GetTopic(t *testing.T) {
	cfg := ActorConfig{Id: 100, ActorType: 2, Index: 3}
	want := "topic_2_3_100"
	if got := cfg.GetTopic(); got != want {
		t.Fatalf("GetTopic()=%q, want %q", got, want)
	}
}

func TestActorConfig_GetNameTopic(t *testing.T) {
	cfg := ActorConfig{ActorType: 5}
	want := "topic_name_5"
	if got := cfg.GetNameTopic(); got != want {
		t.Fatalf("GetNameTopic()=%q, want %q", got, want)
	}
}

func TestActorConfig_GetActorId(t *testing.T) {
	cfg := ActorConfig{Id: 42}
	if cfg.GetActorId() != 42 {
		t.Fatalf("expected 42, got %d", cfg.GetActorId())
	}
}

func TestActorConfig_GetActorType(t *testing.T) {
	cfg := ActorConfig{ActorType: 7}
	if cfg.GetActorType() != 7 {
		t.Fatalf("expected 7, got %d", cfg.GetActorType())
	}
}

// ============================================================
// ActorModeConfig
// ============================================================

func TestActorModeConfig_IsSequential(t *testing.T) {
	tests := []struct {
		mode int
		want bool
	}{
		{0, true}, {1, false}, {2, false},
	}
	for _, tt := range tests {
		c := ActorModeConfig{Mode: tt.mode}
		if got := c.IsSequential(); got != tt.want {
			t.Errorf("Mode=%d: IsSequential()=%v, want %v", tt.mode, got, tt.want)
		}
	}
}

func TestActorModeConfig_IsConcurrent(t *testing.T) {
	tests := []struct {
		mode int
		want bool
	}{
		{0, false}, {1, true}, {2, false},
	}
	for _, tt := range tests {
		c := ActorModeConfig{Mode: tt.mode}
		if got := c.IsConcurrent(); got != tt.want {
			t.Errorf("Mode=%d: IsConcurrent()=%v, want %v", tt.mode, got, tt.want)
		}
	}
}

func TestActorModeConfig_GetPoolSize(t *testing.T) {
	if (ActorModeConfig{}).GetPoolSize() != 100 {
		t.Fatal("default pool size should be 100")
	}
	if (ActorModeConfig{ConcurrentPoolSize: -1}).GetPoolSize() != 100 {
		t.Fatal("negative pool size should default to 100")
	}
	if (ActorModeConfig{ConcurrentPoolSize: 200}).GetPoolSize() != 200 {
		t.Fatal("explicit pool size should be returned")
	}
}

func TestActorModeConfig_GetMaxBatch(t *testing.T) {
	if (ActorModeConfig{}).GetMaxBatch() != 50 {
		t.Fatal("default max batch should be 50")
	}
	if (ActorModeConfig{ConcurrentMaxBatch: -5}).GetMaxBatch() != 50 {
		t.Fatal("negative max batch should default to 50")
	}
	if (ActorModeConfig{ConcurrentMaxBatch: 300}).GetMaxBatch() != 300 {
		t.Fatal("explicit max batch should be returned")
	}
}

// ============================================================
// TickFnItem
// ============================================================

func TestNewTickFnItem(t *testing.T) {
	called := false
	item := NewTickFnItem("test", 1*time.Second, func(ctx context.Context, nowTs int64) {
		called = true
	})
	if item.Name != "test" || item.Interval != 1*time.Second {
		t.Fatal("field mismatch")
	}
	item.Do(context.Background(), 0)
	if !called {
		t.Fatal("Do func was not set")
	}
}

// ============================================================
// ActorCmd Release / Retain
// ============================================================

func TestActorCmd_Release_WithMsg(t *testing.T) {
	msg := zmsg.GetMessage()
	msg.Retain() // refCount=2
	cmd := ActorCmd{Msg: msg}
	cmd.Release()
	if cmd.Msg != nil {
		t.Fatal("Msg should be nil after Release")
	}
	if msg.LoadRefCount() != 1 {
		t.Fatalf("expected RefCount=1, got %d", msg.LoadRefCount())
	}
	msg.Release()
}

func TestActorCmd_Release_NilMsg(t *testing.T) {
	cmd := ActorCmd{}
	cmd.Release() // should not panic
}

func TestActorCmd_Retain_WithMsg(t *testing.T) {
	msg := zmsg.GetMessage()
	defer msg.Release()

	cmd := ActorCmd{Msg: msg}
	cmd.Retain()
	if msg.LoadRefCount() != 2 {
		t.Fatalf("expected RefCount=2, got %d", msg.LoadRefCount())
	}
	msg.Release() // balance
}

func TestActorCmd_Retain_NilMsg(t *testing.T) {
	cmd := ActorCmd{}
	cmd.Retain() // should not panic
}

// ============================================================
// Benchmarks
// ============================================================

func BenchmarkActorCmd_Release(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		msg := zmsg.GetMessage()
		cmd := ActorCmd{Msg: msg}
		cmd.Release()
	}
}

func BenchmarkAtomicRefCount(b *testing.B) {
	var rc int32 = 1
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		atomic.AddInt32(&rc, 1)
		atomic.AddInt32(&rc, -1)
	}
}
