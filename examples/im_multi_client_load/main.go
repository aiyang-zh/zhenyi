package main

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	baseziface "github.com/aiyang-zh/zhenyi-base/ziface"
	"github.com/aiyang-zh/zhenyi-base/znet"
	"github.com/aiyang-zh/zhenyi-base/zserialize"
	"github.com/aiyang-zh/zhenyi-base/ztcp"
)

type workerStats struct {
	sent atomic.Uint64
	recv atomic.Uint64
}

func main() {
	var (
		addr       = flag.String("addr", "127.0.0.1:8001", "gate addr")
		room       = flag.String("room", "lobby", "room")
		clients    = flag.Int("clients", 20, "concurrent client count")
		intervalMs = flag.Int("intervalMs", 1000, "send interval per client (ms)")
		durationS  = flag.Int("durationS", 30, "test duration (seconds)")
		prefix     = flag.String("prefix", "bot", "user nickname prefix")
		benchMode  = flag.String("benchMode", "business", "bench mode: business|framework")

		msgLogin = flag.Int("msgLogin", 1, "login request msg id")
		msgJoin  = flag.Int("msgJoin", 2, "join room request msg id")
		msgSend  = flag.Int("msgSend", 4, "send room message request msg id")
		codec    = flag.String("codec", "json", "payload codec: json|msgpack")
	)
	flag.Parse()
	selectedCodec := strings.ToLower(*codec)
	if selectedCodec != "json" && selectedCodec != "msgpack" {
		panic("codec must be json|msgpack")
	}
	selectedBenchMode := strings.ToLower(*benchMode)
	if selectedBenchMode != "business" && selectedBenchMode != "framework" {
		panic("benchMode must be business|framework")
	}

	if *clients <= 0 {
		panic("clients must be > 0")
	}
	if *intervalMs <= 0 {
		panic("intervalMs must be > 0")
	}
	if *durationS <= 0 {
		panic("durationS must be > 0")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*durationS)*time.Second)
	defer cancel()

	fmt.Printf("start load test addr=%s room=%s clients=%d intervalMs=%d durationS=%d\n",
		*addr, *room, *clients, *intervalMs, *durationS)

	var all workerStats
	var wg sync.WaitGroup
	wg.Add(*clients)

	for i := 0; i < *clients; i++ {
		time.Sleep(1 * time.Millisecond)
		go func(idx int) {
			defer wg.Done()
			runOne(ctx, idx, *addr, *room, *prefix, int32(*msgLogin), int32(*msgJoin), int32(*msgSend), selectedCodec, selectedBenchMode, time.Duration(*intervalMs)*time.Millisecond, &all)
		}(i)
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	start := time.Now()
	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			elapsed := time.Since(start).Seconds()
			sent := all.sent.Load()
			recv := all.recv.Load()
			fmt.Printf("done elapsed=%.1fs sent=%d recv=%d avg_send_qps=%.2f\n", elapsed, sent, recv, float64(sent)/elapsed)
			return
		case <-ticker.C:
			elapsed := time.Since(start).Seconds()
			sent := all.sent.Load()
			recv := all.recv.Load()
			fmt.Printf("progress elapsed=%.1fs sent=%d recv=%d avg_send_qps=%.2f\n", elapsed, sent, recv, float64(sent)/elapsed)
		}
	}
}

func runOne(
	ctx context.Context,
	idx int,
	addr, room, prefix string,
	msgLogin, msgJoin, msgSend int32,
	codec string,
	benchMode string,
	interval time.Duration,
	all *workerStats,
) {
	client, err := ztcp.NewClient(addr, znet.WithAsyncMode())
	if err != nil {
		fmt.Printf("[worker-%d] connect failed: %v\n", idx, err)
		return
	}
	const recvFlushBatch = 64
	recvPending := 0
	defer func() {
		_ = client.Close()
		if recvPending > 0 {
			all.recv.Add(uint64(recvPending))
		}
	}()

	var seq atomic.Uint32
	send := func(msgID int32, payload any) {
		var b []byte
		// Framework mode: join/leave/send 请求 payload 不参与业务，直接跳过编解码，降低分配/GC。
		if !(benchMode == "framework" && (msgID == msgJoin || msgID == msgSend)) {
			var err error
			if codec == "msgpack" {
				b, err = zserialize.MarshalMsgPack(payload)
			} else {
				b, err = zserialize.MarshalJson(payload)
			}
			if err != nil {
				fmt.Printf("[worker-%d] marshal failed: %v\n", idx, err)
				return
			}
		}
		m := znet.GetNetMessage()
		defer m.Release()
		m.MsgId = msgID
		m.SeqId = seq.Add(1)
		m.SetDataCopy(b)
		client.SendMsg(m)
		all.sent.Add(1)
	}

	client.SetReadCall(func(w baseziface.IWireMessage) {
		_ = w
		// Reduce global atomic contention: flush recv count in batches.
		recvPending++
		if recvPending >= recvFlushBatch {
			all.recv.Add(uint64(recvPending))
			recvPending = 0
		}
	})
	client.Read()

	userID := int64(100000 + idx)
	nick := fmt.Sprintf("%s_%d", prefix, idx)

	send(msgLogin, map[string]any{
		"userId":   userID,
		"nickname": nick,
	})
	if benchMode == "framework" {
		// Framework mode: server不会解码 join/leave/send 请求，直接发送空 payload，避免每次发送的 marshal 分配。
		send(msgJoin, nil)
	} else {
		send(msgJoin, map[string]any{
			"room":     room,
			"nickname": nick,
		})
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if benchMode == "framework" {
				// Framework mode: server不会解码 send 请求 payload。
				_ = now
				send(msgSend, nil)
			} else {
				send(msgSend, map[string]any{
					"room": room,
					"text": fmt.Sprintf("hello from %s @%d", nick, now.UnixMilli()),
				})
			}
		}
	}
}
