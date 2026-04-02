package main

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"

	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi-base/zserialize"
	"github.com/aiyang-zh/zhenyi/zcodec"
	"github.com/aiyang-zh/zhenyi/zdiscovery"
	"github.com/aiyang-zh/zhenyi/zgate"
	"github.com/aiyang-zh/zhenyi/ziface"
	"github.com/aiyang-zh/zhenyi/zmodel"
	"github.com/aiyang-zh/zhenyi/zmsg"
	"github.com/aiyang-zh/zhenyi/znats"
	"github.com/aiyang-zh/zhenyi/zstartup"
	"github.com/aiyang-zh/zhenyi/zstream"
)

const (
	ActorTypeGate uint32 = 1
	ActorTypeIM   uint32 = 2
)

const (
	MsgLoginReq int32 = 1
	MsgJoinReq  int32 = 2
	MsgLeaveReq int32 = 3
	MsgSendReq  int32 = 4
)

const (
	ProcessGate uint = 1
	ProcessIM   uint = 2
)

const (
	GateActorID uint64 = 101
	IMActorID   uint64 = 201
)

const (
	MsgWhoReq  int32 = 100
	MsgWhoResp int32 = 101
)

type chatState struct {
	sessionNick  map[uint64]string
	sessionRoom  map[uint64]string
	roomSessions map[string]map[uint64]struct{}
}

// fixedBytesMessage is a zero-allocation IMessage adapter for constant payload bytes.
// It is safe to reuse across goroutines because it never mutates its internal fields.
type fixedBytesMessage struct {
	msgID int32
	data  []byte
}

func (m *fixedBytesMessage) UnmarshalVT([]byte) error { return nil }
func (m *fixedBytesMessage) MarshalVT() ([]byte, error) {
	// Note: this returns the internal slice directly; only MarshalToVT is used by the hot path.
	return m.data, nil
}
func (m *fixedBytesMessage) MarshalToVT(dst []byte) (int, error) { return copy(dst, m.data), nil }
func (m *fixedBytesMessage) SizeVT() int                         { return len(m.data) }
func (m *fixedBytesMessage) GetMsgId() int32                     { return m.msgID }

func newChatState() *chatState {
	return &chatState{
		sessionNick:  make(map[uint64]string),
		sessionRoom:  make(map[uint64]string),
		roomSessions: make(map[string]map[uint64]struct{}),
	}
}

func (s *chatState) setNick(sessionID uint64, nick string) {
	s.sessionNick[sessionID] = nick
}

func (s *chatState) joinRoom(sessionID uint64, room string) {
	prev := s.sessionRoom[sessionID]
	if prev != "" && prev != room {
		if set := s.roomSessions[prev]; set != nil {
			delete(set, sessionID)
		}
	}
	s.sessionRoom[sessionID] = room
	set := s.roomSessions[room]
	if set == nil {
		set = make(map[uint64]struct{})
		s.roomSessions[room] = set
	}
	set[sessionID] = struct{}{}
}

func (s *chatState) leaveRoom(sessionID uint64, room string) {
	if room == "" {
		room = s.sessionRoom[sessionID]
	}
	if room != "" {
		if set := s.roomSessions[room]; set != nil {
			delete(set, sessionID)
		}
	}
	delete(s.sessionRoom, sessionID)
}

func (s *chatState) getNick(sessionID uint64) string {
	nick := s.sessionNick[sessionID]
	if nick == "" {
		return fmt.Sprintf("guest_%d", sessionID)
	}
	return nick
}

type ImServer struct {
	*zstream.Server
	state *chatState
}

type whoRPCResult struct {
	ok       bool
	imNode   string
	rpcError string
}

func main() {
	process := flag.Uint("process", uint(ProcessGate), "process id: 1=gate, 2=im")
	addr := flag.String("addr", "127.0.0.1:8001", "gate listen addr (used when process=1)")
	natsURL := flag.String("nats", znats.DefaultURL, "nats url")
	etcdEP := flag.String("etcd", "127.0.0.1:2379", "etcd endpoint")
	codec := flag.String("codec", "json", "payload codec: json|msgpack")
	benchMode := flag.String("benchMode", "business", "bench mode: business|framework")
	reactor := flag.Bool("reactor", false, "enable TCP reactor mode for gate (mac/linux only; requires no TLS/GM-TLS)")
	sharedSendWorker := flag.Bool("sharedSendWorker", false, "enable shared send worker mode on gate (ztcp/zws/zkcp; default off)")
	flag.Parse()
	selectedCodec := strings.ToLower(*codec)
	if selectedCodec != "json" && selectedCodec != "msgpack" {
		panic(fmt.Sprintf("unsupported codec: %s (expect json|msgpack)", *codec))
	}
	selectedBenchMode := strings.ToLower(*benchMode)
	if selectedBenchMode != "business" && selectedBenchMode != "framework" {
		panic(fmt.Sprintf("unsupported benchMode: %s (expect business|framework)", *benchMode))
	}

	// Framework mode: pre-encode fixed reply payload once, avoid per-message map+marshal.
	type okResp struct {
		Ok bool `json:"ok" msgpack:"ok"`
	}
	var okReplyBytes []byte
	var err error
	if selectedCodec == "msgpack" {
		okReplyBytes, err = zserialize.MarshalMsgPack(okResp{Ok: true})
	} else {
		okReplyBytes, err = zserialize.MarshalJson(okResp{Ok: true})
	}
	if err != nil {
		panic(err)
	}
	replyJoinOK := &fixedBytesMessage{msgID: MsgJoinReq, data: okReplyBytes}
	replyLeaveOK := &fixedBytesMessage{msgID: MsgLeaveReq, data: okReplyBytes}
	replySendOK := &fixedBytesMessage{msgID: MsgSendReq, data: okReplyBytes}

	logConfig := zlog.NewDefaultLoggerConfig()
	logConfig.WithOptions(zlog.WithConsole(true))
	zlog.NewDefaultLoggerWithConfig(logConfig)

	ctx := context.Background()
	znats.NewDefaultNats(*natsURL, 1)
	if znats.DefaultNatsClient == nil {
		panic("nats pool init failed")
	}
	if err := znats.DefaultNatsClient.Connect(ctx); err != nil {
		panic(err)
	}

	etcdCli, err := clientv3.New(clientv3.Config{
		Endpoints: []string{*etcdEP},
	})
	if err != nil {
		panic(err)
	}
	defer etcdCli.Close()

	discoverer, err := zdiscovery.NewEtcdDiscovery(ctx, etcdCli)
	if err != nil {
		panic(err)
	}
	defer discoverer.CloseAll()

	app := zstartup.NewApp(ctx, zstartup.AppConfig{
		Process:  *process,
		IsSingle: false,
		ConnType: 1,
		Actors:   buildActors(*process, *addr),
	})
	app.Group.SetDiscoverer(discoverer)

	err = app.RegisterActorFactory(ActorTypeGate, func(a *zstartup.App, c zmodel.ActorConfig) ziface.IServerActor {
		s := zgate.NewServer(c, a.ConnType)
		s.SetReactorMode(*reactor)
		s.SetSharedSendWorkerMode(*sharedSendWorker)
		s.GetHandleMgr().RegisterHandle(MsgLoginReq, func(ctx context.Context, msg *zmsg.Message) {
			var req struct {
				UserID int64 `json:"userId"`
			}
			if selectedCodec == "msgpack" {
				_ = zserialize.UnmarshalMsgPack(msg.Data, &req)
			} else {
				_ = zserialize.UnmarshalJson(msg.Data, &req)
			}
			// Demonstrate non-blocking Actor -> Actor RPC.
			// 演示 Actor -> Actor RPC（非阻塞版）：
			// Wrap synchronous CallActor in AsyncRunWithMsg to avoid blocking Gate main message thread.
			// 用 AsyncRunWithMsg 包裹同步 CallActor，避免阻塞 Gate 的主消息线程。
			s.AsyncRunWithMsg(msg,
				func(in *zmsg.Message) interface{} {
					whoReply := &zcodec.JSONMessage{}
					whoReq, err := zcodec.NewJSONMessage(MsgWhoReq, map[string]any{
						"from": "gate_login",
					})
					if err != nil {
						panic(err)
					}
					rpcRes := s.CallActor(IMActorID, whoReq, whoReply, 800*time.Millisecond)
					if rpcRes.Code != ziface.ErrCode_Success {
						return whoRPCResult{ok: false, rpcError: rpcRes.Msg}
					}
					var who struct {
						ActorName string `json:"actorName"`
					}
					if err := whoReply.Decode(&who); err != nil || who.ActorName == "" {
						return whoRPCResult{ok: false, rpcError: "decode who reply failed"}
					}
					return whoRPCResult{ok: true, imNode: who.ActorName}
				},
				func(result interface{}) {
					rpc := whoRPCResult{ok: false, rpcError: "unknown"}
					if v, ok := result.(whoRPCResult); ok {
						rpc = v
					}
					imNode := "rpc_failed"
					if rpc.ok {
						imNode = rpc.imNode
					}
					payload := map[string]any{
						"ok":        true,
						"type":      "login_ack",
						"sessionId": msg.SessionId,
						"userId":    req.UserID,
						"imNode":    imNode,
						"rpcError":  rpc.rpcError,
					}
					var data ziface.IMessage
					var err error
					if selectedCodec == "msgpack" {
						data, err = zcodec.NewMsgpackMessage(MsgLoginReq, payload)
					} else {
						data, err = zcodec.NewJSONMessage(MsgLoginReq, payload)
					}
					if err != nil {
						panic(err)
					}
					s.SendToClient(msg, data)
					s.GetLogger().Info("login success", zap.Int64("userId", req.UserID), zap.String("imNode", imNode))
				},
			)
		})
		return s
	})
	if err != nil {
		panic(err)
	}

	err = app.RegisterActorFactory(ActorTypeIM, func(a *zstartup.App, c zmodel.ActorConfig) ziface.IServerActor {
		s := &ImServer{
			Server: zstream.NewServer(c),
			state:  newChatState(),
		}
		s.GetDispatcher().Register(MsgWhoReq, func(ctx context.Context, msg *zmsg.Message) ziface.IMessage {
			payload := map[string]any{
				"ok":        true,
				"actorId":   s.GetActorId(),
				"actorName": s.GetNameTopic(),
				"process":   s.GetActorConfig().Process,
			}
			var data ziface.IMessage
			var err error
			if selectedCodec == "msgpack" {
				data, err = zcodec.NewMsgpackMessage(MsgWhoResp, payload)
			} else {
				data, err = zcodec.NewJSONMessage(MsgWhoResp, payload)
			}
			if err != nil {
				panic(err)
			}
			return data
		})

		s.GetHandleMgr().RegisterHandle(MsgJoinReq, func(ctx context.Context, msg *zmsg.Message) {
			if selectedBenchMode == "framework" {
				s.SendToClient(msg, replyJoinOK)
				return
			}
			var req struct {
				Room     string `json:"room"`
				Nickname string `json:"nickname"`
			}
			if selectedCodec == "msgpack" {
				_ = zserialize.UnmarshalMsgPack(msg.Data, &req)
			} else {
				_ = zserialize.UnmarshalJson(msg.Data, &req)
			}
			if req.Room == "" {
				req.Room = "lobby"
			}
			if req.Nickname != "" {
				s.state.setNick(msg.SessionId, req.Nickname)
			}
			s.state.joinRoom(msg.SessionId, req.Room)

			payload := map[string]any{
				"ok":        true,
				"type":      "join_ack",
				"sessionId": msg.SessionId,
				"room":      req.Room,
				"nickname":  s.state.getNick(msg.SessionId),
			}
			var data ziface.IMessage
			var err error
			if selectedCodec == "msgpack" {
				data, err = zcodec.NewMsgpackMessage(MsgJoinReq, payload)
			} else {
				data, err = zcodec.NewJSONMessage(MsgJoinReq, payload)
			}
			if err != nil {
				panic(err)
			}
			s.SendToClient(msg, data)
		})
		s.GetHandleMgr().RegisterHandle(MsgLeaveReq, func(ctx context.Context, msg *zmsg.Message) {
			if selectedBenchMode == "framework" {
				s.SendToClient(msg, replyLeaveOK)
				return
			}
			var req struct {
				Room string `json:"room"`
			}
			if selectedCodec == "msgpack" {
				_ = zserialize.UnmarshalMsgPack(msg.Data, &req)
			} else {
				_ = zserialize.UnmarshalJson(msg.Data, &req)
			}
			s.state.leaveRoom(msg.SessionId, req.Room)
			payload := map[string]any{
				"ok":        true,
				"type":      "leave_ack",
				"sessionId": msg.SessionId,
				"room":      req.Room,
			}
			var data ziface.IMessage
			var err error
			if selectedCodec == "msgpack" {
				data, err = zcodec.NewMsgpackMessage(MsgLeaveReq, payload)
			} else {
				data, err = zcodec.NewJSONMessage(MsgLeaveReq, payload)
			}
			if err != nil {
				panic(err)
			}
			s.SendToClient(msg, data)
		})
		s.GetHandleMgr().RegisterHandle(MsgSendReq, func(ctx context.Context, msg *zmsg.Message) {
			if selectedBenchMode == "framework" {
				s.SendToClient(msg, replySendOK)
				return
			}
			var req struct {
				Room string `json:"room"`
				Text string `json:"text"`
			}
			if selectedCodec == "msgpack" {
				_ = zserialize.UnmarshalMsgPack(msg.Data, &req)
			} else {
				_ = zserialize.UnmarshalJson(msg.Data, &req)
			}
			if req.Room == "" {
				req.Room = "lobby"
			}
			payload := map[string]any{
				"ok":        true,
				"type":      "chat_echo",
				"sessionId": msg.SessionId,
				"room":      req.Room,
				"nickname":  s.state.getNick(msg.SessionId),
				"text":      req.Text,
			}
			var data ziface.IMessage
			var err error
			if selectedCodec == "msgpack" {
				data, err = zcodec.NewMsgpackMessage(MsgSendReq, payload)
			} else {
				data, err = zcodec.NewJSONMessage(MsgSendReq, payload)
			}
			if err != nil {
				panic(err)
			}
			s.SendToClient(msg, data)
		})
		return s
	})
	if err != nil {
		panic(err)
	}

	if err := app.Run(); err != nil {
		panic(err)
	}
}

func buildActors(process uint, gateAddr string) []zmodel.ActorConfig {
	switch process {
	case ProcessGate:
		return []zmodel.ActorConfig{
			{
				Id:        GateActorID,
				ActorType: ActorTypeGate,
				Name:      "gate",
				Index:     1,
				Addr:      gateAddr,
				Process:   uint32(ProcessGate),
			},
		}
	case ProcessIM:
		return []zmodel.ActorConfig{
			{
				Id:        IMActorID,
				ActorType: ActorTypeIM,
				Name:      "im",
				Index:     1,
				Process:   uint32(ProcessIM),
			},
		}
	default:
		panic(fmt.Sprintf("unsupported process id: %d (expect 1 or 2)", process))
	}
}
