package zactor

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aiyang-zh/zhenyi/zmsg"
)

// ============================================================
// checkSlot / recycleSlot coverage tests
// ============================================================

func TestCheckSlot_FreeSlot_NoOp(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := NewSender(ctx, 16)
	slot := &s.slots[0]
	atomic.StoreInt32(&slot.state, SlotFree)
	s.checkSlot(slot, time.Now().UnixMilli())
	if atomic.LoadInt32(&slot.state) != SlotFree {
		t.Fatal("free slot should stay free")
	}
}

func TestCheckSlot_AbandonedSlot_Expired(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := NewSender(ctx, 16)
	slot := &s.slots[0]
	atomic.StoreInt32(&slot.state, SlotAbandoned)
	atomic.StoreInt64(&slot.timestamp, time.Now().Add(-10*time.Second).UnixMilli())
	oldVer := atomic.LoadUint64(&slot.version)

	s.checkSlot(slot, time.Now().UnixMilli())

	if atomic.LoadInt32(&slot.state) != SlotFree {
		t.Fatal("expired abandoned slot should be recycled to Free")
	}
	if atomic.LoadUint64(&slot.version) <= oldVer {
		t.Fatal("version should have incremented")
	}
}

func TestCheckSlot_AbandonedSlot_NotExpired(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := NewSender(ctx, 16)
	slot := &s.slots[0]
	atomic.StoreInt32(&slot.state, SlotAbandoned)
	atomic.StoreInt64(&slot.timestamp, time.Now().UnixMilli())

	s.checkSlot(slot, time.Now().UnixMilli())

	if atomic.LoadInt32(&slot.state) != SlotAbandoned {
		t.Fatal("non-expired abandoned slot should stay abandoned")
	}
}

func TestCheckSlot_WaitingSlot_Expired(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := NewSender(ctx, 16)
	slot := &s.slots[0]
	atomic.StoreInt32(&slot.state, SlotWaiting)
	atomic.StoreInt64(&slot.timestamp, time.Now().Add(-3*time.Minute).UnixMilli())

	s.checkSlot(slot, time.Now().UnixMilli())

	if atomic.LoadInt32(&slot.state) != SlotFree {
		t.Fatal("expired waiting slot should be recycled")
	}
}

func TestCheckSlot_WaitingSlot_NotExpired(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := NewSender(ctx, 16)
	slot := &s.slots[0]
	atomic.StoreInt32(&slot.state, SlotWaiting)
	atomic.StoreInt64(&slot.timestamp, time.Now().UnixMilli())

	s.checkSlot(slot, time.Now().UnixMilli())

	if atomic.LoadInt32(&slot.state) != SlotWaiting {
		t.Fatal("non-expired waiting slot should stay waiting")
	}
}

func TestRecycleSlot_WithMessage(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := NewSender(ctx, 16)
	slot := &s.slots[0]
	if slot.ch == nil {
		slot.ch = make(chan *zmsg.Message, 1)
	}

	msg := zmsg.GetMessage()
	msg.MsgId = 42
	slot.ch <- msg.Retain()

	atomic.StoreInt32(&slot.state, SlotAbandoned)
	oldVer := atomic.LoadUint64(&slot.version)

	s.recycleSlot(slot)

	if atomic.LoadInt32(&slot.state) != SlotFree {
		t.Fatal("recycled slot should be Free")
	}
	if atomic.LoadUint64(&slot.version) != oldVer+1 {
		t.Fatal("version should increment by 1")
	}
	select {
	case <-slot.ch:
		t.Fatal("channel should be empty after recycle")
	default:
	}
}

func TestRecycleSlot_EmptyChannel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := NewSender(ctx, 16)
	slot := &s.slots[0]
	atomic.StoreInt32(&slot.state, SlotWaiting)

	s.recycleSlot(slot)

	if atomic.LoadInt32(&slot.state) != SlotFree {
		t.Fatal("should be Free")
	}
}

func TestSender_TimeoutLateReplyAndRecycle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := NewSender(ctx, 16)

	rpcId, err := s.AddSender()
	if err != nil {
		t.Fatalf("AddSender failed: %v", err)
	}

	// 1) 先超时返回（GetReply 会把 slot 从 Waiting -> Abandoned）
	resp, ok := s.GetReply(rpcId, 1*time.Millisecond)
	if resp != nil {
		resp.Release()
	}
	if ok {
		t.Fatal("expected timeout")
	}

	idx := rpcId & s.indexMask
	slot := &s.slots[idx]
	if st := atomic.LoadInt32(&slot.state); st != SlotAbandoned && st != SlotFree {
		t.Fatalf("unexpected slot state after timeout: %d", st)
	}

	// 2) 迟到回复：SetReply 不应把消息写入已超时的 slot
	late := zmsg.GetMessage()
	late.RpcId = rpcId
	late.MsgId = 123
	s.SetReply(late)
	late.Release()

	if slot.ch != nil {
		select {
		case m := <-slot.ch:
			if m != nil {
				m.Release()
			}
			t.Fatal("late reply should not be delivered into slot channel")
		default:
		}
	}

	// 3) 模拟 watchdog 回收：把 Abandoned 的 timestamp 设旧并调用 checkSlot
	atomic.StoreInt32(&slot.state, SlotAbandoned)
	atomic.StoreInt64(&slot.timestamp, time.Now().Add(-10*time.Second).UnixMilli())
	s.checkSlot(slot, time.Now().UnixMilli())

	if st := atomic.LoadInt32(&slot.state); st != SlotFree {
		t.Fatalf("expected slot recycled to Free, got %d", st)
	}
}

// ============================================================
// SetReply / GetReply edge cases
// ============================================================

func TestSetReply_VersionMismatch(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := NewSender(ctx, 16)

	rpcId, err := s.AddSender()
	if err != nil {
		t.Fatalf("AddSender failed: %v", err)
	}

	idx := rpcId & s.indexMask
	atomic.AddUint64(&s.slots[idx].version, 1)

	msg := zmsg.GetMessage()
	msg.RpcId = rpcId
	s.SetReply(msg)
	msg.Release()

	select {
	case <-s.slots[idx].ch:
		t.Fatal("should not have received message due to version mismatch")
	default:
	}
	atomic.StoreInt32(&s.slots[idx].state, SlotFree)
}

func TestSetReply_SlotNotWaiting(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := NewSender(ctx, 16)

	rpcId, err := s.AddSender()
	if err != nil {
		t.Fatalf("AddSender failed: %v", err)
	}
	idx := rpcId & s.indexMask

	atomic.StoreInt32(&s.slots[idx].state, SlotFree)

	msg := zmsg.GetMessage()
	msg.RpcId = rpcId
	s.SetReply(msg)
	msg.Release()
}

func TestGetReply_ContextDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	s := NewSender(ctx, 16)

	rpcId, err := s.AddSender()
	if err != nil {
		t.Fatalf("AddSender failed: %v", err)
	}
	cancel()

	_, ok := s.GetReply(rpcId, 5*time.Second)
	if ok {
		t.Fatal("should fail when context is done")
	}
}

func TestGetReply_StaleMessage(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := NewSender(ctx, 16)

	rpcId, err := s.AddSender()
	if err != nil {
		t.Fatalf("AddSender failed: %v", err)
	}
	idx := rpcId & s.indexMask

	stale := zmsg.GetMessage()
	stale.RpcId = rpcId + 1
	s.slots[idx].ch <- stale.Retain()

	go func() {
		time.Sleep(50 * time.Millisecond)
		correct := zmsg.GetMessage()
		correct.RpcId = rpcId
		s.slots[idx].ch <- correct.Retain()
	}()

	data, ok := s.GetReply(rpcId, 2*time.Second)
	if !ok {
		t.Fatal("should eventually get the correct message")
	}
	data.Release()
}

// TestWatchdog_ExitsOnContextCancel exercises the watchdog goroutine.
// When ctx is canceled, watchdog should exit (no panic).
func TestWatchdog_ExitsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	_ = NewSender(ctx, 16)
	cancel()
	time.Sleep(100 * time.Millisecond)
}

func TestSender_ConcurrentSuccessFlow(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := NewSender(ctx, 1024)
	warm := zmsg.GetMessage()
	warm.Release()

	const workers = 16
	const perWorker = 1000
	var wg sync.WaitGroup

	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				rpcId, err := s.AddSender()
				if err != nil {
					t.Errorf("AddSender failed: %v", err)
					return
				}

				reply := zmsg.GetMessage()
				reply.RpcId = rpcId
				reply.IsResponse = true
				s.SetReply(reply)
				reply.Release()

				got, ok := s.GetReply(rpcId, 200*time.Millisecond)
				if !ok {
					t.Errorf("GetReply timeout for rpcId=%d", rpcId)
					return
				}
				if got == nil || got.RpcId != rpcId {
					if got != nil {
						got.Release()
					}
					t.Errorf("unexpected reply rpcId, expected=%d got=%v", rpcId, got)
					return
				}
				got.Release()
			}
		}()
	}
	wg.Wait()
}

func TestSender_ConcurrentTimeoutAndLateReply(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := NewSender(ctx, 32768)
	warm := zmsg.GetMessage()
	warm.Release()

	const workers = 8
	const perWorker = 300
	var wg sync.WaitGroup

	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				var rpcId uint64
				var err error
				for retry := 0; retry < 200; retry++ {
					rpcId, err = s.AddSender()
					if err == nil {
						break
					}
					time.Sleep(2 * time.Millisecond)
				}
				if err != nil {
					t.Errorf("worker=%d AddSender failed after retries: %v", workerID, err)
					return
				}

				// 偶数：正常回包；奇数：先超时，后迟到回包
				if i%2 == 0 {
					reply := zmsg.GetMessage()
					reply.RpcId = rpcId
					reply.IsResponse = true
					s.SetReply(reply)
					reply.Release()

					got, ok := s.GetReply(rpcId, 200*time.Millisecond)
					if !ok || got == nil || got.RpcId != rpcId {
						if got != nil {
							got.Release()
						}
						t.Errorf("worker=%d success path mismatch rpcId=%d ok=%v", workerID, rpcId, ok)
						return
					}
					got.Release()
					continue
				}

				got, ok := s.GetReply(rpcId, 1*time.Millisecond)
				if ok {
					if got != nil {
						got.Release()
					}
					t.Errorf("worker=%d expected timeout rpcId=%d", workerID, rpcId)
					return
				}

				late := zmsg.GetMessage()
				late.RpcId = rpcId
				late.IsResponse = true
				s.SetReply(late)
				late.Release()
			}
		}(w)
	}
	wg.Wait()
}
