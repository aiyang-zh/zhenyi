package zactor

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi/ziface"
	"github.com/aiyang-zh/zhenyi/zmodel"
	"github.com/aiyang-zh/zhenyi/zmsg"
)

func init() {
	// benchmark 进程内禁用文件日志，避免 rotatelogs lock 冲突
	cfg := zlog.NewDefaultLoggerConfig()
	cfg.Logs = map[string]int{}
	cfg.IsConsole = false
	zlog.NewDefaultLoggerWithConfig(cfg)
}

// ============================================================
// Handle 基准测试
// ============================================================

func BenchmarkHandle_GetClientHandle(b *testing.B) {
	h := newTestHandle()
	h.RegisterHandle(100, func(ctx context.Context, msg *zmsg.Message) {})

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = h.GetClientHandle(100)
	}
}

func BenchmarkHandle_HandleClientMessage(b *testing.B) {
	h := newTestHandle()
	h.RegisterHandle(100, func(ctx context.Context, msg *zmsg.Message) {})

	msg := zmsg.GetMessage()
	msg.MsgId = 100
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		h.HandleClientMessage(ctx, msg)
	}
	msg.Release()
}

// ============================================================
// Dispatcher 基准测试
// ============================================================

func BenchmarkDispatcher_Dispatch_Processor(b *testing.B) {
	cfg := zmodel.ActorConfig{Id: 1, Name: "bench", ActorType: 1, Index: 1}
	a := NewActor(cfg)
	proc := &mockProcessor{}
	a.GetDispatcher().Register(100, func(ctx context.Context, msg *zmsg.Message) ziface.IMessage {
		return proc.Process(ctx, msg)
	})

	msg := zmsg.GetMessage()
	msg.MsgId = 100
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		a.GetDispatcher().Dispatch(ctx, msg)
	}
	msg.Release()
}

func BenchmarkDispatcher_Dispatch_MsgHandler(b *testing.B) {
	cfg := zmodel.ActorConfig{Id: 1, Name: "bench", ActorType: 1, Index: 1}
	a := NewActor(cfg)

	a.GetDispatcher().Register(200, func(ctx context.Context, msg *zmsg.Message) ziface.IMessage {
		return nil
	})

	msg := zmsg.GetMessage()
	msg.MsgId = 200
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		a.GetDispatcher().Dispatch(ctx, msg)
	}
	msg.Release()
}

// ============================================================
// Actor Push 基准测试
// ============================================================

func BenchmarkActor_Push1(b *testing.B) {
	a := newTestActor()
	defer a.Close(context.Background())

	// 启动一个消费者 goroutine 消费队列
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	defer func() {
		cancel()
		wg.Wait()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		msgs := make([]zmodel.ActorCmd, 200)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				a.mailBoxQueue.DequeueBatch(msgs)
			}
		}
	}()
	msg := zmsg.GetMessage()
	msg.MsgId = 100
	cmd := zmodel.ActorCmd{
		Type: zmodel.CmdTypeMsg,
		Msg:  msg,
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		a.Push(cmd)
	}
	msg.Release()
}
func BenchmarkActor_Push(b *testing.B) {
	a := newTestActor()
	defer a.Close(context.Background())

	// 启动一个消费者 goroutine 消费队列
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	defer func() {
		cancel()
		wg.Wait()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		msgs := make([]zmodel.ActorCmd, 200)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				n := a.mailBoxQueue.DequeueBatch(msgs)
				for i := 0; i < n; i++ {
					msgs[i].Release()
				}
			}
		}
	}()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		msg := zmsg.GetMessage()
		msg.MsgId = 100
		a.Push(zmodel.ActorCmd{
			Type: zmodel.CmdTypeMsg,
			Msg:  msg,
		})
	}
}

func BenchmarkActor_Push_Parallel(b *testing.B) {
	a := newTestActor()
	defer a.Close(context.Background())

	// 消费者
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	defer func() {
		cancel()
		wg.Wait()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		msgs := make([]zmodel.ActorCmd, 200)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				n := a.mailBoxQueue.DequeueBatch(msgs)
				for i := 0; i < n; i++ {
					msgs[i].Release()
				}
			}
		}
	}()

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			msg := zmsg.GetMessage()
			msg.MsgId = 100
			a.Push(zmodel.ActorCmd{
				Type: zmodel.CmdTypeMsg,
				Msg:  msg,
			})
		}
	})
}

// ============================================================
// SafeHandleMessage 基准测试
// ============================================================

func BenchmarkActor_SafeHandleMessage_SafeFn(b *testing.B) {
	a := newTestActor()
	defer a.Close(context.Background())

	ctx := context.Background()
	nowTs := int64(0)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		a.SafeHandleMessage(ctx, zmodel.ActorCmd{
			Type: zmodel.CmdTypeSafeFn,
			Fn:   func() {},
		}, nowTs)
	}
}

func BenchmarkActor_SafeHandleMessage_Tick(b *testing.B) {
	a := newTestActor()
	defer a.Close(context.Background())

	// 注册一个轻量 update
	a.registerTickFn(&zmodel.TickFnItem{
		Name: "bench_tick",
		Do:   func(ctx context.Context, nowTs int64) {},
	})

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		a.SafeHandleMessage(ctx, zmodel.ActorCmd{
			Type:    zmodel.CmdTypeTick,
			TickNow: int64(i),
		}, int64(i))
	}
}

// ============================================================
// ActorMsgSender 基准测试
// ============================================================

func BenchmarkSender_AddSender(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := NewSender(ctx, 0)

	// 预热 + 释放所有 slot
	for i := 0; i < DefaultMaxPendingRPCs/2; i++ {
		id, _ := s.AddSender()
		// 模拟快速 reply + get 释放 slot
		reply := zmsg.GetMessage()
		reply.RpcId = id
		reply.IsResponse = true
		s.SetReply(reply)
		reply.Release()
		data, _ := s.GetReply(id, 1*time.Millisecond)
		if data != nil {
			data.Release()
		}
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		id, err := s.AddSender()
		if err != nil {
			b.Fatalf("AddSender failed at %d: %v", i, err)
		}
		// 释放 slot（不然会耗尽）
		reply := zmsg.GetMessage()
		reply.RpcId = id
		reply.IsResponse = true
		s.SetReply(reply)
		reply.Release()
		data, _ := s.GetReply(id, 1*time.Millisecond)
		if data != nil {
			data.Release()
		}
	}
}

func BenchmarkSender_SetReply(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := NewSender(ctx, 0)

	// 预先分配一个 RPC，循环 SetReply + GetReply
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		id, err := s.AddSender()
		if err != nil {
			b.Fatalf("AddSender failed: %v", err)
		}
		reply := zmsg.GetMessage()
		reply.RpcId = id
		reply.IsResponse = true
		s.SetReply(reply)
		reply.Release()
		data, _ := s.GetReply(id, 1*time.Millisecond)
		if data != nil {
			data.Release()
		}
	}
}

// ============================================================
// Group 基准测试
// ============================================================

func BenchmarkGroup_GetActorById(b *testing.B) {
	g := NewGroup(1, true)
	a := newTestActor()
	g.AddActor(a)
	defer a.Close(context.Background())

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = g.GetActorById(10001)
	}
}

// ============================================================
// AsyncRunWithMsg / AsyncRun 零分配基准测试
// ============================================================

// primeAsyncTaskPool 向 asyncTask 的 sync.Pool 灌入 n 个独立对象。
//
// 若循环里 Get 后立即 Put，池中始终约保留 1 个对象；AsyncRun 流水线会同时在 worker 与 mailbox
// 回调路径上占用 task（见 pprof：initAsyncTaskPool.func1.1 占 alloc 大头），导致 getAsyncTask 频繁走 New。
func primeAsyncTaskPool(n int) {
	initAsyncTaskPool()
	tasks := make([]*asyncTask, n)
	for i := 0; i < n; i++ {
		tasks[i] = getAsyncTask()
	}
	for i := 0; i < n; i++ {
		putAsyncTask(tasks[i])
	}
}

// primeActorMailboxNodes 在启动 drain 协程之前灌满 mailbox 的 MPSC 节点池（pprof：NewUnboundedMPSC.func1）。
// 仅做 Enqueue/DequeueBatch，不经过 Actor.Run；Tick 命令无 Msg，无需 Release。
func primeActorMailboxNodes(a *Actor, n int) {
	buf := make([]zmodel.ActorCmd, 512)
	for i := 0; i < n; i++ {
		a.mailBoxQueue.Enqueue(zmodel.ActorCmd{Type: zmodel.CmdTypeTick, TickNow: 1})
	}
	for !a.mailBoxQueue.Empty() {
		a.mailBoxQueue.DequeueBatch(buf)
	}
}

// warmupAsyncBench 在 ResetTimer 之前填充 asyncTask 池并跑若干轮真实 AsyncRun，
// 使 asyncTask / mailbox MPSC 节点池进入稳态（pprof mem 中冷启动扩容不计入计时区间）。
func warmupAsyncBench(a *Actor, withMsg bool, msg *zmsg.Message) {
	// 与默认 Actor worker 池规模同量级，避免稳态仍因池内对象不足而分配（见 primeAsyncTaskPool 注释）。
	primeAsyncTaskPool(8192)
	const cycles = 3000
	for i := 0; i < cycles; i++ {
		if withMsg {
			a.AsyncRunWithMsg(msg, func(m *zmsg.Message) interface{} { return nil }, func(interface{}) {})
		} else {
			a.AsyncRun(func() interface{} { return nil }, func(interface{}) {})
		}
	}
}

func BenchmarkAsyncTaskPool_GetPut(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		t := getAsyncTask()
		putAsyncTask(t)
	}
}

// BenchmarkAsyncRunWithMsg 使用「慢消费」防止 mailbox 堆积：消费者每次 Dequeue 后 Sleep 100µs，
// 会人为制造背压，测的是偏调度/队列积压下的路径（与 FastDrain 对照）。
func BenchmarkAsyncRunWithMsg(b *testing.B) {
	a := newTestActor()
	defer a.Close(context.Background())
	primeActorMailboxNodes(a, 50000)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	defer func() {
		cancel()
		wg.Wait()
	}()

	// 消费 Actor mailbox 防止堆积
	wg.Add(1)
	go func() {
		defer wg.Done()
		msgs := make([]zmodel.ActorCmd, 200)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				a.mailBoxQueue.DequeueBatch(msgs)
				time.Sleep(100 * time.Microsecond)
			}
		}
	}()

	msg := zmsg.GetMessage()
	msg.MsgId = 1
	defer msg.Release()

	warmupAsyncBench(a, true, msg)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		a.AsyncRunWithMsg(msg, func(m *zmsg.Message) interface{} {
			return nil
		}, func(res interface{}) {})
	}
}

// BenchmarkAsyncRunWithMsg_FastDrain 消费者全速 DequeueBatch，不 Sleep，用于与 BenchmarkAsyncRunWithMsg
// 对比「背压 vs 无背压」下的 ns/op 差异。
func BenchmarkAsyncRunWithMsg_FastDrain(b *testing.B) {
	a := newTestActor()
	defer a.Close(context.Background())
	primeActorMailboxNodes(a, 50000)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	defer func() {
		cancel()
		wg.Wait()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		msgs := make([]zmodel.ActorCmd, 200)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				a.mailBoxQueue.DequeueBatch(msgs)
			}
		}
	}()

	msg := zmsg.GetMessage()
	msg.MsgId = 1
	defer msg.Release()

	warmupAsyncBench(a, true, msg)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		a.AsyncRunWithMsg(msg, func(m *zmsg.Message) interface{} {
			return nil
		}, func(res interface{}) {})
	}
}

// BenchmarkAsyncRun 同 BenchmarkAsyncRunWithMsg：慢消费 + Sleep，偏背压场景。
func BenchmarkAsyncRun(b *testing.B) {
	a := newTestActor()
	defer a.Close(context.Background())
	primeActorMailboxNodes(a, 50000)
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	defer func() {
		cancel()
		wg.Wait()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		msgs := make([]zmodel.ActorCmd, 200)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				a.mailBoxQueue.DequeueBatch(msgs)
				time.Sleep(100 * time.Microsecond)
			}
		}
	}()

	warmupAsyncBench(a, false, nil)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		a.AsyncRun(func() interface{} {
			return nil
		}, func(res interface{}) {})
	}
}

// BenchmarkAsyncRun_FastDrain 同 BenchmarkAsyncRunWithMsg_FastDrain：无 Sleep，全速 drain。
func BenchmarkAsyncRun_FastDrain(b *testing.B) {
	a := newTestActor()
	defer a.Close(context.Background())
	primeActorMailboxNodes(a, 50000)
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	defer func() {
		cancel()
		wg.Wait()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		msgs := make([]zmodel.ActorCmd, 200)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				a.mailBoxQueue.DequeueBatch(msgs)
			}
		}
	}()

	warmupAsyncBench(a, false, nil)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		a.AsyncRun(func() interface{} {
			return nil
		}, func(res interface{}) {})
	}
}
