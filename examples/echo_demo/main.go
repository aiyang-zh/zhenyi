package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime/pprof"
	"time"

	baseziface "github.com/aiyang-zh/zhenyi-base/ziface"
	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi-base/znet"
	"github.com/aiyang-zh/zhenyi-base/zserialize"
	"github.com/aiyang-zh/zhenyi/zcodec"
	"github.com/aiyang-zh/zhenyi/zgate"
	"github.com/aiyang-zh/zhenyi/ziface"
	"github.com/aiyang-zh/zhenyi/zmodel"
	"github.com/aiyang-zh/zhenyi/zmsg"
	"github.com/aiyang-zh/zhenyi/zstartup"
	"github.com/aiyang-zh/zhenyi/zstream"
)

const (
	ActorTypeGate uint32 = 1
	ActorTypeEcho uint32 = 2
	MsgEchoReq    int32  = 1
	MsgCloseReq   int32  = 2
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8021", "gate listen addr")
	connKind := flag.String("conn", "tcp", "listen protocol: tcp | ws")
	cpuProfile := flag.String("cpuprofile", "", "write cpu profile to file (optional)")
	cpuProfileSec := flag.Int("cpuprofileSec", 10, "cpu profile duration in seconds")
	flag.Parse()

	connProto := znet.TCP
	if *connKind == "ws" || *connKind == "websocket" {
		connProto = znet.WebSocket
	} else if *connKind != "tcp" {
		panic(fmt.Sprintf("unknown -conn %q (use tcp or ws)", *connKind))
	}

	logCfg := zlog.NewDefaultLoggerConfig()
	logCfg.WithOptions(zlog.WithConsole(true))
	zlog.NewDefaultLoggerWithConfig(logCfg)

	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			panic(err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			_ = f.Close()
			panic(err)
		}
		go func() {
			time.Sleep(time.Duration(*cpuProfileSec) * time.Second)
			pprof.StopCPUProfile()
			_ = f.Close()
			fmt.Printf("cpu profile written: %s\n", *cpuProfile)
		}()
	}

	app := zstartup.NewApp(context.Background(), zstartup.AppConfig{
		IsSingle: true, ConnType: connProto,
		Actors: []zmodel.ActorConfig{
			{Id: 1, ActorType: ActorTypeGate, Name: "gate", Index: 1, Addr: *addr, Process: 1},
			{Id: 2, ActorType: ActorTypeEcho, Name: "echo", Index: 1, Process: 1},
		},
	})

	var echo *zstream.Server
	if err := app.RegisterActorFactory(ActorTypeGate, func(a *zstartup.App, c zmodel.ActorConfig) ziface.IServerActor {
		g := zgate.NewServer(c, a.ConnType)
		g.WithNetServerHook(func(srv baseziface.IServer) { srv.SetHeartbeatTimeout(0) })
		g.OnChannelClose(func(ch baseziface.IChannel) {
			if echo == nil || ch == nil {
				return
			}
			m := zmsg.GetMessage()
			m.MsgId, m.FromClient, m.SessionId, m.SrcActor = MsgCloseReq, true, ch.GetChannelId(), g.GetActorId()
			echo.Push(zmodel.ActorCmd{Type: zmodel.CmdTypeClient, Msg: m})
		})
		return g
	}); err != nil {
		panic(err)
	}

	if err := app.RegisterActorFactory(ActorTypeEcho, func(a *zstartup.App, c zmodel.ActorConfig) ziface.IServerActor {
		s := zstream.NewServer(c)
		echo = s
		s.GetHandleMgr().RegisterHandle(MsgCloseReq, func(ctx context.Context, msg *zmsg.Message) {
			_ = ctx
			_ = msg // demo: ignore close notification
		})
		s.GetHandleMgr().RegisterHandle(MsgEchoReq, func(ctx context.Context, msg *zmsg.Message) {
			_ = ctx
			var req struct {
				Text string `json:"text"`
			}
			_ = zserialize.UnmarshalJson(msg.Data, &req)
			data, err := zcodec.NewJSONMessage(MsgEchoReq, map[string]any{"type": "echo_ack", "sessionId": msg.SessionId, "text": req.Text})
			if err != nil {
				panic(err)
			}
			s.SendToClient(msg, data)
		})
		return s
	}); err != nil {
		panic(err)
	}

	if err := app.Run(); err != nil {
		panic(err)
	}
}
