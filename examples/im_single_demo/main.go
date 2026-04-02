package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/aiyang-zh/zhenyi-base/zencrypt"
	gmtls "github.com/aiyang-zh/zhenyi-base/zgmtls"
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
	ActorTypeIM   uint32 = 2
)

const (
	MsgLoginReq int32 = 1
	MsgJoinReq  int32 = 2
	MsgLeaveReq int32 = 3
	MsgSendReq  int32 = 4
)

type chatState struct {
	sessionNick  map[uint64]string
	sessionRoom  map[uint64]string
	roomSessions map[string]map[uint64]struct{}
}

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

// batchToRoomExcept 向房间内除 excludeSession 外的连接批量下行（复用 zactor.BatchSendToClients：payload 只序列化一次）。
func (s *ImServer) batchToRoomExcept(origin *zmsg.Message, room string, excludeSession uint64, clientMsg ziface.IMessage) {
	gate := origin.SrcActor
	if gate == 0 {
		s.GetLogger().Error("batchToRoomExcept: origin.SrcActor is zero")
		return
	}
	var ids []int64
	for sid := range s.state.roomSessions[room] {
		if sid == excludeSession {
			continue
		}
		ids = append(ids, int64(sid))
	}
	if len(ids) == 0 {
		return
	}
	s.BatchSendToClients(origin, map[uint64][]int64{gate: ids}, clientMsg)
}

// batchToRoomAll 向房间内全部连接批量下行（含发送者）。
func (s *ImServer) batchToRoomAll(origin *zmsg.Message, room string, clientMsg ziface.IMessage) {
	gate := origin.SrcActor
	if gate == 0 {
		s.GetLogger().Error("batchToRoomAll: origin.SrcActor is zero")
		return
	}
	var ids []int64
	for sid := range s.state.roomSessions[room] {
		ids = append(ids, int64(sid))
	}
	if len(ids) == 0 {
		return
	}
	s.BatchSendToClients(origin, map[uint64][]int64{gate: ids}, clientMsg)
}

type GateServer struct {
	*zgate.Server
}

func (s *GateServer) applyGateGMTLS(gmSignCert, gmSignKey, gmEncCert, gmEncKey, gmCert, gmKey, gmCipherSuite string) {
	dual := gmSignCert != "" && gmSignKey != "" && gmEncCert != "" && gmEncKey != ""
	single := gmCert != "" && gmKey != ""
	switch {
	case dual:
		if err := s.SetGMTLS(gmSignCert, gmSignKey, gmEncCert, gmEncKey); err != nil {
			panic(err)
		}
	case single:
		if err := s.SetGMTLSSingle(gmCert, gmKey); err != nil {
			panic(err)
		}
	default:
		partial := (gmCert != "" || gmKey != "") || (gmSignCert != "" || gmSignKey != "" || gmEncCert != "" || gmEncKey != "")
		if partial {
			panic("国密证书参数不完整：单证书需同时指定 -gmCert 与 -gmKey；双证书需同时指定 -gmSignCert -gmSignKey -gmEncCert -gmEncKey")
		}
	}
	s.applyGMTLSCipherSuiteFlag(gmCipherSuite, dual || single)
}

// applyGMTLSCipherSuiteFlag 控制 GM-TLS 套件：默认与 zgmtls 一致（优先 ECDHE）；可显式仅 ECDHE / 仅 ECC / 两者顺序固定。
func (s *GateServer) applyGMTLSCipherSuiteFlag(mode string, gmEnabled bool) {
	if !gmEnabled {
		return
	}
	m := strings.ToLower(strings.TrimSpace(mode))
	switch m {
	case "", "default":
		// zgmtls.Config 未设 CipherSuites 时 getCipherSuites 为 ECDHE 优先
		return
	case "ecdhe":
		s.SetGMTLSCipherSuites([]uint16{gmtls.GMTLS_ECDHE_SM2_WITH_SM4_SM3})
	case "ecc":
		s.SetGMTLSCipherSuites([]uint16{gmtls.GMTLS_SM2_WITH_SM4_SM3})
	case "both":
		s.SetGMTLSCipherSuites([]uint16{gmtls.GMTLS_ECDHE_SM2_WITH_SM4_SM3, gmtls.GMTLS_SM2_WITH_SM4_SM3})
	default:
		panic(fmt.Sprintf("未知 -gmCipherSuite %q（可用 default|ecdhe|ecc|both）", mode))
	}
}

func main() {
	var (
		gateAddr         = flag.String("addr", "127.0.0.1:8001", "gate listen addr for this demo")
		connKind         = flag.String("conn", "tcp", "listen protocol: tcp | ws (浏览器请用 ws)")
		reactor          = flag.Bool("reactor", false, "enable TCP reactor mode (mac/linux only; requires -conn=tcp and no TLS/GM-TLS)")
		sharedSendWorker = flag.Bool("sharedSendWorker", false, "enable shared send worker mode on gate (ztcp/zws/zkcp; default off)")

		gmCert        = flag.String("gmCert", "", "国密 SM2 单证书 PEM（与 -gmKey 成对；启用 Gate GM-TLS）")
		gmKey         = flag.String("gmKey", "", "国密 SM2 单证书私钥 PEM")
		gmSignCert    = flag.String("gmSignCert", "", "国密双证书：签名证书 PEM")
		gmSignKey     = flag.String("gmSignKey", "", "国密双证书：签名私钥 PEM")
		gmEncCert     = flag.String("gmEncCert", "", "国密双证书：加密证书 PEM")
		gmEncKey      = flag.String("gmEncKey", "", "国密双证书：加密私钥 PEM")
		gmCipherSuite = flag.String("gmCipherSuite", "default", "GM-TLS 套件策略：default（库默认，优先 ECDHE）| ecdhe（仅 ECDHE_SM2）| ecc（仅 SM2 加密证书密钥协商）| both（显式 ECDHE 再 ECC）")
		payloadEncKey = flag.String("payloadEncKey", "", "线协议 payload 国密 SM4-GCM（密钥经 SM3 派生）；非空则须客户端同密钥，与 GM-TLS 独立")
	)
	flag.Parse()

	connProto := znet.TCP
	switch strings.ToLower(strings.TrimSpace(*connKind)) {
	case "ws", "websocket":
		connProto = znet.WebSocket
	case "tcp":
	default:
		panic(fmt.Sprintf("unknown -conn %q (use tcp or ws)", *connKind))
	}

	logConfig := zlog.NewDefaultLoggerConfig()
	logConfig.WithOptions(zlog.WithConsole(true))
	zlog.NewDefaultLoggerWithConfig(logConfig)

	app := zstartup.NewApp(context.Background(), zstartup.AppConfig{
		Process:  0,
		IsSingle: true,
		ConnType: connProto,
		Actors: []zmodel.ActorConfig{
			zmodel.ActorConfig{
				Id:        1,
				ActorType: ActorTypeGate,
				Name:      "gate",
				Index:     1,
				Addr:      *gateAddr,
				Process:   1,
			},
			zmodel.ActorConfig{
				Id:        2,
				ActorType: ActorTypeIM,
				Name:      "im",
				Index:     1,
				Process:   1,
			},
		},
	})

	err := app.RegisterActorFactory(ActorTypeGate, func(a *zstartup.App, c zmodel.ActorConfig) ziface.IServerActor {
		s := &GateServer{
			Server: zgate.NewServer(c, a.ConnType),
		}
		s.SetReactorMode(*reactor)
		s.SetSharedSendWorkerMode(*sharedSendWorker)
		s.applyGateGMTLS(*gmSignCert, *gmSignKey, *gmEncCert, *gmEncKey, *gmCert, *gmKey, *gmCipherSuite)
		if k := strings.TrimSpace(*payloadEncKey); k != "" {
			s.SetEncrypt(zencrypt.NewSM4GcmEncrypt(k))
		}
		s.GetHandleMgr().RegisterHandle(MsgLoginReq, func(ctx context.Context, msg *zmsg.Message) {
			var req struct {
				UserID int64 `json:"userId"`
			}

			_ = zserialize.UnmarshalJson(msg.Data, &req)

			payload := map[string]any{
				"ok":        true,
				"type":      "login_ack",
				"sessionId": msg.SessionId,
				"userId":    req.UserID,
			}
			var data ziface.IMessage
			var err error
			data, err = zcodec.NewJSONMessage(MsgLoginReq, payload)
			if err != nil {
				panic(err)
			}
			s.SendToClient(msg, data)
			s.GetLogger().Info("login success", zap.Int64("userId", req.UserID))
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

		s.GetHandleMgr().RegisterHandle(MsgJoinReq, func(ctx context.Context, msg *zmsg.Message) {
			var req struct {
				Room     string `json:"room"`
				Nickname string `json:"nickname"`
			}
			_ = zserialize.UnmarshalJson(msg.Data, &req)
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
			data, err = zcodec.NewJSONMessage(MsgJoinReq, payload)
			if err != nil {
				panic(err)
			}
			s.SendToClient(msg, data)

			notifyJoin, err := zcodec.NewJSONMessage(MsgJoinReq, map[string]any{
				"type":      "room_notify",
				"event":     "join",
				"room":      req.Room,
				"sessionId": msg.SessionId,
				"nickname":  s.state.getNick(msg.SessionId),
			})
			if err != nil {
				panic(err)
			}
			s.batchToRoomExcept(msg, req.Room, msg.SessionId, notifyJoin)
		})
		s.GetHandleMgr().RegisterHandle(MsgLeaveReq, func(ctx context.Context, msg *zmsg.Message) {
			var req struct {
				Room string `json:"room"`
			}
			_ = zserialize.UnmarshalJson(msg.Data, &req)
			room := req.Room
			if room == "" {
				room = s.state.sessionRoom[msg.SessionId]
			}
			if room != "" {
				notifyLeave, err := zcodec.NewJSONMessage(MsgLeaveReq, map[string]any{
					"type":      "room_notify",
					"event":     "leave",
					"room":      room,
					"sessionId": msg.SessionId,
					"nickname":  s.state.getNick(msg.SessionId),
				})
				if err != nil {
					panic(err)
				}
				s.batchToRoomExcept(msg, room, msg.SessionId, notifyLeave)
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
			data, err = zcodec.NewJSONMessage(MsgLeaveReq, payload)
			if err != nil {
				panic(err)
			}
			s.SendToClient(msg, data)
		})
		s.GetHandleMgr().RegisterHandle(MsgSendReq, func(ctx context.Context, msg *zmsg.Message) {
			var req struct {
				Room string `json:"room"`
				Text string `json:"text"`
			}
			_ = zserialize.UnmarshalJson(msg.Data, &req)
			if req.Room == "" {
				req.Room = "lobby"
			}
			// 确保发送者在房间成员表里，否则广播列表会缺自己或为空。
			s.state.joinRoom(msg.SessionId, req.Room)

			payload := map[string]any{
				"type":          "chat_broadcast",
				"room":          req.Room,
				"fromSessionId": msg.SessionId,
				"nickname":      s.state.getNick(msg.SessionId),
				"text":          req.Text,
			}
			payloadBytes, err := json.Marshal(payload)
			if err != nil {
				panic(err)
			}
			// 与 gmsm sm3.New().Write(b).Sum(nil) 等价，用 zencrypt.SM3Bytes 即可
			payload["sign"] = hex.EncodeToString(zencrypt.SM3Bytes(payloadBytes))

			data, err := zcodec.NewJSONMessage(MsgSendReq, payload)
			if err != nil {
				panic(err)
			}
			s.batchToRoomAll(msg, req.Room, data)
		})
		return s
	})
	if err != nil {
		panic(err)
	}
	err = app.Run()
	if err != nil {
		panic(err)
	}
}
