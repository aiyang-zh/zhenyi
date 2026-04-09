package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	baseziface "github.com/aiyang-zh/zhenyi-base/ziface"
	"go.uber.org/zap"

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
	ActorTypeGate       uint32 = 1
	ActorTypeWhiteboard uint32 = 2
)

const (
	MsgJoinReq  int32 = 1
	MsgDrawReq  int32 = 2
	MsgClearReq int32 = 3
	MsgCloseReq int32 = 4
)

const maxStrokesPerRoom = 4000

type stroke struct {
	ID        uint64  `json:"id"`
	SessionID uint64  `json:"sessionId"`
	X0        float64 `json:"x0"`
	Y0        float64 `json:"y0"`
	X1        float64 `json:"x1"`
	Y1        float64 `json:"y1"`
	Color     string  `json:"color"`
	Width     float64 `json:"width"`
	Ts        int64   `json:"ts"`
}

type roomState struct {
	Name     string
	Strokes  []stroke
	Sessions map[uint64]struct{}
}

type whiteboardState struct {
	rooms map[string]*roomState
}

func newWhiteboardState() *whiteboardState {
	return &whiteboardState{rooms: make(map[string]*roomState)}
}

func (s *whiteboardState) ensureRoom(name string) *roomState {
	if name == "" {
		name = "default"
	}
	if r, ok := s.rooms[name]; ok {
		return r
	}
	r := &roomState{
		Name:     name,
		Strokes:  make([]stroke, 0, 512),
		Sessions: make(map[uint64]struct{}),
	}
	s.rooms[name] = r
	return r
}

type WhiteboardServer struct {
	*zstream.Server
	state       *whiteboardState
	userRoom    map[uint64]string
	nextStroke  uint64
	gateActorID uint64
}

func (s *WhiteboardServer) sendToSession(origin *zmsg.Message, sessionID uint64, data ziface.IMessage) {
	env := *origin
	env.SessionId = sessionID
	s.SendToClient(&env, data)
}

func (s *WhiteboardServer) broadcastRoom(origin *zmsg.Message, room string, data ziface.IMessage) {
	r := s.state.ensureRoom(room)
	for sid := range r.Sessions {
		s.sendToSession(origin, sid, data)
	}
}

func (s *WhiteboardServer) leaveSession(sessionID uint64) {
	room := s.userRoom[sessionID]
	if room == "" {
		return
	}
	r := s.state.ensureRoom(room)
	delete(r.Sessions, sessionID)
	delete(s.userRoom, sessionID)
}

func (s *WhiteboardServer) joinRoom(origin *zmsg.Message, sessionID uint64, room, nickname string) {
	s.leaveSession(sessionID)

	r := s.state.ensureRoom(room)
	r.Sessions[sessionID] = struct{}{}
	s.userRoom[sessionID] = r.Name

	ack, err := zcodec.NewJSONMessage(MsgJoinReq, map[string]any{
		"type":       "join_ack",
		"ok":         true,
		"room":       r.Name,
		"sessionId":  sessionID,
		"nickname":   nickname,
		"strokes":    r.Strokes,
		"serverTime": time.Now().UnixMilli(),
	})
	if err != nil {
		panic(err)
	}
	s.sendToSession(origin, sessionID, ack)

	notice, err := zcodec.NewJSONMessage(MsgJoinReq, map[string]any{
		"type":      "presence",
		"action":    "join",
		"room":      r.Name,
		"sessionId": sessionID,
		"nickname":  nickname,
	})
	if err != nil {
		panic(err)
	}
	s.broadcastRoom(origin, r.Name, notice)
}

func (s *WhiteboardServer) pushStroke(origin *zmsg.Message, sessionID uint64, req stroke) {
	room := s.userRoom[sessionID]
	if room == "" {
		return
	}
	r := s.state.ensureRoom(room)
	s.nextStroke++
	req.ID = s.nextStroke
	req.SessionID = sessionID
	req.Ts = time.Now().UnixMilli()
	if req.Width <= 0 {
		req.Width = 2
	}
	if req.Color == "" {
		req.Color = "#67b8ff"
	}
	r.Strokes = append(r.Strokes, req)
	if len(r.Strokes) > maxStrokesPerRoom {
		r.Strokes = r.Strokes[len(r.Strokes)-maxStrokesPerRoom:]
	}

	msg, err := zcodec.NewJSONMessage(MsgDrawReq, map[string]any{
		"type":   "draw_event",
		"room":   r.Name,
		"stroke": req,
	})
	if err != nil {
		panic(err)
	}
	s.broadcastRoom(origin, r.Name, msg)
}

func (s *WhiteboardServer) clearRoom(origin *zmsg.Message, sessionID uint64) {
	room := s.userRoom[sessionID]
	if room == "" {
		return
	}
	r := s.state.ensureRoom(room)
	r.Strokes = r.Strokes[:0]
	msg, err := zcodec.NewJSONMessage(MsgClearReq, map[string]any{
		"type":      "clear_event",
		"room":      room,
		"sessionId": sessionID,
	})
	if err != nil {
		panic(err)
	}
	s.broadcastRoom(origin, room, msg)
}

func main() {
	addr := flag.String("addr", "127.0.0.1:8011", "gate listen addr")
	webAddr := flag.String("web", "127.0.0.1:8081", "optional static web server addr, empty to disable")
	webRoot := flag.String("webRoot", "./examples", "static web root directory (serve /collab_whiteboard_demo/web/ etc.)")
	connKind := flag.String("conn", "ws", "listen protocol: tcp | ws")
	reactor := flag.Bool("reactor", false, "enable TCP reactor mode (takes effect only when -conn=tcp and no TLS)")
	flag.Parse()

	if *webAddr != "" {
		if st, err := os.Stat(*webRoot); err != nil || !st.IsDir() {
			panic(fmt.Sprintf("invalid -webRoot %q: %v", *webRoot, err))
		}
		go func() {
			mux := http.NewServeMux()
			mux.Handle("/", http.FileServer(http.Dir(*webRoot)))
			srv := &http.Server{
				Addr:              *webAddr,
				Handler:           mux,
				ReadHeaderTimeout: 5 * time.Second,
				ReadTimeout:       10 * time.Second,
				WriteTimeout:      30 * time.Second,
				IdleTimeout:       60 * time.Second,
			}
			fmt.Printf("whiteboard web server start at http://%s/\n", *webAddr)
			fmt.Printf("open: http://%s/collab_whiteboard_demo/web/\n", *webAddr)
			if err := srv.ListenAndServe(); err != nil {
				fmt.Printf("whiteboard web server stopped: %v\n", err)
			}
		}()
	}

	connProto := znet.TCP
	switch *connKind {
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
			{Id: 1, ActorType: ActorTypeGate, Name: "gate", Index: 1, Addr: *addr, Process: 1},
			{Id: 2, ActorType: ActorTypeWhiteboard, Name: "whiteboard", Index: 1, Process: 1},
		},
	})

	var boardSrv *WhiteboardServer
	err := app.RegisterActorFactory(ActorTypeGate, func(a *zstartup.App, c zmodel.ActorConfig) ziface.IServerActor {
		s := zgate.NewServer(c, a.ConnType)
		s.SetReactorMode(*reactor)
		s.WithNetServerHook(func(srv baseziface.IServer) {
			srv.SetHeartbeatTimeout(0)
		})
		s.OnChannelClose(func(ch baseziface.IChannel) {
			if boardSrv == nil || ch == nil {
				return
			}
			m := zmsg.GetMessage()
			m.MsgId = MsgCloseReq
			m.FromClient = true
			m.SessionId = ch.GetChannelId()
			m.SrcActor = s.GetActorId()
			boardSrv.Push(zmodel.ActorCmd{Type: zmodel.CmdTypeClient, Msg: m})
		})
		return s
	})
	if err != nil {
		panic(err)
	}

	err = app.RegisterActorFactory(ActorTypeWhiteboard, func(a *zstartup.App, c zmodel.ActorConfig) ziface.IServerActor {
		s := &WhiteboardServer{
			Server:      zstream.NewServer(c),
			state:       newWhiteboardState(),
			userRoom:    make(map[uint64]string),
			gateActorID: 1,
		}
		boardSrv = s

		s.GetHandleMgr().RegisterHandle(MsgJoinReq, func(ctx context.Context, msg *zmsg.Message) {
			_ = ctx
			var req struct {
				Room     string `json:"room"`
				Nickname string `json:"nickname"`
			}
			_ = zserialize.UnmarshalJson(msg.Data, &req)
			if req.Room == "" {
				req.Room = "default"
			}
			if req.Nickname == "" {
				req.Nickname = fmt.Sprintf("u_%d", msg.SessionId)
			}
			s.joinRoom(msg, msg.SessionId, req.Room, req.Nickname)
		})

		s.GetHandleMgr().RegisterHandle(MsgDrawReq, func(ctx context.Context, msg *zmsg.Message) {
			_ = ctx
			var req stroke
			_ = zserialize.UnmarshalJson(msg.Data, &req)
			s.pushStroke(msg, msg.SessionId, req)
		})

		s.GetHandleMgr().RegisterHandle(MsgClearReq, func(ctx context.Context, msg *zmsg.Message) {
			_ = ctx
			s.clearRoom(msg, msg.SessionId)
		})

		s.GetHandleMgr().RegisterHandle(MsgCloseReq, func(ctx context.Context, msg *zmsg.Message) {
			_ = ctx
			sessionID := msg.SessionId
			room := s.userRoom[sessionID]
			s.leaveSession(sessionID)
			if room == "" {
				return
			}
			notice, err := zcodec.NewJSONMessage(MsgJoinReq, map[string]any{
				"type":      "presence",
				"action":    "leave",
				"room":      room,
				"sessionId": sessionID,
			})
			if err != nil {
				panic(err)
			}
			s.broadcastRoom(msg, room, notice)
		})

		s.GetLogger().Info("whiteboard actor ready", zap.Uint64("gateActorId", s.gateActorID))
		return s
	})
	if err != nil {
		panic(err)
	}

	if err := app.Run(); err != nil {
		panic(err)
	}
}
