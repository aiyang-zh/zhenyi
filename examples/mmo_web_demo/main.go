package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	baseziface "github.com/aiyang-zh/zhenyi-base/ziface"
	"go.uber.org/zap"

	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi-base/znet"
	"github.com/aiyang-zh/zhenyi-base/zserialize"
	"github.com/aiyang-zh/zhenyi/zaoi"
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
	ActorTypeMMO  uint32 = 2
)

const (
	MsgLoginReq  int32 = 1
	MsgEnterReq  int32 = 10
	MsgMoveReq   int32 = 11
	MsgAttackReq int32 = 12
	MsgCloseReq  int32 = 13
)

const (
	maxPlayerHP    = 100
	attackDamage   = 18
	attackRange    = 56.0
	attackCooldown = 500 * time.Millisecond
	respawnDelay   = 3 * time.Second

	worldMinX      = 0.0
	worldMaxX      = 800.0
	worldMinY      = 0.0
	worldMaxY      = 500.0
	aoiGridSize    = 100.0
	playerViewDist = 200.0
)

type player struct {
	SessionID   uint64           `json:"sessionId"`
	Nickname    string           `json:"nickname"`
	X           float64          `json:"x"`
	Y           float64          `json:"y"`
	SpawnX      float64          `json:"-"`
	SpawnY      float64          `json:"-"`
	Room        string           `json:"room"`
	HP          int              `json:"hp"`
	MaxHP       int              `json:"maxHp"`
	Dead        bool             `json:"dead"`
	RespawnAtMs int64            `json:"respawnAt,omitempty"`
	LastAttack  time.Time        `json:"-"`
	aoiNode     *zaoi.EntityNode `json:"-"`
}

func (p *player) GetId() int64                     { return int64(p.SessionID) }
func (p *player) GetType() zaoi.EntityType         { return zaoi.EntityTypeAll }
func (p *player) GetPosition() zaoi.IVector        { return zaoi.Vector2{X: p.X, Y: p.Y} }
func (p *player) SetPosition(v zaoi.IVector)       { p.X, p.Y = v.GetX(), v.GetY() }
func (p *player) GetViewDistance() float64         { return playerViewDist }
func (p *player) GetAoiNode() *zaoi.EntityNode     { return p.aoiNode }
func (p *player) SetAoiNode(node *zaoi.EntityNode) { p.aoiNode = node }

type worldState struct {
	players map[uint64]*player
}

func newWorldState() *worldState {
	return &worldState{players: make(map[uint64]*player)}
}

func (s *worldState) ensurePlayer(sessionID uint64) *player {
	p := s.players[sessionID]
	if p != nil {
		return p
	}
	x := float64((sessionID*37)%700 + 40)
	y := float64((sessionID*53)%380 + 40)
	p = &player{
		SessionID: sessionID,
		Nickname:  fmt.Sprintf("p_%d", sessionID),
		X:         x,
		Y:         y,
		SpawnX:    x,
		SpawnY:    y,
		Room:      "world",
		MaxHP:     maxPlayerHP,
		HP:        maxPlayerHP,
	}
	s.players[sessionID] = p
	return p
}

func (s *worldState) roomPlayers(room string) []player {
	out := make([]player, 0, len(s.players))
	for _, p := range s.players {
		if p.Room == room {
			out = append(out, *p)
		}
	}
	return out
}

type MmoServer struct {
	*zstream.Server
	state *worldState

	world    *zaoi.WorldManager
	roomZone map[string]int
	nextZone int
	gateID   uint64
}

func (s *MmoServer) staticAoiForRoom(room string) *zaoi.StaticAoi {
	if s.world == nil {
		s.world = zaoi.NewWorldManager()
		s.roomZone = make(map[string]int)
		s.nextZone = 1
	}
	if zid, ok := s.roomZone[room]; ok {
		z := s.world.Zones[zid]
		if z == nil {
			panic("zaoi: missing zone for room " + room)
		}
		return z.IAoi.(*zaoi.StaticAoi)
	}
	id := s.nextZone
	s.nextZone++
	bounds := [4]float64{worldMinX, worldMaxX, worldMinY, worldMaxY}
	a := zaoi.NewStaticAoi(bounds, aoiGridSize)
	s.world.AddZone(&zaoi.Zone{
		Id:     id,
		Name:   room,
		Type:   zaoi.ZoneTypeStatic,
		Bounds: bounds,
		IAoi:   a,
	})
	s.roomZone[room] = id
	return a
}

func (s *MmoServer) leaveRoomAOI(p *player, room string) {
	if room == "" {
		return
	}
	a := s.staticAoiForRoom(room)
	if p.GetAoiNode() != nil {
		a.RemoveEntity(p)
	}
}

func (s *MmoServer) enterRoomAOI(p *player, prevRoom, newRoom string) {
	if prevRoom != "" && prevRoom != newRoom {
		s.leaveRoomAOI(p, prevRoom)
	}
	aoi := s.staticAoiForRoom(newRoom)
	if p.GetAoiNode() == nil {
		if err := aoi.AddEntity(p); err != nil {
			p.X = clamp(p.X, 16, 784)
			p.Y = clamp(p.Y, 16, 484)
			_ = aoi.AddEntity(p)
		}
		return
	}
	if err := aoi.UpdateEntity(p, zaoi.Vector2{X: p.X, Y: p.Y}); err != nil {
		s.leaveRoomAOI(p, newRoom)
		p.X = clamp(p.X, 16, 784)
		p.Y = clamp(p.Y, 16, 484)
		_ = aoi.AddEntity(p)
	}
}

func (s *MmoServer) sendToSession(origin *zmsg.Message, sessionID uint64, data ziface.IMessage) {
	env := *origin
	env.SessionId = sessionID
	s.SendToClient(&env, data)
}

func (s *MmoServer) visiblePlayerList(viewer *player, room string) []player {
	aoi := s.staticAoiForRoom(room)
	if viewer.GetAoiNode() == nil {
		return s.state.roomPlayers(room)
	}
	var buf []zaoi.IEntity
	aoi.GetNearbyEntities(viewer, &buf)
	out := make([]player, 0, len(buf)+2)
	out = append(out, *viewer)
	seen := map[uint64]struct{}{viewer.SessionID: {}}
	for _, e := range buf {
		id := uint64(e.GetId())
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		pl := s.state.players[id]
		if pl != nil && pl.Room == room {
			out = append(out, *pl)
		}
	}
	return out
}

func (s *MmoServer) playerSees(viewer, subject *player) bool {
	if viewer.SessionID == subject.SessionID {
		return true
	}
	if viewer.Room != subject.Room {
		return false
	}
	aoi := s.staticAoiForRoom(viewer.Room)
	if viewer.GetAoiNode() == nil {
		return true
	}
	var buf []zaoi.IEntity
	aoi.GetNearbyEntities(viewer, &buf)
	sid := int64(subject.SessionID)
	for _, e := range buf {
		if e.GetId() == sid {
			return true
		}
	}
	return false
}

func (s *MmoServer) sessionsForCombat(room string, att, tgt *player) []uint64 {
	seen := make(map[uint64]struct{})
	var out []uint64
	for _, q := range s.state.players {
		if q.Room != room {
			continue
		}
		if s.playerSees(q, att) || s.playerSees(q, tgt) {
			if _, ok := seen[q.SessionID]; ok {
				continue
			}
			seen[q.SessionID] = struct{}{}
			out = append(out, q.SessionID)
		}
	}
	return out
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func dist2(ax, ay, bx, by float64) float64 {
	dx := ax - bx
	dy := ay - by
	return dx*dx + dy*dy
}

// applyRespawns 将已到重生时间的死亡玩家拉回出生点并满血；返回需要广播快照的房间名与复活玩家列表（用于同步 AOI）。
func (s *worldState) applyRespawns(nowMs int64) (map[string]struct{}, []*player) {
	rooms := make(map[string]struct{})
	var revived []*player
	for _, p := range s.players {
		if !p.Dead || p.RespawnAtMs == 0 || nowMs < p.RespawnAtMs {
			continue
		}
		p.HP = p.MaxHP
		p.Dead = false
		p.RespawnAtMs = 0
		p.X, p.Y = p.SpawnX, p.SpawnY
		rooms[p.Room] = struct{}{}
		revived = append(revived, p)
	}
	return rooms, revived
}

func (s *MmoServer) broadcastSnapshot(origin *zmsg.Message, room string) {
	for _, p := range s.state.players {
		if p.Room != room {
			continue
		}
		plist := s.visiblePlayerList(p, room)
		snap, err := zcodec.NewJSONMessage(MsgMoveReq, map[string]any{
			"type":        "world_snapshot",
			"room":        room,
			"players":     plist,
			"aoi":         true,
			"viewDist":    playerViewDist,
			"attackRange": attackRange,
		})
		if err != nil {
			panic(err)
		}
		s.sendToSession(origin, p.SessionID, snap)
	}
}

func (s *MmoServer) broadcastCombat(origin *zmsg.Message, room string, att, tgt *player, payload map[string]any) {
	msg, err := zcodec.NewJSONMessage(MsgAttackReq, payload)
	if err != nil {
		panic(err)
	}
	for _, sid := range s.sessionsForCombat(room, att, tgt) {
		s.sendToSession(origin, sid, msg)
	}
}

func (s *MmoServer) flushRespawns(origin *zmsg.Message, nowMs int64) {
	rooms, revived := s.state.applyRespawns(nowMs)
	for _, p := range revived {
		if p.GetAoiNode() != nil {
			aoi := s.staticAoiForRoom(p.Room)
			_ = aoi.UpdateEntity(p, zaoi.Vector2{X: p.X, Y: p.Y})
		}
	}
	for room := range rooms {
		s.broadcastSnapshot(origin, room)
	}
}

func (s *MmoServer) pickAttackTarget(attacker *player, preferID uint64) *player {
	r2 := attackRange * attackRange
	if preferID != 0 && preferID != attacker.SessionID {
		t := s.state.players[preferID]
		if t != nil && t.Room == attacker.Room && !t.Dead && dist2(attacker.X, attacker.Y, t.X, t.Y) <= r2 {
			return t
		}
		// 客户端显式指定了目标但不可攻击：不再退化为「打最近」，避免与点选语义冲突
		return nil
	}
	var best *player
	var bestD float64
	try := func(t *player) {
		if t == nil || t.SessionID == attacker.SessionID || t.Room != attacker.Room || t.Dead {
			return
		}
		d := dist2(attacker.X, attacker.Y, t.X, t.Y)
		if d > r2 {
			return
		}
		if best == nil || d < bestD {
			best, bestD = t, d
		}
	}
	aoi := s.staticAoiForRoom(attacker.Room)
	if aoi != nil && attacker.GetAoiNode() != nil {
		var buf []zaoi.IEntity
		aoi.GetNearbyEntities(attacker, &buf)
		for _, e := range buf {
			try(s.state.players[uint64(e.GetId())])
		}
		if best != nil {
			return best
		}
	}
	for _, t := range s.state.players {
		try(t)
	}
	return best
}

func (s *MmoServer) onSessionClose(sessionID uint64) {
	p := s.state.players[sessionID]
	if p == nil {
		return
	}
	room := p.Room
	s.leaveRoomAOI(p, room)
	delete(s.state.players, sessionID)
	origin := &zmsg.Message{SrcActor: s.gateID}
	s.broadcastSnapshot(origin, room)
}

func (s *MmoServer) tickRespawn(ctx context.Context, nowTs int64) {
	_ = ctx
	origin := &zmsg.Message{SrcActor: s.gateID}
	s.flushRespawns(origin, nowTs)
}

func main() {
	addr := flag.String("addr", "127.0.0.1:8001", "gate listen addr")
	connKind := flag.String("conn", "ws", "listen protocol: tcp | ws")
	reactor := flag.Bool("reactor", false, "enable TCP reactor mode (mac/linux only; takes effect only when -conn=tcp and no TLS)")
	flag.Parse()

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
			{
				Id:        1,
				ActorType: ActorTypeGate,
				Name:      "gate",
				Index:     1,
				Addr:      *addr,
				Process:   1,
			},
			{
				Id:        2,
				ActorType: ActorTypeMMO,
				Name:      "mmo",
				Index:     1,
				Process:   1,
			},
		},
	})

	var mmoSrv *MmoServer
	err := app.RegisterActorFactory(ActorTypeGate, func(a *zstartup.App, c zmodel.ActorConfig) ziface.IServerActor {
		s := zgate.NewServer(c, a.ConnType)
		s.SetReactorMode(*reactor)
		// Demo: disable idle read deadline (znet heartbeat) via hook — no extra zgate API per knob.
		s.WithNetServerHook(func(srv baseziface.IServer) {
			srv.SetHeartbeatTimeout(0)
		})
		s.OnChannelClose(func(ch baseziface.IChannel) {
			if mmoSrv == nil || ch == nil {
				return
			}
			m := zmsg.GetMessage()
			m.MsgId = MsgCloseReq
			m.FromClient = true
			m.SessionId = ch.GetChannelId()
			m.SrcActor = s.GetActorId()
			mmoSrv.Push(zmodel.ActorCmd{
				Type: zmodel.CmdTypeClient,
				Msg:  m,
			})
		})
		s.GetHandleMgr().RegisterHandle(MsgLoginReq, func(ctx context.Context, msg *zmsg.Message) {
			var req struct {
				UserID   int64  `json:"userId"`
				Nickname string `json:"nickname"`
			}
			_ = zserialize.UnmarshalJson(msg.Data, &req)
			if req.Nickname == "" {
				req.Nickname = fmt.Sprintf("u_%d", req.UserID)
			}
			data, err := zcodec.NewJSONMessage(MsgLoginReq, map[string]any{
				"type":      "login_ack",
				"ok":        true,
				"sessionId": msg.SessionId,
				"userId":    req.UserID,
				"nickname":  req.Nickname,
			})
			if err != nil {
				panic(err)
			}
			s.SendToClient(msg, data)
			s.GetLogger().Info("mmo login", zap.Int64("userId", req.UserID), zap.String("nickname", req.Nickname))
		})
		return s
	})
	if err != nil {
		panic(err)
	}

	err = app.RegisterActorFactory(ActorTypeMMO, func(a *zstartup.App, c zmodel.ActorConfig) ziface.IServerActor {
		s := &MmoServer{
			Server: zstream.NewServer(c),
			state:  newWorldState(),
			gateID: 1,
		}
		mmoSrv = s
		s.RegisterTickFn("respawn_tick", 200*time.Millisecond, s.tickRespawn)
		s.GetHandleMgr().RegisterHandle(MsgEnterReq, func(ctx context.Context, msg *zmsg.Message) {
			s.flushRespawns(msg, time.Now().UnixMilli())
			var req struct {
				Nickname string `json:"nickname"`
				Room     string `json:"room"`
			}
			_ = zserialize.UnmarshalJson(msg.Data, &req)
			newRoom := req.Room
			if newRoom == "" {
				newRoom = "world"
			}
			p := s.state.ensurePlayer(msg.SessionId)
			if req.Nickname != "" {
				p.Nickname = req.Nickname
			}
			prevRoom := p.Room
			p.Room = newRoom
			s.enterRoomAOI(p, prevRoom, newRoom)

			players := s.visiblePlayerList(p, newRoom)
			enterAck, err := zcodec.NewJSONMessage(MsgEnterReq, map[string]any{
				"type":        "enter_ack",
				"ok":          true,
				"room":        newRoom,
				"selfId":      p.SessionID,
				"players":     players,
				"aoi":         true,
				"viewDist":    playerViewDist,
				"attackRange": attackRange,
				"worldSize":   map[string]float64{"width": 800, "height": 500},
			})
			if err != nil {
				panic(err)
			}
			s.SendToClient(msg, enterAck)
			// 否则仅新连接收到 enter_ack，房内其他客户端要等某次移动才会 world_snapshot，后进入者不会出现在先进入者画面上
			if prevRoom != "" && prevRoom != newRoom {
				s.broadcastSnapshot(msg, prevRoom)
			}
			s.broadcastSnapshot(msg, newRoom)
		})
		s.GetHandleMgr().RegisterHandle(MsgMoveReq, func(ctx context.Context, msg *zmsg.Message) {
			s.flushRespawns(msg, time.Now().UnixMilli())
			var req struct {
				X float64 `json:"x"`
				Y float64 `json:"y"`
			}
			_ = zserialize.UnmarshalJson(msg.Data, &req)
			p := s.state.ensurePlayer(msg.SessionId)
			if p.Dead {
				return
			}
			p.X = clamp(req.X, 16, 784)
			p.Y = clamp(req.Y, 16, 484)
			if p.GetAoiNode() != nil {
				aoi := s.staticAoiForRoom(p.Room)
				_ = aoi.UpdateEntity(p, zaoi.Vector2{X: p.X, Y: p.Y})
			}
			s.broadcastSnapshot(msg, p.Room)
		})
		s.GetHandleMgr().RegisterHandle(MsgAttackReq, func(ctx context.Context, msg *zmsg.Message) {
			s.flushRespawns(msg, time.Now().UnixMilli())
			var req struct {
				TargetID uint64 `json:"targetId"`
			}
			_ = zserialize.UnmarshalJson(msg.Data, &req)
			att := s.state.ensurePlayer(msg.SessionId)
			ack := func(ok bool, reason string) {
				m, err := zcodec.NewJSONMessage(MsgAttackReq, map[string]any{
					"type":   "attack_ack",
					"ok":     ok,
					"reason": reason,
				})
				if err != nil {
					panic(err)
				}
				s.SendToClient(msg, m)
			}
			if att.Dead {
				ack(false, "dead")
				return
			}
			now := time.Now()
			if now.Sub(att.LastAttack) < attackCooldown {
				ack(false, "cooldown")
				return
			}
			tgt := s.pickAttackTarget(att, req.TargetID)
			if tgt == nil {
				ack(false, "no_target")
				return
			}
			att.LastAttack = now
			tgt.HP -= attackDamage
			if tgt.HP < 0 {
				tgt.HP = 0
			}
			if tgt.HP == 0 {
				tgt.Dead = true
				tgt.RespawnAtMs = now.Add(respawnDelay).UnixMilli()
			}
			ack(true, "")
			s.broadcastCombat(msg, att.Room, att, tgt, map[string]any{
				"type":       "combat_event",
				"action":     "hit",
				"attackerId": att.SessionID,
				"targetId":   tgt.SessionID,
				"damage":     attackDamage,
				"targetHp":   tgt.HP,
				"killed":     tgt.Dead,
			})
			s.broadcastSnapshot(msg, att.Room)
		})
		s.GetHandleMgr().RegisterHandle(MsgCloseReq, func(ctx context.Context, msg *zmsg.Message) {
			s.onSessionClose(msg.SessionId)
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
