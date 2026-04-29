package zactor

import (
	"context"
	"testing"
	"time"

	"github.com/aiyang-zh/zhenyi/zmodel"
	"github.com/aiyang-zh/zhenyi/zmsg"
)

func TestStartAsyncThen_CallbackOnActorThread(t *testing.T) {
	a := newTestActor()
	defer a.Close(context.Background())

	done := make(chan int, 1)
	ok := a.StartAsyncThen(nil,
		func(_ *zmsg.Message) (interface{}, error) { return 7, nil },
		func(_ *Actor, v interface{}, err error) {
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			done <- v.(int)
		},
	)
	if !ok {
		t.Fatal("StartAsyncThen submit failed")
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	msgs := make([]zmodel.ActorCmd, 16)
	for time.Now().Before(deadline) {
		n := a.mailBoxQueue.DequeueBatch(msgs)
		for i := 0; i < n; i++ {
			a.SafeHandleMessage(context.Background(), msgs[i], 0)
		}
		select {
		case got := <-done:
			if got != 7 {
				t.Fatalf("expected 7, got %d", got)
			}
			return
		default:
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("callback not executed")
}
