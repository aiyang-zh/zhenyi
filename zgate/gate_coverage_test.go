package zgate

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	baseziface "github.com/aiyang-zh/zhenyi-base/ziface"
	basezlog "github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi/zbus"
	"github.com/aiyang-zh/zhenyi/ziface"
	"github.com/aiyang-zh/zhenyi/zmodel"
	"github.com/aiyang-zh/zhenyi/zmsg"
	"github.com/aiyang-zh/zhenyi/zroute"
)

func init() {
	basezlog.NewDefaultLogger()
}

type fakeWireMsg struct {
	msgID int32
	seqID uint32
	data  []byte
}

func (m *fakeWireMsg) GetMsgId() int32         { return m.msgID }
func (m *fakeWireMsg) SetMsgId(v int32)        { m.msgID = v }
func (m *fakeWireMsg) GetSeqId() uint32        { return m.seqID }
func (m *fakeWireMsg) SetSeqId(v uint32)       { m.seqID = v }
func (m *fakeWireMsg) GetMessageData() []byte  { return m.data }
func (m *fakeWireMsg) SetMessageData(v []byte) { m.data = v }
func (m *fakeWireMsg) Reset()                  { m.msgID, m.seqID, m.data = 0, 0, nil }

type fakeChannel struct {
	id           uint64
	authId       uint64
	allow        bool
	limitSet     bool
	closeCallSet bool
	closeCall    func(baseziface.IChannel)
	lastRecSet   bool
	recvBytes    int
	sentMu       sync.Mutex
	sentCount    int
}

// ITransport
func (c *fakeChannel) Start()                                      {}
func (c *fakeChannel) SendBatchMsg(messages []baseziface.IMessage) {}
func (c *fakeChannel) Close()                                      {}
func (c *fakeChannel) GetChannelId() uint64                        { return c.id }
func (c *fakeChannel) IsOpen() bool                                { return true }
func (c *fakeChannel) Flush() error                                { return nil }
func (c *fakeChannel) GetWriterTier() baseziface.BufferTier        { return baseziface.BufferTier(0) }
func (c *fakeChannel) GetBuffered() int                            { return 0 }
func (c *fakeChannel) WriteImmediate(msg baseziface.IWireMessage) error {
	_ = msg
	return nil
}

// ISession
func (c *fakeChannel) GetAuthId() uint64 { return c.authId }
func (c *fakeChannel) SetAuthId(authId uint64) {
	c.authId = authId
}
func (c *fakeChannel) GetRpcId() uint64 { return 0 }
func (c *fakeChannel) SetLimit(rate baseziface.ILimit) {
	_ = rate
	c.limitSet = true
}
func (c *fakeChannel) Allow() bool                         { return c.allow }
func (c *fakeChannel) SetHeartbeatTimeout(d time.Duration) { _ = d }
func (c *fakeChannel) UpdateLastRecTime()                  { c.lastRecSet = true }
func (c *fakeChannel) Check() bool                         { return false }
func (c *fakeChannel) SetCloseCall(closeCall func(baseziface.IChannel)) {
	c.closeCallSet = true
	c.closeCall = closeCall
}
func (c *fakeChannel) Send(msg baseziface.IMessage) {
	c.sentMu.Lock()
	c.sentCount++
	c.sentMu.Unlock()
	if msg != nil {
		msg.Release()
	}
}
func (c *fakeChannel) StartSend(ctx context.Context) { _ = ctx }
func (c *fakeChannel) RecordRecv(dataLen int)        { c.recvBytes += dataLen }

type fakeServer struct {
	mu     sync.Mutex
	byID   map[uint64]baseziface.IChannel
	byAuth map[uint64]baseziface.IChannel
	closed bool
}

func newFakeServer() *fakeServer {
	return &fakeServer{
		byID:   make(map[uint64]baseziface.IChannel),
		byAuth: make(map[uint64]baseziface.IChannel),
	}
}

func (s *fakeServer) Server(ctx context.Context)      { <-ctx.Done() }
func (s *fakeServer) Close()                          { s.closed = true }
func (s *fakeServer) GetEncrypt() baseziface.IEncrypt { return nil }
func (s *fakeServer) HandleRead(channel baseziface.IChannel, message baseziface.IWireMessage) {
	_ = channel
	_ = message
}
func (s *fakeServer) GetChannel(channelId uint64) baseziface.IChannel {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.byID[channelId]
}
func (s *fakeServer) GetAddr() string                           { return "fake" }
func (s *fakeServer) SetMaxConnections(max int64)               { _ = max }
func (s *fakeServer) SetTLSConfig(cfg *baseziface.TLSConfig)    { _ = cfg }
func (s *fakeServer) SetHeartbeatTimeout(timeout time.Duration) { _ = timeout }
func (s *fakeServer) SetChannelAuth(channelId uint64, authId uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch := s.byID[channelId]
	if ch != nil {
		ch.SetAuthId(authId)
		s.byAuth[authId] = ch
	}
}
func (s *fakeServer) GetChannelByAuthId(authId uint64) baseziface.IChannel {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.byAuth[authId]
}
func (s *fakeServer) RemoveChannel(channelId uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.byID, channelId)
}
func (s *fakeServer) SetEncrypt(iEncrypt baseziface.IEncrypt) { _ = iEncrypt }
func (s *fakeServer) SyncMode() bool                          { return false }

type fakeBus struct {
	mu        sync.Mutex
	calls     int
	failN     int
	lastTopic string
}

func (b *fakeBus) Subscribe(topic string, handler zbus.Handler) (zbus.Subscription, error) {
	_ = topic
	_ = handler
	return nil, nil
}
func (b *fakeBus) Broadcast(topic string, data []byte) error {
	_ = data
	b.mu.Lock()
	defer b.mu.Unlock()
	b.calls++
	b.lastTopic = topic
	if b.calls <= b.failN {
		return errors.New("broadcast fail")
	}
	return nil
}

type fakeGroup struct {
	other []zmodel.ActorConfig
	disc  ziface.Discoverer
}

func (g *fakeGroup) AddActor(iActor ziface.IActor)             { _ = iActor }
func (g *fakeGroup) GetActorById(actorId uint64) ziface.IActor { _ = actorId; return nil }
func (g *fakeGroup) GetOtherActorById(actorId uint64) (zmodel.ActorConfig, bool) {
	return zmodel.ActorConfig{}, false
}
func (g *fakeGroup) Run(ctx context.Context) error            { return nil }
func (g *fakeGroup) GetDiscoverer() ziface.Discoverer         { return g.disc }
func (g *fakeGroup) SetDiscoverer(discover ziface.Discoverer) { g.disc = discover }
func (g *fakeGroup) IsSingle() bool                           { return false }
func (g *fakeGroup) GetActorCh() chan ziface.IActor           { return make(chan ziface.IActor) }
func (g *fakeGroup) FindPoolActorByType(actorType uint32) (zmodel.ActorConfig, error) {
	return zmodel.ActorConfig{}, errors.New("no")
}
func (g *fakeGroup) RegisterRoutes(actor ziface.IActor, msgIDs []int32) { _ = actor; _ = msgIDs }
func (g *fakeGroup) LookupActorsByMsgID(msgID int32) []ziface.IActor    { _ = msgID; return nil }
func (g *fakeGroup) GetOtherActorConfigs() []zmodel.ActorConfig         { return g.other }
func (g *fakeGroup) GetScriptEngine(engineType ziface.ScriptEngineType) ziface.IScriptEngine {
	_ = engineType
	return nil
}
func (g *fakeGroup) CloseScriptEngines() {}

type reverseOrderStrategy struct{}

func (reverseOrderStrategy) PickOne(_ *zmsg.Message, cands []zmodel.ActorConfig) int {
	if len(cands) == 0 {
		return -1
	}
	return len(cands) - 1
}

type dummyDiscoverer struct{}

func (dummyDiscoverer) FindRandom(key string) zmodel.ActorConfig {
	_ = key
	return zmodel.ActorConfig{}
}
func (dummyDiscoverer) FindPoll(key string) zmodel.ActorConfig { _ = key; return zmodel.ActorConfig{} }
func (dummyDiscoverer) FindMod(actorType uint32, userId uint64) zmodel.ActorConfig {
	_ = actorType
	_ = userId
	return zmodel.ActorConfig{}
}
func (dummyDiscoverer) Unregister(c zmodel.ActorConfig) error { _ = c; return nil }
func (dummyDiscoverer) Register(c zmodel.ActorConfig) error   { _ = c; return nil }
func (dummyDiscoverer) Watch() chan zmodel.ActorConfig        { return make(chan zmodel.ActorConfig) }
func (dummyDiscoverer) FindAllByPrefix(key string) []zmodel.ActorServerRegister {
	_ = key
	return nil
}

func TestGate_ServerCoreBranches(t *testing.T) {
	cfg := zmodel.ActorConfig{Id: 1, ActorType: 2, Addr: "127.0.0.1:0", Rate: 10, Burst: 10, IsLimiter: true}
	s := NewServer(cfg, 0)

	s.SetRemoteRouteStrategy(nil)
	s.SetRemoteRouteStrategy(zroute.FirstCandidateStrategy{})
	s.SetHTTPAddr("")
	s.SetTraceHook(func(msg *zmsg.Message) { msg.TraceIdHi = 1; msg.SpanId = 2 })
	s.OnNoRoute(func(orig *zmsg.Message) (*zmsg.Message, bool) {
		_ = orig
		return nil, true
	})
	// cover HTTP lazy init
	_ = s.HTTP()
	_ = s.HTTP()

	fs := newFakeServer()
	ch := &fakeChannel{id: 7, authId: 42, allow: true}
	fs.byID[ch.id] = ch
	s.server = fs

	if s.OnAccept(context.Background(), nil) {
		t.Fatalf("expected false for nil channel")
	}
	if !s.OnAccept(context.Background(), ch) {
		t.Fatalf("expected true accept")
	}
	if !ch.limitSet || !ch.closeCallSet {
		t.Fatalf("expected limiter and closecall set")
	}
	if s.metrics.OnlineUsers.Load() != 1 {
		t.Fatalf("online=%d", s.metrics.OnlineUsers.Load())
	}
	s.OnChannelClose(func(c baseziface.IChannel) { _ = c })
	s.channelClose(ch)
	if s.metrics.OnlineUsers.Load() != 0 {
		t.Fatalf("online=%d", s.metrics.OnlineUsers.Load())
	}

	wm := &fakeWireMsg{msgID: 100, seqID: 10, data: []byte("hi")}
	s.OnRead(context.Background(), ch, wm)
	// cover allow=false branch
	ch2 := &fakeChannel{id: 8, authId: 1, allow: false}
	s.OnRead(context.Background(), ch2, wm)
	// cover channel nil branch
	s.OnRead(context.Background(), nil, wm)

	if got := s.GetAuthIdSession(ch.id); got != 42 {
		t.Fatalf("auth=%d", got)
	}
	if got := s.GetAuthIdSession(999); got != 0 {
		t.Fatalf("auth=%d", got)
	}
	s.SetSessionAuth(ch.id, 99)
	if got := fs.GetChannel(ch.id).GetAuthId(); got != 99 {
		t.Fatalf("auth=%d", got)
	}
	if got := s.GetChannel(ch.id); got == nil {
		t.Fatalf("expected channel")
	}

	msg := zmsg.GetMessage()
	msg.MsgId = 123
	msg.SessionId = ch.id
	s.HandleClientMessage(context.Background(), msg)

	// cover gateHandler self branch: register handler for msgId 500
	var handled bool
	s.GetHandleMgr().RegisterHandle(500, func(ctx context.Context, msg *zmsg.Message) {
		_ = ctx
		_ = msg
		handled = true
	})
	msgSelf := zmsg.GetMessage()
	msgSelf.MsgId = 500
	msgSelf.SessionId = ch.id
	s.HandleClientMessage(context.Background(), msgSelf)
	if !handled {
		t.Fatalf("expected self handler invoked")
	}

	msg2 := zmsg.GetMessage()
	msg2.MsgId = 200

	msg2.ToClient = true
	msg2.SeqId = 777
	s.metrics.RTTTracker.Record(ch.id, msg2.SeqId)
	s.HandleRespMessage(context.Background(), msg2)

	// cover sendNoRouteError hook handled=false path
	s.noRouteHandler = func(orig *zmsg.Message) (*zmsg.Message, bool) {
		_ = orig
		return nil, false
	}
	s.sendNoRouteError(msg)

	// cover sendClient: missing authId (no-op)
	msg3 := zmsg.GetMessage()
	s.sendClient(msg3)

	s.localRecvCount = 105
	s.localSentCount = 205
	_ = s.Close(context.Background())
	if !fs.closed {
		t.Fatalf("expected server closed")
	}

	cctx, cancel := context.WithCancel(context.Background())
	cancel()
	s.ReportMetrics(cctx)
}

func TestGate_RunServer_CanceledContext(t *testing.T) {
	cfg := zmodel.ActorConfig{Id: 1, ActorType: 2}
	s := NewServer(cfg, 0)
	fs := newFakeServer()
	s.server = fs
	s.SetInitServer(func(ctx context.Context) error { _ = ctx; return nil })

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.RunServer(ctx); err != nil {
		t.Fatalf("RunServer err: %v", err)
	}
}

func TestGate_NewNetServer_DefaultPanics(t *testing.T) {
	cfg := zmodel.ActorConfig{Id: 1, ActorType: 2}
	s := NewServer(cfg, 0)
	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic")
		}
	}()
	// invalid connType triggers default branch
	s.NewNetServer(context.Background(), 0, "127.0.0.1:0")
}

func TestGate_SendNoRouteError_Nil(t *testing.T) {
	cfg := zmodel.ActorConfig{Id: 1, ActorType: 2}
	s := NewServer(cfg, 0)
	s.sendNoRouteError(nil)
}

func TestGate_RouteToRemoteActor_BusFallback(t *testing.T) {
	cfg := zmodel.ActorConfig{Id: 1, ActorType: 2}
	s := NewServer(cfg, 0)

	g := &fakeGroup{
		other: []zmodel.ActorConfig{
			{Id: 10, ActorType: 100, Index: 1, SupportedMsgIDs: []int32{7}},
			{Id: 11, ActorType: 100, Index: 2, SupportedMsgIDs: []int32{7}},
		},
		disc: dummyDiscoverer{},
	}
	s.SetGroup(g)
	s.SetRemoteRouteStrategy(reverseOrderStrategy{})

	bus := &fakeBus{failN: 1}
	zbus.DefaultBus = bus
	t.Cleanup(func() { zbus.DefaultBus = nil })

	msg := zmsg.GetMessage()
	msg.MsgId = 7
	if ok := s.routeToRemoteActor(msg); !ok {
		t.Fatalf("expected routed")
	}
	if bus.calls != 2 {
		t.Fatalf("broadcast calls=%d", bus.calls)
	}
	if bus.lastTopic == "" {
		t.Fatalf("expected topic set")
	}
}

func TestGate_MonitorStatusBranches(t *testing.T) {
	cfg := zmodel.ActorConfig{Id: 1, ActorType: 2}
	s := NewServer(cfg, 0)

	s.metrics.OnlineUsers.Store(0)
	_ = s.GetMonitorData()
	s.metrics.OnlineUsers.Store(1)
	_ = s.GetMonitorData()
	s.metrics.OnlineUsers.Store(9001)
	_ = s.GetMonitorData()
}

// wrapper to satisfy metricServer for attachNetMetrics
type metricCapServer struct {
	baseziface.IServer
	m  baseziface.IMetrics
	cm baseziface.IChannelMetrics
	sf func() baseziface.ISessionStats
}

func (s *metricCapServer) SetMetrics(m baseziface.IMetrics)               { s.m = m }
func (s *metricCapServer) SetChannelMetrics(m baseziface.IChannelMetrics) { s.cm = m }
func (s *metricCapServer) SetSessionStatsFactory(f func() baseziface.ISessionStats) {
	s.sf = f
}

func TestAttachNetMetricsAndBridgeMethods(t *testing.T) {
	ms := &metricCapServer{IServer: newFakeServer()}
	attachNetMetrics(ms)
	if ms.m == nil || ms.cm == nil || ms.sf == nil {
		t.Fatalf("expected metrics and session stats factory injected")
	}
	_ = ms.sf()
	ms.m.ConnInc()
	ms.m.ConnDec()
	ms.m.ConnRejectedInc()
	ms.cm.BytesRecAdd(0)
	ms.cm.BytesRecAdd(10)
	ms.cm.BytesSentAdd(0)
	ms.cm.BytesSentAdd(10)
	ms.cm.ConnErrorsInc()
	ms.cm.ConnHeartbeatTimeoutInc()
}
