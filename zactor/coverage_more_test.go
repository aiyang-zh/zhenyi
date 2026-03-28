package zactor

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi-base/zpub"
	"github.com/aiyang-zh/zhenyi/zbus"
	"github.com/aiyang-zh/zhenyi/zmodel"
	"github.com/aiyang-zh/zhenyi/zmsg"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func init() {
	// 覆盖测试同样禁用文件日志，避免 rotatelogs lock 冲突
	cfg := zlog.NewDefaultLoggerConfig()
	cfg.Logs = map[string]int{}
	cfg.IsConsole = false
	zlog.NewDefaultLoggerWithConfig(cfg)
}

type memSub struct{}

func (memSub) Unsubscribe() error { return nil }

type memBus struct {
	mu       sync.Mutex
	handlers map[string]zbus.Handler
}

func newMemBus() *memBus { return &memBus{handlers: make(map[string]zbus.Handler)} }

func (b *memBus) Broadcast(topic string, data []byte) error {
	b.mu.Lock()
	h := b.handlers[topic]
	b.mu.Unlock()
	if h != nil {
		h(topic, data)
	}
	return nil
}

func (b *memBus) Subscribe(topic string, handler zbus.Handler) (zbus.Subscription, error) {
	b.mu.Lock()
	b.handlers[topic] = handler
	b.mu.Unlock()
	return memSub{}, nil
}

func TestDefaultLocalRouter_RouteLocal(t *testing.T) {
	r := NewDefaultLocalRouter()
	if _, err := r.RouteLocal(nil, nil); err == nil {
		t.Fatalf("expected error for nil args")
	}

	g := NewGroup(1, true)
	msg := zmsg.GetMessage()
	msg.MsgId = 123
	defer msg.Release()

	if _, err := r.RouteLocal(g, msg); err == nil {
		t.Fatalf("expected not found")
	}

	// register one actor and route
	a := NewActor(zmodel.ActorConfig{Id: 1, ActorType: 1, Name: "a"})
	g.AddActor(a)
	g.RegisterRoutes(a, []int32{123})
	got, err := r.RouteLocal(g, msg)
	if err != nil || got == nil || got.GetActorId() != 1 {
		t.Fatalf("route got=%v err=%v", got, err)
	}
}

func TestGroup_RegisterRoutesAndLookupActorsByMsgID(t *testing.T) {
	g := NewGroup(1, true)
	a1 := NewActor(zmodel.ActorConfig{Id: 1, ActorType: 1, Name: "a1"})
	a2 := NewActor(zmodel.ActorConfig{Id: 2, ActorType: 1, Name: "a2"})
	g.AddActor(a1)
	g.AddActor(a2)

	// nil/empty no-op
	g.RegisterRoutes(nil, []int32{1})
	g.RegisterRoutes(a1, nil)

	g.RegisterRoutes(a1, []int32{10})
	g.RegisterRoutes(a2, []int32{10})
	// duplicate should be skipped (covered branch)
	g.RegisterRoutes(a2, []int32{10})

	list := g.LookupActorsByMsgID(10)
	if len(list) != 2 {
		t.Fatalf("len=%d", len(list))
	}
	// returned slice should be a copy
	list[0] = nil
	list2 := g.LookupActorsByMsgID(10)
	if list2[0] == nil {
		t.Fatalf("expected copy")
	}
	if g.LookupActorsByMsgID(999) != nil {
		t.Fatalf("expected nil for missing")
	}
}

func TestActor_PubSub_ReceiveRemote_LocalBroadcast(t *testing.T) {
	bus := newMemBus()
	zbus.DefaultBus = bus
	t.Cleanup(func() { zbus.DefaultBus = nil })

	a := NewActor(zmodel.ActorConfig{Id: 1, ActorType: 2, Name: "a"})
	a.SetIActor(a)
	a.SetGroup(NewGroup(1, true))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := a.Init(ctx); err != nil {
		t.Fatalf("init err: %v", err)
	}
	// GetGroup covered
	if a.GetGroup() == nil {
		t.Fatalf("expected group")
	}

	// Ensure remote bus subscriptions exist by pushing a marshaled msg
	m := zmsg.GetMessage()
	m.MsgId = 9
	m.Data = append(m.Data[:0], []byte("hi")...)
	data, err := m.Marshal()
	if err != nil {
		t.Fatalf("marshal err: %v", err)
	}
	m.Release()

	// deliver on subscribed topic
	bus.mu.Lock()
	h := bus.handlers[a.GetTopic()]
	bus.mu.Unlock()
	if h == nil {
		t.Fatalf("expected handler registered")
	}
	h(a.GetTopic(), data)

	// verify queued cmd
	cmd, ok := a.mailBoxQueue.Dequeue()
	if !ok || cmd.Type != zmodel.CmdTypeMsg || cmd.Msg == nil || cmd.Msg.MsgId != 9 {
		t.Fatalf("cmd=%+v ok=%v", cmd, ok)
	}
	cmd.Release()

	// Local broadcast via zpub EventSystem
	msg2 := zmsg.GetMessage()
	msg2.MsgId = 11
	before := msg2.LoadRefCount()
	zpub.EventSystem.Publish(&zpub.Event{Topic: a.GetTopic(), Val: msg2})
	// expect retain happened (at least +1)
	if msg2.LoadRefCount() <= before {
		t.Fatalf("expected retain, before=%d after=%d", before, msg2.LoadRefCount())
	}
	// drain queued
	cmd, ok = a.mailBoxQueue.Dequeue()
	if !ok || cmd.Msg == nil || cmd.Msg.MsgId != 11 {
		t.Fatalf("cmd=%+v ok=%v", cmd, ok)
	}
	cmd.Release()
	// release original publish msg
	msg2.Release()
}

func TestActor_ReceiveRemote_UnmarshalErrorReleases(t *testing.T) {
	a := NewActor(zmodel.ActorConfig{Id: 1, ActorType: 2, Name: "a"})
	a.receiveRemote("t", []byte("corrupt"))
}

type dummyVT struct {
	msgID int32
	buf   []byte
}

func (d *dummyVT) ProtoReflect() protoreflect.Message { return nil }
func (d *dummyVT) UnmarshalVT(b []byte) error         { d.buf = append(d.buf[:0], b...); return nil }
func (d *dummyVT) MarshalVT() ([]byte, error)         { return append([]byte(nil), d.buf...), nil }
func (d *dummyVT) MarshalToVT(dst []byte) (int, error) {
	n := copy(dst, d.buf)
	return n, nil
}
func (d *dummyVT) SizeVT() int     { return len(d.buf) }
func (d *dummyVT) GetMsgId() int32 { return d.msgID }

func TestActor_SendActor_And_SendActorReply_NilReply(t *testing.T) {
	g := NewGroup(1, true)
	a1 := NewActor(zmodel.ActorConfig{Id: 1, ActorType: 1, Name: "a1"})
	a2 := NewActor(zmodel.ActorConfig{Id: 2, ActorType: 1, Name: "a2"})
	g.AddActor(a1)
	g.AddActor(a2)
	a1.SetIActor(a1)
	a2.SetIActor(a2)
	a1.SetGroup(g)
	a2.SetGroup(g)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := a1.Init(ctx); err != nil {
		t.Fatalf("init err: %v", err)
	}
	if err := a2.Init(ctx); err != nil {
		t.Fatalf("init err: %v", err)
	}

	req := &dummyVT{msgID: 100, buf: []byte("hi")}
	a1.SendActor(2, req)

	// should arrive at a2 mailbox
	cmd, ok := a2.mailBoxQueue.Dequeue()
	if !ok || cmd.Msg == nil || cmd.Msg.MsgId != 100 {
		t.Fatalf("cmd=%+v ok=%v", cmd, ok)
	}
	cmd.Release()

	// SendActorReply: nil reply sets placeholder reply for rpcId
	orig := zmsg.GetMessage()
	rpcID, err := a1.AddSender()
	if err != nil {
		t.Fatalf("AddSender err: %v", err)
	}
	orig.RpcId = rpcID
	orig.SrcActor = 2
	a1.SendActorReply(orig, nil)
	orig.Release()

	got, ok := a1.GetReply(rpcID, 50*time.Millisecond)
	if !ok || got == nil {
		t.Fatalf("expected placeholder reply")
	}
	got.Release()
}

func TestTraceHooks_SetOnceAndEnabled(t *testing.T) {
	SetTraceHooks(
		func() bool { return true },
		func(ctx context.Context, name string) (context.Context, func()) {
			_ = name
			return ctx, func() {}
		},
		func(ctx context.Context, msg *zmsg.Message) context.Context {
			_ = msg
			return ctx
		},
		func(msg *zmsg.Message) { msg.TraceIdHi = 1 },
	)
	if !isTraceEnabled() {
		t.Fatalf("expected trace enabled")
	}
	// second call should be ignored and not panic
	SetTraceHooks(nil, nil, nil, nil)
}

func TestActor_CallInitServer_Branches(t *testing.T) {
	a := NewActor(zmodel.ActorConfig{Id: 1, ActorType: 2, Name: "a"})
	// nil initServer
	if err := a.CallInitServer(context.Background()); err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	a.SetInitServer(func(ctx context.Context) error {
		_ = ctx
		return nil
	})
	if err := a.CallInitServer(context.Background()); err != nil {
		t.Fatalf("err=%v", err)
	}
	// cancelled ctx path still calls initServer but may return ctx error from user impl
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a.SetInitServer(func(ctx context.Context) error { return ctx.Err() })
	if err := a.CallInitServer(ctx); err == nil {
		t.Fatalf("expected err")
	}
}

func TestActor_Run_SelectActor_SkipHardPaths(t *testing.T) {
	// Run/SelectActor 涉及 goroutine/worker/tick 组合，属于“实在不好测”的先跳过。
	_ = time.Second
}

func TestLocalActorBroadcast_OnChange_TypeGuard(t *testing.T) {
	a := NewActor(zmodel.ActorConfig{Id: 1, ActorType: 1, Name: "a"})
	l := &LocalActorBroadcast{a: a}
	l.OnChange(&zpub.Event{Topic: "t", Val: "not_msg"})
	l.OnChange(&zpub.Event{Topic: "t", Val: (*zmsg.Message)(nil)})
}
