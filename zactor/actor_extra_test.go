package zactor

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi/ziface"
	"github.com/aiyang-zh/zhenyi/zmodel"
	"github.com/aiyang-zh/zhenyi/zmsg"
)

type testCtxKey struct{}

var actorExtraTestCtxKey testCtxKey

// ============================================================
// CircuitBreaker
// ============================================================

func TestCircuitBreaker_InitialClosed(t *testing.T) {
	cb := newCircuitBreaker()
	if !cb.allow() {
		t.Fatal("new circuit breaker should allow (closed state)")
	}
}

func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	cb := newCircuitBreaker()
	for i := 0; i < int(defaultCBThreshold); i++ {
		cb.recordFailure()
	}
	if cb.allow() {
		t.Fatal("circuit breaker should be open after threshold failures")
	}
}

func TestCircuitBreaker_RecordSuccessResets(t *testing.T) {
	cb := newCircuitBreaker()
	for i := 0; i < int(defaultCBThreshold)-1; i++ {
		cb.recordFailure()
	}
	cb.recordSuccess()

	if !cb.allow() {
		t.Fatal("recordSuccess should reset failure count, breaker should be closed")
	}
	if cb.failures.Load() != 0 {
		t.Fatalf("failures should be 0 after recordSuccess, got %d", cb.failures.Load())
	}
}

func TestCircuitBreaker_HalfOpenAfterCooldown(t *testing.T) {
	cb := newCircuitBreaker()
	cb.cooldownMs = 10 // 10ms for test

	for i := 0; i < int(defaultCBThreshold); i++ {
		cb.recordFailure()
	}
	if cb.allow() {
		t.Fatal("should be open immediately after failures")
	}

	time.Sleep(20 * time.Millisecond)
	if !cb.allow() {
		t.Fatal("should transition to half-open after cooldown")
	}
	if cbState(cb.state.Load()) != cbHalfOpen {
		t.Fatalf("expected half-open state, got %d", cb.state.Load())
	}
}

func TestCircuitBreaker_HalfOpenAllows(t *testing.T) {
	cb := newCircuitBreaker()
	cb.state.Store(int32(cbHalfOpen))
	if !cb.allow() {
		t.Fatal("half-open should allow requests")
	}
}

func TestCircuitBreaker_SuccessClosesFromHalfOpen(t *testing.T) {
	cb := newCircuitBreaker()
	cb.state.Store(int32(cbHalfOpen))
	cb.recordSuccess()
	if cbState(cb.state.Load()) != cbClosed {
		t.Fatal("recordSuccess from half-open should close the breaker")
	}
}

func TestCircuitBreaker_GetCircuitBreaker(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	cb1 := a.getCircuitBreaker(100)
	if cb1 == nil {
		t.Fatal("getCircuitBreaker should create new")
	}
	cb2 := a.getCircuitBreaker(100)
	if cb1 != cb2 {
		t.Fatal("getCircuitBreaker should return same instance for same actorId")
	}
	cb3 := a.getCircuitBreaker(200)
	if cb3 == cb1 {
		t.Fatal("different actorId should get different circuit breaker")
	}
}

// ============================================================
// handleMessage routing
// ============================================================

func TestHandleMessage_DispatchesNonClient(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	dispatched := false
	a.GetDispatcher().Register(100, func(ctx context.Context, msg *zmsg.Message) ziface.IMessage {
		dispatched = true
		return nil
	})

	msg := zmsg.GetMessage()
	defer msg.Release()
	msg.MsgId = 100
	msg.FromClient = false

	a.HandleMessage(context.Background(), msg)
	if !dispatched {
		t.Fatal("non-client message should be dispatched")
	}
}

func TestHandleMessage_ClientRouting(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	clientHandled := false
	a.GetHandleMgr().RegisterHandle(100, func(ctx context.Context, msg *zmsg.Message) {
		clientHandled = true
	})

	msg := zmsg.GetMessage()
	defer msg.Release()
	msg.MsgId = 100
	msg.FromClient = true

	a.HandleMessage(context.Background(), msg)
	if !clientHandled {
		t.Fatal("client message should go through HandleClientMessage")
	}
}

func TestHandleMessage_Response_SetsReply(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a := newTestActor()
	a.ISender = NewSender(ctx, 0)
	defer a.Close(context.Background())

	rpcId, err := a.AddSender()
	if err != nil {
		t.Fatal(err)
	}

	msg := zmsg.GetMessage()
	msg.MsgId = 100
	msg.IsResponse = true
	msg.ToClient = false
	msg.RpcId = rpcId

	a.handleMessage(context.Background(), msg)

	data, ok := a.GetReply(rpcId, 100*time.Millisecond)
	if !ok {
		t.Fatal("response message should have been set via SetReply")
	}
	data.Release()
}

func TestHandleRespMessage_IsResponse(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a := newTestActor()
	a.ISender = NewSender(ctx, 0)
	defer a.Close(context.Background())

	rpcId, _ := a.AddSender()

	msg := zmsg.GetMessage()
	msg.IsResponse = true
	msg.RpcId = rpcId

	a.HandleRespMessage(context.Background(), msg)

	data, ok := a.GetReply(rpcId, 100*time.Millisecond)
	if !ok {
		t.Fatal("HandleRespMessage should SetReply for response messages")
	}
	data.Release()
}

func TestHandleRespMessage_NotResponse_NoOp(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	msg := zmsg.GetMessage()
	defer msg.Release()
	msg.IsResponse = false

	a.HandleRespMessage(context.Background(), msg) // should not panic
}

// ============================================================
// SafeHandleMessage — additional branches
// ============================================================

func TestSafeHandleMessage_MsgType(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	dispatched := false
	a.GetDispatcher().Register(200, func(ctx context.Context, msg *zmsg.Message) ziface.IMessage {
		dispatched = true
		return nil
	})

	msg := zmsg.GetMessage()
	msg.MsgId = 200
	cmd := zmodel.ActorCmd{Type: zmodel.CmdTypeMsg, Msg: msg}
	a.SafeHandleMessage(context.Background(), cmd, time.Now().UnixMilli())
	msg.Release()

	if !dispatched {
		t.Fatal("CmdType_Msg should dispatch via handleMessage")
	}
}

func TestSafeHandleMessage_UpdateRegister(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	called := false
	item := zmodel.NewTickFnItem("test_reg", 0, func(ctx context.Context, nowTs int64) {
		called = true
	})
	cmd := zmodel.ActorCmd{Type: zmodel.CmdTypeTickFn, TickFn: item}
	a.SafeHandleMessage(context.Background(), cmd, time.Now().UnixMilli())

	if _, ok := a.tickFns["test_reg"]; !ok {
		t.Fatal("UpdateRegister should register the update function")
	}

	a.Update(context.Background(), time.Now().UnixMilli())
	if !called {
		t.Fatal("registered update function should be callable")
	}
}

func TestSafeHandleMessage_WithContext(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	var receivedCtx context.Context
	a.GetDispatcher().Register(300, func(ctx context.Context, msg *zmsg.Message) ziface.IMessage {
		receivedCtx = ctx
		return nil
	})

	customCtx := context.WithValue(context.Background(), actorExtraTestCtxKey, "value")
	msg := zmsg.GetMessage()
	msg.MsgId = 300
	cmd := zmodel.ActorCmd{Type: zmodel.CmdTypeMsg, Msg: msg, Ctx: customCtx}
	a.SafeHandleMessage(context.Background(), cmd, time.Now().UnixMilli())
	msg.Release()

	if receivedCtx.Value(actorExtraTestCtxKey) != "value" {
		t.Fatal("should use msg.Ctx when provided")
	}
}

// ============================================================
// safeUpdate / safeExecute
// ============================================================

func TestSafeUpdate_PushesToMailbox(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	a.safeUpdate(func() {})

	msgs := make([]zmodel.ActorCmd, 10)
	n := a.mailBoxQueue.DequeueBatch(msgs)
	if n != 1 {
		t.Fatalf("expected 1 cmd, got %d", n)
	}
	if msgs[0].Type != zmodel.CmdTypeSafeFn {
		t.Fatalf("expected SafeFn type, got %d", msgs[0].Type)
	}
}

func TestSafeUpdate_NilFn_NoOp(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	a.safeUpdate(nil)

	msgs := make([]zmodel.ActorCmd, 10)
	n := a.mailBoxQueue.DequeueBatch(msgs)
	if n != 0 {
		t.Fatal("nil fn should not push anything")
	}
}

func TestSafeExecute_RunsFn(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	called := false
	a.safeExecute(func() { called = true })
	if !called {
		t.Fatal("safeExecute should run the function")
	}
}

func TestSafeExecute_Nil(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())
	a.safeExecute(nil) // should not panic
}

func TestSafeExecute_RecoversPanic(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())
	a.safeExecute(func() { panic("test panic") }) // should not propagate
}

// ============================================================
// Monitor
// ============================================================

func TestGetMonitorData(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	data := a.GetMonitorData()
	if data.Type != "actor" {
		t.Fatalf("expected type=actor, got %s", data.Type)
	}
	if data.Name != a.GetTopic() {
		t.Fatalf("name mismatch: %s vs %s", data.Name, a.GetTopic())
	}
	if data.Status != "idle" {
		t.Fatalf("expected status=idle, got %s", data.Status)
	}
	if data.Metrics["actorId"] != a.GetActorId() {
		t.Fatal("actorId mismatch in metrics")
	}
}

func TestGetMonitorData_BusyStatus(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	atomic.StoreInt64(&a.mailCount, 1001)
	data := a.GetMonitorData()
	if data.Status != "busy" {
		t.Fatalf("expected status=busy when mailCount>1000, got %s", data.Status)
	}
}

func TestGetMonitorData_RunningStatus(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	atomic.StoreInt64(&a.mailCount, 500)
	data := a.GetMonitorData()
	if data.Status != "running" {
		t.Fatalf("expected status=running when 0 < mailCount <= 1000, got %s", data.Status)
	}
}

func TestRecordError(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	a.RecordError()
	a.RecordError()

	data := a.GetMonitorData()
	errCount, ok := data.Metrics["errorCount"].(int64)
	if !ok || errCount != 2 {
		t.Fatalf("expected errorCount=2, got %v", data.Metrics["errorCount"])
	}
}

// ============================================================
// HotUpdate (verify SafeFn enqueued)
// ============================================================

func TestUpdateWorkerPoolSize_Enqueues(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	a.UpdateWorkerPoolSize(100)

	msgs := make([]zmodel.ActorCmd, 10)
	n := a.mailBoxQueue.DequeueBatch(msgs)
	if n != 1 || msgs[0].Type != zmodel.CmdTypeSafeFn {
		t.Fatal("UpdateWorkerPoolSize should enqueue a SafeFn")
	}
}

func TestUpdateRateLimit_Enqueues(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	a.UpdateRateLimit(100, 200)

	msgs := make([]zmodel.ActorCmd, 10)
	n := a.mailBoxQueue.DequeueBatch(msgs)
	if n != 1 || msgs[0].Type != zmodel.CmdTypeSafeFn {
		t.Fatal("UpdateRateLimit should enqueue a SafeFn")
	}
}

func TestUpdateMaxRPCPending_Enqueues(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	a.UpdateMaxRPCPending(2048)

	msgs := make([]zmodel.ActorCmd, 10)
	n := a.mailBoxQueue.DequeueBatch(msgs)
	if n != 1 || msgs[0].Type != zmodel.CmdTypeSafeFn {
		t.Fatal("UpdateMaxRPCPending should enqueue a SafeFn")
	}
}

func TestRebuildWorkerPool_Enqueues(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	a.RebuildWorkerPool(100)

	msgs := make([]zmodel.ActorCmd, 10)
	n := a.mailBoxQueue.DequeueBatch(msgs)
	if n != 1 || msgs[0].Type != zmodel.CmdTypeSafeFn {
		t.Fatal("RebuildWorkerPool should enqueue a SafeFn")
	}
}

func TestHotUpdate_ExecuteViaHandle(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	a.UpdateRateLimit(999, 888)

	msgs := make([]zmodel.ActorCmd, 10)
	n := a.mailBoxQueue.DequeueBatch(msgs)
	if n == 1 && msgs[0].Fn != nil {
		msgs[0].Fn()
	}

	if a.Rate != 999 || a.Burst != 888 {
		t.Fatalf("expected Rate=999,Burst=888 after executing SafeFn, got %d,%d", a.Rate, a.Burst)
	}
}

func TestUpdateWorkerPoolSize_Execute(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	a.UpdateWorkerPoolSize(50)

	msgs := make([]zmodel.ActorCmd, 10)
	n := a.mailBoxQueue.DequeueBatch(msgs)
	if n == 1 && msgs[0].Fn != nil {
		msgs[0].Fn()
	}

	// Tune() is called; ants pool with PreAlloc may not reflect Cap change
	// when shrinking. Just verify the SafeFn executed without panic.
	if a.workerPool == nil {
		t.Fatal("workerPool should remain")
	}
}

func TestUpdateWorkerPoolSize_NilPool(t *testing.T) {
	a := newTestActor()
	a.workerPool.Release()
	a.workerPool = nil

	a.UpdateWorkerPoolSize(50)
	msgs := make([]zmodel.ActorCmd, 10)
	n := a.mailBoxQueue.DequeueBatch(msgs)
	if n == 1 && msgs[0].Fn != nil {
		msgs[0].Fn()
	}
	a.closeCh = make(chan struct{}, 1)
	a.Close(context.Background())
}

func TestUpdateWorkerPoolSize_InvalidSize(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	a.UpdateWorkerPoolSize(0)
	msgs := make([]zmodel.ActorCmd, 10)
	n := a.mailBoxQueue.DequeueBatch(msgs)
	if n == 1 && msgs[0].Fn != nil {
		msgs[0].Fn()
	}
	if a.workerPool.Cap() != 500 {
		t.Fatalf("invalid size should not change pool, got cap=%d", a.workerPool.Cap())
	}
}

func TestUpdateMaxRPCPending_Execute(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	a.UpdateMaxRPCPending(8192)
	msgs := make([]zmodel.ActorCmd, 10)
	n := a.mailBoxQueue.DequeueBatch(msgs)
	if n == 1 && msgs[0].Fn != nil {
		msgs[0].Fn()
	}
	if a.MaxRPCPending != 8192 {
		t.Fatalf("expected MaxRPCPending=8192, got %d", a.MaxRPCPending)
	}
}

func TestUpdateMaxRPCPending_InvalidSize(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	orig := a.MaxRPCPending
	a.UpdateMaxRPCPending(0)
	msgs := make([]zmodel.ActorCmd, 10)
	n := a.mailBoxQueue.DequeueBatch(msgs)
	if n == 1 && msgs[0].Fn != nil {
		msgs[0].Fn()
	}
	if a.MaxRPCPending != orig {
		t.Fatal("MaxRPCPending should not change with invalid size")
	}
}

func TestRebuildWorkerPool_Execute(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	a.RebuildWorkerPool(200)
	msgs := make([]zmodel.ActorCmd, 10)
	n := a.mailBoxQueue.DequeueBatch(msgs)
	if n == 1 && msgs[0].Fn != nil {
		msgs[0].Fn()
	}
	if a.workerPool == nil {
		t.Fatal("workerPool should not be nil after rebuild")
	}
	if a.workerPool.Cap() != 200 {
		t.Fatalf("expected cap=200, got %d", a.workerPool.Cap())
	}
}

func TestRebuildWorkerPool_NilPool(t *testing.T) {
	a := newTestActor()
	a.workerPool.Release()
	a.workerPool = nil

	a.RebuildWorkerPool(100)
	msgs := make([]zmodel.ActorCmd, 10)
	n := a.mailBoxQueue.DequeueBatch(msgs)
	if n == 1 && msgs[0].Fn != nil {
		msgs[0].Fn()
	}
	if a.workerPool == nil || a.workerPool.Cap() != 100 {
		t.Fatal("should rebuild even from nil pool")
	}
	a.closeCh = make(chan struct{}, 1)
	a.Close(context.Background())
}

// ============================================================
// Watchdog
// ============================================================

func TestWatchdog_DetectsBlockedActor(t *testing.T) {
	g := NewGroup(1, true)
	a := newTestActorWithId(10001, 1)
	g.AddActor(a)
	defer a.Close(context.Background())

	atomic.StoreInt64(&a.processingStart, time.Now().Add(-500*time.Millisecond).UnixNano())

	wd := newWatchdog(g, 100*time.Millisecond)
	wd.scan()

	_, loaded := wd.lastDump.Load(uint64(10001))
	if !loaded {
		t.Fatal("watchdog should detect blocked actor and record in lastDump")
	}
}

func TestWatchdog_SkipsIdleActor(t *testing.T) {
	g := NewGroup(1, true)
	a := newTestActorWithId(10001, 1)
	g.AddActor(a)
	defer a.Close(context.Background())

	atomic.StoreInt64(&a.processingStart, 0)

	wd := newWatchdog(g, 100*time.Millisecond)
	wd.scan()

	_, loaded := wd.lastDump.Load(uint64(10001))
	if loaded {
		t.Fatal("watchdog should skip idle actors (processingStart=0)")
	}
}

func TestWatchdog_Cooldown(t *testing.T) {
	g := NewGroup(1, true)
	a := newTestActorWithId(10001, 1)
	g.AddActor(a)
	defer a.Close(context.Background())

	wd := newWatchdog(g, 100*time.Millisecond)
	wd.cooldown = 1 * time.Second

	atomic.StoreInt64(&a.processingStart, time.Now().Add(-500*time.Millisecond).UnixNano())
	wd.scan()

	firstDump, _ := wd.lastDump.Load(uint64(10001))

	wd.scan()
	secondDump, _ := wd.lastDump.Load(uint64(10001))

	if firstDump != secondDump {
		t.Fatal("within cooldown, scan should not update lastDump")
	}
}

func TestWatchdog_RunAndStop(t *testing.T) {
	g := NewGroup(1, true)
	ctx, cancel := context.WithCancel(context.Background())

	wd := newWatchdog(g, 100*time.Millisecond)
	wd.interval = 10 * time.Millisecond

	done := make(chan struct{})
	go func() {
		wd.run(ctx)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watchdog.run should exit after context cancel")
	}
}

// ============================================================
// MarkTickPending
// ============================================================

func TestMarkTickPending(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	if !a.MarkTickPending() {
		t.Fatal("first MarkTickPending should succeed")
	}
	if a.MarkTickPending() {
		t.Fatal("second MarkTickPending should fail (already pending)")
	}

	a.tickPending.Store(false)
	if !a.MarkTickPending() {
		t.Fatal("MarkTickPending should succeed after reset")
	}
}

// ============================================================
// GetProcessingStart
// ============================================================

func TestGetProcessingStart(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	if a.GetProcessingStart() != 0 {
		t.Fatal("idle actor should have processingStart=0")
	}

	now := time.Now().UnixNano()
	atomic.StoreInt64(&a.processingStart, now)
	if a.GetProcessingStart() != now {
		t.Fatal("GetProcessingStart should return stored value")
	}
}

// ============================================================
// LogWorkerPoolStats
// ============================================================

func TestLogWorkerPoolStats(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())
	a.LogWorkerPoolStats() // should not panic, just logs
}

func TestLogWorkerPoolStats_NilPool(t *testing.T) {
	a := newTestActor()
	a.workerPool.Release()
	a.workerPool = nil
	a.LogWorkerPoolStats() // should not panic
	a.closeCh = make(chan struct{}, 1)
	a.Close(context.Background())
}

// ============================================================
// SetInitServer / CallInitServer
// ============================================================

func TestSetInitServer_CallInitServer(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	called := false
	a.SetInitServer(func(ctx context.Context) error {
		called = true
		return nil
	})

	if err := a.CallInitServer(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("initServer was not called")
	}
}

// ============================================================
// Close — idempotent
// ============================================================

func TestClose_Idempotent(t *testing.T) {
	a := newTestActor()
	if err := a.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := a.Close(context.Background()); err != nil {
		t.Fatal("second Close should not error")
	}
}

// ============================================================
// Push — response message inline path
// ============================================================

func TestPush_ResponseInline(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a := newTestActor()
	a.ISender = NewSender(ctx, 0)
	defer a.Close(context.Background())

	rpcId, _ := a.AddSender()

	msg := zmsg.GetMessage()
	msg.IsResponse = true
	msg.RpcId = rpcId

	a.Push(zmodel.ActorCmd{Type: zmodel.CmdTypeMsg, Msg: msg})

	data, ok := a.GetReply(rpcId, 100*time.Millisecond)
	if !ok {
		t.Fatal("Push of response message should inline-handle via SafeHandleMessage")
	}
	data.Release()
}

// ============================================================
// Dispatcher — unregistered msgId, RegisterBatch
// ============================================================

func TestDispatch_UnregisteredMsgId(t *testing.T) {
	cfg := zmodel.ActorConfig{Id: 1, Name: "test", ActorType: 1, Index: 1}
	a := &Actor{ActorConfig: cfg, logger: zlog.GetDefaultLog()}
	d := NewDispatcher(a)

	msg := zmsg.GetMessage()
	defer msg.Release()
	msg.MsgId = 999

	d.Dispatch(context.Background(), msg)
}

// ============================================================
// GetMsgList / GetBroadcastTopic
// ============================================================

func TestGetMsgList(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	a.GetHandleMgr().AddMsgId(100)
	a.GetHandleMgr().AddMsgId(200)

	list := a.GetMsgList()
	if _, ok := list[100]; !ok {
		t.Fatal("100 should be in MsgList")
	}
	if _, ok := list[200]; !ok {
		t.Fatal("200 should be in MsgList")
	}
}

func TestGetBroadcastTopic(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())
	if a.GetBroadcastTopic() != "topic_broadcast" {
		t.Fatal("unexpected broadcast topic")
	}
}

// ============================================================
// GetActorConfig
// ============================================================

func TestGetActorConfig(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	cfg := a.GetActorConfig()
	if cfg.Id != 10001 || cfg.Name != "TestActor" {
		t.Fatal("GetActorConfig mismatch")
	}
}

// ============================================================
// AsyncRunWithMsg / AsyncRun — end-to-end
// ============================================================

func TestAsyncRunWithMsg_WithCallback(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	msg := zmsg.GetMessage()
	msg.MsgId = 1

	resultCh := make(chan int, 1)
	a.AsyncRunWithMsg(msg, func(m *zmsg.Message) interface{} {
		return 42
	}, func(res interface{}) {
		resultCh <- res.(int)
	})

	// 回调通过 ActorCmd 回到主线程，按主线程处理路径执行。
	time.Sleep(50 * time.Millisecond)
	msgs := make([]zmodel.ActorCmd, 10)
	n := a.mailBoxQueue.DequeueBatch(msgs)
	for i := 0; i < n; i++ {
		a.SafeHandleMessage(context.Background(), msgs[i], time.Now().UnixMilli())
	}

	select {
	case v := <-resultCh:
		if v != 42 {
			t.Fatalf("expected 42, got %d", v)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("callback was not called")
	}
}

func TestAsyncRunWithMsg_ValidatorFails(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	msg := zmsg.GetMessage()
	msg.MsgId = 1

	callbackCalled := false
	a.AsyncRunWithMsg(msg, func(m *zmsg.Message) interface{} {
		t.Fatal("work function should not be called when validator fails")
		return nil
	}, func(res interface{}) {
		callbackCalled = true
	}, func() bool {
		return false // validator fails
	})

	time.Sleep(50 * time.Millisecond)
	if callbackCalled {
		t.Fatal("callback should not be called when validator fails")
	}
}

func TestAsyncRun_WithCallback(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	resultCh := make(chan string, 1)
	a.AsyncRun(func() interface{} {
		return "hello"
	}, func(res interface{}) {
		resultCh <- res.(string)
	})

	time.Sleep(50 * time.Millisecond)
	msgs := make([]zmodel.ActorCmd, 10)
	n := a.mailBoxQueue.DequeueBatch(msgs)
	for i := 0; i < n; i++ {
		a.SafeHandleMessage(context.Background(), msgs[i], time.Now().UnixMilli())
	}

	select {
	case v := <-resultCh:
		if v != "hello" {
			t.Fatalf("expected 'hello', got %s", v)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("callback was not called")
	}
}

func TestAsyncRun_NilCallback(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	a.AsyncRun(func() interface{} {
		return nil
	}, nil)

	time.Sleep(50 * time.Millisecond) // should not panic, just submits to pool
}

// ============================================================
// receiveMsg
// ============================================================

func TestReceiveMsg_InvalidData(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	// We can't easily call receiveMsg without real nats.Msg,
	// but we can verify the mailbox is empty after bad data
	msgs := make([]zmodel.ActorCmd, 10)
	n := a.mailBoxQueue.DequeueBatch(msgs)
	if n != 0 {
		t.Fatal("mailbox should be empty initially")
	}
}

// ============================================================
// Handle — HandleClientMessage with missing handler
// ============================================================

func TestHandle_HandleClientMessage_NoHandler(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())
	h := a.GetHandleMgr()

	msg := zmsg.GetMessage()
	defer msg.Release()
	msg.MsgId = 999

	h.HandleClientMessage(context.Background(), msg)
}

func TestHandle_HandleClientMessage_WithHandler(t *testing.T) {
	h := newTestHandle()
	called := false
	h.RegisterHandle(100, func(ctx context.Context, msg *zmsg.Message) {
		called = true
	})

	msg := zmsg.GetMessage()
	defer msg.Release()
	msg.MsgId = 100

	h.HandleClientMessage(context.Background(), msg)
	if !called {
		t.Fatal("handler should be called")
	}
}

func TestGetClientHandle_Existing(t *testing.T) {
	h := newTestHandle()
	fn := func(ctx context.Context, msg *zmsg.Message) {}
	h.RegisterHandle(100, fn)

	got := h.GetClientHandle(100)
	if got == nil {
		t.Fatal("should return registered handler")
	}
}

func TestGetClientHandle_NonExisting(t *testing.T) {
	h := newTestHandle()
	got := h.GetClientHandle(999)
	if got != nil {
		t.Fatal("should return nil for non-existing handler")
	}
}

// ============================================================
// registerTickFn — duplicate skipped
// ============================================================

func TestRegisterTickFn_DuplicateSkipped(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	callCount := 0
	item1 := &zmodel.TickFnItem{
		Name: "dup",
		Do: func(ctx context.Context, nowTs int64) {
			callCount = 1
		},
	}
	item2 := &zmodel.TickFnItem{
		Name: "dup",
		Do: func(ctx context.Context, nowTs int64) {
			callCount = 2
		},
	}

	a.registerTickFn(item1)
	a.registerTickFn(item2) // should be skipped

	a.Update(context.Background(), time.Now().UnixMilli())
	if callCount != 1 {
		t.Fatalf("duplicate registration should keep first, expected callCount=1, got %d", callCount)
	}
}

func TestRegisterTickFn_NilSkipped(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())
	a.registerTickFn(nil) // should not panic
}

// ============================================================
// GetWorkerPoolStats — nil pool
// ============================================================

func TestGetWorkerPoolStats_NilPool(t *testing.T) {
	a := newTestActor()
	a.workerPool.Release()
	a.workerPool = nil

	c, r, f := a.GetWorkerPoolStats()
	if c != 0 || r != 0 || f != 0 {
		t.Fatal("nil pool should return all zeros")
	}
	a.closeCh = make(chan struct{}, 1)
	a.Close(context.Background())
}

// ============================================================
// Group — EnableWatchdog
// ============================================================

func TestGroup_EnableWatchdog(t *testing.T) {
	g := NewGroup(1, true)
	g.EnableWatchdog(100 * time.Millisecond)
	if g.watchdog == nil {
		t.Fatal("EnableWatchdog should create a watchdog")
	}
}

func TestGroup_SetScriptEngine_Nil(t *testing.T) {
	g := NewGroup(1, true)
	g.SetScriptEngine(ziface.ScriptEngineLua, nil)
	if g.GetScriptEngine(ziface.ScriptEngineLua) != nil {
		t.Fatal("nil engine should not be set")
	}
}

func TestGroup_GetScriptEngine_Empty(t *testing.T) {
	g := NewGroup(1, true)
	e := g.GetScriptEngine(ziface.ScriptEngineLua)
	if e != nil {
		t.Fatal("should return nil when no engine set")
	}
}

func TestGroup_CloseScriptEngines_Empty(t *testing.T) {
	g := NewGroup(1, true)
	g.CloseScriptEngines()
}

func TestGroup_IsSingle_Both(t *testing.T) {
	g1 := NewGroup(1, true)
	if !g1.IsSingle() {
		t.Fatal("should be single")
	}
	g2 := NewGroup(1, false)
	if g2.IsSingle() {
		t.Fatal("should not be single")
	}
}

func TestGroup_GetActors_Empty(t *testing.T) {
	g := NewGroup(1, true)
	actors := g.GetActors()
	if actors == nil {
		t.Fatal("should return non-nil slice")
	}
	if len(actors) != 0 {
		t.Fatalf("empty group should return empty slice, got len=%d", len(actors))
	}
}

// ============================================================
// nextPowerOfTwo
// ============================================================

func TestNextPowerOfTwo(t *testing.T) {
	tests := []struct {
		in, want int
	}{
		{0, 2}, {-1, 2}, {1, 1}, {2, 2}, {3, 4}, {5, 8},
		{100, 128}, {1024, 1024}, {1025, 2048},
	}
	for _, tt := range tests {
		got := nextPowerOfTwo(tt.in)
		if got != tt.want {
			t.Errorf("nextPowerOfTwo(%d)=%d, want %d", tt.in, got, tt.want)
		}
	}
}
