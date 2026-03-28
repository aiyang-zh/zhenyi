package zactor

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi/ziface"
	"github.com/aiyang-zh/zhenyi/zmodel"
	"github.com/aiyang-zh/zhenyi/zmsg"
)

func init() {
	// 测试环境禁用文件日志，避免并发测试/benchmark 的日志锁冲突
	cfg := zlog.NewDefaultLoggerConfig()
	cfg.Logs = map[string]int{}
	cfg.IsConsole = false
	zlog.NewDefaultLoggerWithConfig(cfg)
}

// ============================================================
// Handle（handler.go）— 消息处理器注册与分发
// ============================================================

func newTestHandle() *HandleRegistry {
	cfg := zmodel.ActorConfig{
		Id:        1,
		Name:      "test",
		ActorType: 1,
		Index:     1,
		Process:   1,
	}
	a := NewActor(cfg)
	return NewHandleRegistry(a)
}

func TestHandle_RegisterAndGet(t *testing.T) {
	h := newTestHandle()

	called := false
	h.RegisterHandle(100, func(ctx context.Context, msg *zmsg.Message) {
		called = true
	})

	handler := h.GetClientHandle(100)
	if handler == nil {
		t.Fatal("expected handler for msgId=100")
	}

	handler(context.Background(), zmsg.GetMessage())
	if !called {
		t.Fatal("handler was not called")
	}
}

func TestHandle_RegisterDuplicate(t *testing.T) {
	h := newTestHandle()

	callCount := 0
	h.RegisterHandle(100, func(ctx context.Context, msg *zmsg.Message) {
		callCount = 1
	})
	// 重复注册应被忽略
	h.RegisterHandle(100, func(ctx context.Context, msg *zmsg.Message) {
		callCount = 2
	})

	handler := h.GetClientHandle(100)
	handler(context.Background(), zmsg.GetMessage())
	if callCount != 1 {
		t.Fatalf("expected first handler (callCount=1), got %d", callCount)
	}
}

func TestHandle_GetNotFound(t *testing.T) {
	h := newTestHandle()

	handler := h.GetClientHandle(999)
	if handler != nil {
		t.Fatal("expected nil for unregistered msgId")
	}
}

func TestHandle_AddMsgId(t *testing.T) {
	h := newTestHandle()
	h.AddMsgId(200)
	h.AddMsgId(201)

	list := h.GetMsgIdList()
	if _, ok := list[200]; !ok {
		t.Fatal("expected msgId 200 in list")
	}
	if _, ok := list[201]; !ok {
		t.Fatal("expected msgId 201 in list")
	}
}

// ============================================================
// Dispatcher（dispatcher.go）— 消息分发器
// ============================================================

type mockProcessor struct {
	called  bool
	lastMsg *zmsg.Message
}

func (p *mockProcessor) Process(ctx context.Context, msg *zmsg.Message) ziface.IMessage {
	p.called = true
	p.lastMsg = msg
	return nil
}

func TestDispatcher_RegisterAndDispatch_Handler(t *testing.T) {
	cfg := zmodel.ActorConfig{Id: 1, Name: "test", ActorType: 1, Index: 1}
	a := &Actor{ActorConfig: cfg}
	d := NewDispatcher(a)

	proc := &mockProcessor{}
	d.Register(100, func(ctx context.Context, msg *zmsg.Message) ziface.IMessage {
		return proc.Process(ctx, msg)
	})

	msg := zmsg.GetMessage()
	defer msg.Release()
	msg.MsgId = 100

	d.Dispatch(context.Background(), msg)

	if !proc.called {
		t.Fatal("processor was not called")
	}
	if proc.lastMsg != msg {
		t.Fatal("processor received wrong message")
	}
}

func TestDispatcher_Register(t *testing.T) {
	cfg := zmodel.ActorConfig{Id: 1, Name: "test", ActorType: 1, Index: 1}
	a := &Actor{ActorConfig: cfg}
	d := NewDispatcher(a)

	called := false
	d.Register(200, func(ctx context.Context, msg *zmsg.Message) ziface.IMessage {
		called = true
		return nil
	})

	msg := zmsg.GetMessage()
	defer msg.Release()
	msg.MsgId = 200

	d.Dispatch(context.Background(), msg)
	if !called {
		t.Fatal("msg handler was not called")
	}
}

func TestDispatcher_RegisterBatch(t *testing.T) {
	cfg := zmodel.ActorConfig{Id: 1, Name: "test", ActorType: 1, Index: 1}
	a := &Actor{ActorConfig: cfg}
	d := NewDispatcher(a)

	proc1 := &mockProcessor{}
	proc2 := &mockProcessor{}
	d.RegisterBatch(map[int32]ziface.MsgHandlerFunc{
		300: func(ctx context.Context, msg *zmsg.Message) ziface.IMessage { return proc1.Process(ctx, msg) },
		301: func(ctx context.Context, msg *zmsg.Message) ziface.IMessage { return proc2.Process(ctx, msg) },
	})

	msg1 := zmsg.GetMessage()
	defer msg1.Release()
	msg1.MsgId = 300
	d.Dispatch(context.Background(), msg1)

	msg2 := zmsg.GetMessage()
	defer msg2.Release()
	msg2.MsgId = 301
	d.Dispatch(context.Background(), msg2)

	if !proc1.called || !proc2.called {
		t.Fatal("batch registered processors not all called")
	}
}

func TestDispatcher_UnregisteredMsg_NoOp(t *testing.T) {
	cfg := zmodel.ActorConfig{Id: 1, Name: "test", ActorType: 1, Index: 1}
	a := &Actor{ActorConfig: cfg, logger: zlog.GetDefaultLog()}
	d := NewDispatcher(a)

	msg := zmsg.GetMessage()
	defer msg.Release()
	msg.MsgId = 999

	// 不应 panic
	d.Dispatch(context.Background(), msg)
}

// ============================================================
// ActorMsgSender（sender.go）— RPC 槽位管理
// ============================================================

func TestSender_AddSenderAndSetReply(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := NewSender(ctx, 0)

	rpcId, err := s.AddSender()
	if err != nil {
		t.Fatalf("AddSender failed: %v", err)
	}
	if rpcId == 0 {
		t.Fatal("expected non-zero rpcId")
	}

	// 构建回复消息
	reply := zmsg.GetMessage()
	reply.RpcId = rpcId
	reply.IsResponse = true

	s.SetReply(reply)

	// 取回复
	data, ok := s.GetReply(rpcId, 1*time.Second)
	if !ok {
		t.Fatal("GetReply returned false")
	}
	if data.RpcId != rpcId {
		t.Fatalf("expected RpcId=%d, got %d", rpcId, data.RpcId)
	}
	data.Release()
}

func TestSender_GetReply_Timeout(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := NewSender(ctx, 0)

	rpcId, err := s.AddSender()
	if err != nil {
		t.Fatalf("AddSender failed: %v", err)
	}

	// 不发 SetReply，等待超时
	start := time.Now()
	_, ok := s.GetReply(rpcId, 100*time.Millisecond)
	elapsed := time.Since(start)

	if ok {
		t.Fatal("expected timeout (ok=false)")
	}
	if elapsed < 90*time.Millisecond {
		t.Fatalf("timeout too fast: %v", elapsed)
	}
}

func TestSender_GetReply_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	s := NewSender(ctx, 0)

	rpcId, err := s.AddSender()
	if err != nil {
		t.Fatalf("AddSender failed: %v", err)
	}

	// 50ms 后取消上下文
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, ok := s.GetReply(rpcId, 5*time.Second)
	if ok {
		t.Fatal("expected cancel (ok=false)")
	}
}

func TestSender_MultipleConcurrentRPCs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := NewSender(ctx, 0)

	const rpcCount = 100
	rpcIds := make([]uint64, rpcCount)

	// 分配多个 RPC
	for i := 0; i < rpcCount; i++ {
		id, err := s.AddSender()
		if err != nil {
			t.Fatalf("AddSender[%d] failed: %v", i, err)
		}
		rpcIds[i] = id
	}

	// 检查唯一性
	seen := make(map[uint64]bool)
	for _, id := range rpcIds {
		if seen[id] {
			t.Fatalf("duplicate rpcId: %d", id)
		}
		seen[id] = true
	}

	// 并发回复
	var wg sync.WaitGroup
	wg.Add(rpcCount)
	for i := 0; i < rpcCount; i++ {
		go func(idx int) {
			defer wg.Done()
			reply := zmsg.GetMessage()
			reply.RpcId = rpcIds[idx]
			reply.IsResponse = true
			s.SetReply(reply)
		}(i)
	}
	wg.Wait()

	// 并发获取回复
	var wg2 sync.WaitGroup
	successCount := int32(0)
	wg2.Add(rpcCount)
	for i := 0; i < rpcCount; i++ {
		go func(idx int) {
			defer wg2.Done()
			data, ok := s.GetReply(rpcIds[idx], 2*time.Second)
			if ok {
				atomic.AddInt32(&successCount, 1)
				data.Release()
			}
		}(i)
	}
	wg2.Wait()

	if successCount != rpcCount {
		t.Fatalf("expected %d successful replies, got %d", rpcCount, successCount)
	}
}

func TestSender_StaleReply_Ignored(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := NewSender(ctx, 0)

	// 分配一个 RPC 并等超时
	rpcId, _ := s.AddSender()
	_, ok := s.GetReply(rpcId, 50*time.Millisecond)
	if ok {
		t.Fatal("expected timeout")
	}

	// 超时后 SetReply 不应导致 panic 或阻塞
	reply := zmsg.GetMessage()
	reply.RpcId = rpcId
	reply.IsResponse = true
	s.SetReply(reply) // 不应 panic
}

// ============================================================
// Actor 创建与基础操作
// ============================================================

func newTestActor() *Actor {
	cfg := zmodel.ActorConfig{
		Id:        10001,
		Name:      "TestActor",
		ActorType: 1,
		Index:     1,
		Process:   1,
	}
	a := NewActor(cfg)
	a.SetIActor(a)
	return a
}

func TestActor_Create(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	if a.GetActorId() != 10001 {
		t.Fatalf("expected actorId=10001, got %d", a.GetActorId())
	}
	if a.GetActorType() != 1 {
		t.Fatalf("expected actorType=1, got %d", a.GetActorType())
	}
	if a.GetLogger() == nil {
		t.Fatal("expected non-nil logger")
	}
	if a.GetHandleMgr() == nil {
		t.Fatal("expected non-nil handle manager")
	}
}

func TestActor_Push_NormalMessage(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	msg := zmsg.GetMessage()
	msg.MsgId = 100

	a.Push(zmodel.ActorCmd{
		Type: zmodel.CmdTypeMsg,
		Msg:  msg,
	})

	// 消息进入 mailBoxQueue，从中取出验证
	msgs := make([]zmodel.ActorCmd, 10)
	n := a.mailBoxQueue.DequeueBatch(msgs)
	if n != 1 {
		t.Fatalf("expected 1 message in queue, got %d", n)
	}
	if msgs[0].Msg.MsgId != 100 {
		t.Fatalf("expected MsgId=100, got %d", msgs[0].Msg.MsgId)
	}
	msgs[0].Release()
}

func TestActor_Push_SafeFn(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	a.Push(zmodel.ActorCmd{
		Type: zmodel.CmdTypeSafeFn,
		Fn:   func() {},
	})

	msgs := make([]zmodel.ActorCmd, 10)
	n := a.mailBoxQueue.DequeueBatch(msgs)
	if n != 1 {
		t.Fatalf("expected 1 cmd in queue, got %d", n)
	}
	if msgs[0].Type != zmodel.CmdTypeSafeFn {
		t.Fatalf("expected CmdTypeSafeFn, got %d", msgs[0].Type)
	}
}

func TestActor_RegisterTickFn(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	a.RegisterTickFn("test_update", 1*time.Second, func(ctx context.Context, nowTs int64) {})

	msgs := make([]zmodel.ActorCmd, 10)
	n := a.mailBoxQueue.DequeueBatch(msgs)
	if n != 1 {
		t.Fatalf("expected 1 cmd, got %d", n)
	}
	if msgs[0].Type != zmodel.CmdTypeTickFn {
		t.Fatalf("expected CmdTypeTickFn, got %d", msgs[0].Type)
	}
}

func TestActor_SafeHandleMessage_Tick(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	tickCalled := false
	a.registerTickFn(&zmodel.TickFnItem{
		Name: "test_tick",
		Do: func(ctx context.Context, nowTs int64) {
			tickCalled = true
		},
	})

	cmd := zmodel.ActorCmd{
		Type:    zmodel.CmdTypeTick,
		TickNow: time.Now().UnixMilli(),
	}
	a.SafeHandleMessage(context.Background(), cmd, time.Now().UnixMilli())

	if !tickCalled {
		t.Fatal("tick update function was not called")
	}
}

func TestActor_SafeHandleMessage_SafeFn(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	executed := false
	cmd := zmodel.ActorCmd{
		Type: zmodel.CmdTypeSafeFn,
		Fn:   func() { executed = true },
	}
	a.SafeHandleMessage(context.Background(), cmd, time.Now().UnixMilli())

	if !executed {
		t.Fatal("SafeFn was not executed")
	}
}

func TestActor_SafeHandleMessage_ClientMsg(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	handlerCalled := false
	a.GetHandleMgr().RegisterHandle(500, func(ctx context.Context, msg *zmsg.Message) {
		handlerCalled = true
	})

	msg := zmsg.GetMessage()
	msg.MsgId = 500
	msg.FromClient = true

	cmd := zmodel.ActorCmd{
		Type: zmodel.CmdTypeClient,
		Msg:  msg,
	}
	a.SafeHandleMessage(context.Background(), cmd, time.Now().UnixMilli())

	if !handlerCalled {
		t.Fatal("client message handler was not called")
	}
	msg.Release()
}

func TestActor_Update_IntervalControl(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	callCount := 0
	a.registerTickFn(&zmodel.TickFnItem{
		Name:     "interval_test",
		Interval: 100 * time.Millisecond,
		LastTime: 0,
		Do: func(ctx context.Context, nowTs int64) {
			callCount++
		},
	})

	now := time.Now().UnixMilli()

	// 第一次调用 — LastTime=0，应执行
	a.Update(context.Background(), now)
	if callCount != 1 {
		t.Fatalf("expected 1 call, got %d", callCount)
	}

	// 立即再调用 — 间隔不够，不应执行
	a.Update(context.Background(), now+10)
	if callCount != 1 {
		t.Fatalf("expected still 1 call (interval not passed), got %d", callCount)
	}

	// 100ms 后调用 — 应执行
	a.Update(context.Background(), now+110)
	if callCount != 2 {
		t.Fatalf("expected 2 calls, got %d", callCount)
	}
}

func TestActor_WorkerPoolStats(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	cap, running, free := a.GetWorkerPoolStats()
	if cap != 500 {
		t.Fatalf("expected pool cap=500, got %d", cap)
	}
	if running != 0 {
		t.Fatalf("expected 0 running, got %d", running)
	}
	if free != 500 {
		t.Fatalf("expected 500 free, got %d", free)
	}
}

func TestActor_Close(t *testing.T) {
	a := newTestActor()
	err := a.Close(context.Background())
	if err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	// 重复 Close 不应 panic（channel 已关闭）
	// 但第二次 close(closeCh) 会 panic，所以这里只测试一次
}

// ============================================================
// Group — 基础 CRUD
// ============================================================

func TestGroup_AddAndGetActor(t *testing.T) {
	g := NewGroup(1, true)

	a := newTestActor()
	g.AddActor(a)

	found := g.GetActorById(10001)
	if found == nil {
		t.Fatal("expected to find actor 10001")
	}
	if found.GetActorId() != 10001 {
		t.Fatalf("expected actorId=10001, got %d", found.GetActorId())
	}

	a.Close(context.Background())
}

func TestGroup_GetActorById_NotFound(t *testing.T) {
	g := NewGroup(1, true)

	found := g.GetActorById(99999)
	if found != nil {
		t.Fatal("expected nil for non-existent actor")
	}
}

func TestGroup_GetActors(t *testing.T) {
	g := NewGroup(1, true)

	a1 := newTestActorWithId(10001, 1)
	a2 := newTestActorWithId(10002, 2)
	g.AddActor(a1)
	g.AddActor(a2)

	actors := g.GetActors()
	if len(actors) != 2 {
		t.Fatalf("expected 2 actors, got %d", len(actors))
	}

	a1.Close(context.Background())
	a2.Close(context.Background())
}

func TestGroup_IsSingle(t *testing.T) {
	g1 := NewGroup(1, true)
	if !g1.IsSingle() {
		t.Fatal("expected IsSingle=true")
	}

	g2 := NewGroup(1, false)
	if g2.IsSingle() {
		t.Fatal("expected IsSingle=false")
	}
}

func TestGroup_FindPoolActorByType(t *testing.T) {
	g := NewGroup(1, true)

	a1 := newTestActorWithId(10001, 1)
	a2 := newTestActorWithId(10002, 1)
	g.AddActor(a1)
	g.AddActor(a2)

	cfg, err := g.FindPoolActorByType(1)
	if err != nil {
		t.Fatalf("FindPoolActorByType failed: %v", err)
	}
	if cfg.ActorType != 1 {
		t.Fatalf("expected actorType=1, got %d", cfg.ActorType)
	}

	a1.Close(context.Background())
	a2.Close(context.Background())
}

func TestGroup_FindPoolActorByType_NotFound(t *testing.T) {
	g := NewGroup(1, true)

	_, err := g.FindPoolActorByType(999)
	if err == nil {
		t.Fatal("expected error for non-existent actor type")
	}
}

func TestGroup_ScriptEngine(t *testing.T) {
	g := NewGroup(1, true)

	// 未注册时返回 nil
	if g.GetScriptEngine(ziface.ScriptEngineLua) != nil {
		t.Fatal("expected nil for unregistered engine")
	}

	// nil engine 不应注册
	g.SetScriptEngine(ziface.ScriptEngineLua, nil)
	if g.GetScriptEngine(ziface.ScriptEngineLua) != nil {
		t.Fatal("expected nil after setting nil engine")
	}

	// CloseScriptEngines 不应 panic（空列表）
	g.CloseScriptEngines()
}

// ============================================================
// 辅助函数
// ============================================================

func newTestActorWithId(id uint64, actorType uint32) *Actor {
	cfg := zmodel.ActorConfig{
		Id:        id,
		Name:      "TestActor",
		ActorType: actorType,
		Index:     1,
		Process:   1,
	}
	a := NewActor(cfg)
	a.SetIActor(a)
	return a
}
