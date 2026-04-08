package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/aiyang-zh/zhenyi-base/ziface"
	"github.com/aiyang-zh/zhenyi-base/znet"
	"github.com/aiyang-zh/zhenyi-base/ztcp"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8021", "gate addr")
	text := flag.String("text", "hello", "text to echo")
	flag.Parse()

	client, err := ztcp.NewClient(*addr, znet.WithAsyncMode())
	if err != nil {
		panic(err)
	}
	defer client.Close()

	var seq atomic.Uint32
	client.SetReadCall(func(w ziface.IWireMessage) {
		fmt.Printf("[recv] msgId=%d seq=%d data=%q\n", w.GetMsgId(), w.GetSeqId(), string(w.GetMessageData()))
	})
	client.Read()

	payload, _ := json.Marshal(map[string]any{"text": *text})
	m := znet.GetNetMessage()
	m.MsgId = 1
	m.SeqId = seq.Add(1)
	m.SetDataCopy(payload)
	client.SendMsg(m)
	m.Release()

	time.Sleep(500 * time.Millisecond)
}
