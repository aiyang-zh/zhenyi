package zgate

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aiyang-zh/zhenyi-base/zid"
	baseziface "github.com/aiyang-zh/zhenyi-base/ziface"
	"github.com/aiyang-zh/zhenyi-base/zkcp"
	"github.com/aiyang-zh/zhenyi-base/zlimiter"
	"github.com/aiyang-zh/zhenyi-base/znet"
	"github.com/aiyang-zh/zhenyi-base/ztcp"
	"github.com/aiyang-zh/zhenyi-base/zws"
	"github.com/aiyang-zh/zhenyi/zactor"
	"github.com/aiyang-zh/zhenyi/zbus"
	"github.com/aiyang-zh/zhenyi/zhttp"
	"github.com/aiyang-zh/zhenyi/ziface"
	"github.com/aiyang-zh/zhenyi/zmetrics"
	"github.com/aiyang-zh/zhenyi/zmodel"
	"github.com/aiyang-zh/zhenyi/zmsg"
	"github.com/aiyang-zh/zhenyi/zroute"
	"go.uber.org/zap"
)

// Compile-time assertion that Server implements IServerActor.
// 编译期断言：Server 满足 IServerActor（带 RunServer 生命周期）。
var _ ziface.IServerActor = (*Server)(nil)

// Server is the gateway actor that owns a long-connection server and routes client messages.
// Server 是网关 Actor：持有底层长连接 Server，并负责将客户端消息路由到后端 Actor。
type Server struct {
	*zactor.Actor
	*SessionManager
	server           baseziface.IServer
	tlsConfig        *baseziface.TLSConfig
	connType         znet.ConnProtocol
	channelCloseCall func(baseziface.IChannel)
	metrics          *ServerMetrics
	router           ziface.LocalRouter
	localRecvCount   int64
	localSentCount   int64

	httpAddr   string // Auto-start HTTP in Init when non-empty / 非空时 Init 内自动起 HTTP 监听，空表示不启
	httpOnce   sync.Once
	httpServer ziface.IHttpServer

	remoteStrategy zroute.RemoteRouteStrategy
	// Reusable buffer for remote candidate list construction.
	// 远程候选构建复用缓冲。
	// routeToRemoteActor runs in Gate actor main loop, so single-thread reuse reduces per-call make.
	// routeToRemoteActor 在 Gate Actor 主循环内调用，单线程复用该切片可减少每次 make。
	remoteCandidatesBuf []zmodel.ActorConfig

	noRouteHandler func(orig *zmsg.Message) (reply *zmsg.Message, handled bool)

	traceInjectGateRecv func(msg *zmsg.Message)

	// payloadEncrypt 线协议 body 层加解密（在 TLS 记录之内、12 字节头之后的 payload）。
	// 非 nil 时在 NewNetServer 内调用底层 IServer.SetEncrypt；nil 表示使用默认 BaseEncrypt（不加密）。
	payloadEncrypt baseziface.IEncrypt

	// netServerHook runs after underlying ztcp/zws/zkcp server is created and TLS/encrypt/shared-send are applied.
	// Use WithNetServerHook to tune BaseServer (e.g. SetHeartbeatTimeout). Multiple calls chain in order.
	netServerHook func(baseziface.IServer)

	// useReactorMode enables ztcp ServerReactor (zreactor) for single-loop TCP read.
	// useReactorMode 会让 Gate 在满足条件时调用底层 ztcp.ServerReactor。
	useReactorMode bool
	// useSharedSendWorkerMode enables shared send workers on underlying net server.
	// useSharedSendWorkerMode 开启底层共享写 worker 模式（默认关闭，保持历史行为）。
	useSharedSendWorkerMode bool
}

// NewServer creates a gateway server actor with given actor config and connection protocol.
// NewServer 创建网关 Server Actor，指定 Actor 配置与底层连接协议。
func NewServer(actorConfig zmodel.ActorConfig, connType znet.ConnProtocol) *Server {
	s := &Server{
		Actor:    zactor.NewActor(actorConfig),
		connType: connType,
		metrics: &ServerMetrics{
			RTTTracker: NewLockFreeRTTTracker(zmodel.GetFrameworkTuning().RTTBufferSize, zmodel.GetFrameworkTuning().RTTMaxSamples),
		},
		SessionManager: NewSessionManager(),
		router:         zactor.NewDefaultLocalRouter(),
		remoteStrategy: zroute.FirstCandidateStrategy{},
		// Initialize with small capacity and grow/reuse based on load.
		// 远程候选复用缓冲：初始给小容量，后续按负载按需扩容并复用。
		remoteCandidatesBuf: make([]zmodel.ActorConfig, 0, 8),
	}
	s.SetIActor(s)
	return s
}

// SetRemoteRouteStrategy sets remote actor picking strategy.
// SetRemoteRouteStrategy 设置跨进程远程 Actor 的选址策略。
// Passing nil restores default first-candidate behavior.
// 传 nil 表示恢复为默认策略（保持历史行为：第一个候选优先）。
func (s *Server) SetRemoteRouteStrategy(strategy zroute.RemoteRouteStrategy) {
	if strategy == nil {
		s.remoteStrategy = zroute.FirstCandidateStrategy{}
		return
	}
	s.remoteStrategy = strategy
}

// SetHTTPAddr sets HTTP listen address (e.g. ":8080", "0.0.0.0:8080").
// SetHTTPAddr 设置 HTTP 监听地址（如 ":8080"、"0.0.0.0:8080"）。
// If non-empty, RunServer auto-starts HTTP without calling HTTP().Run.
// RunServer 时若非空会自动起 HTTP（无需再调 HTTP().Run）。
// Call before RunServer; register routes first.
// 不设或设为空则不启 HTTP。需在 RunServer 前调用；先注册路由再跑 Gate。
func (s *Server) SetHTTPAddr(addr string) { s.httpAddr = addr }

// SetTLSConfig sets transport TLS/GM-TLS config.
// SetTLSConfig 设置底层长连接 TLS/GM-TLS 配置。
// Nil or Mode=None disables TLS.
// 传 nil 或 Mode=None 表示不启用 TLS。
// Call before Init/RunServer.
// 需在 Init/RunServer 前调用。
func (s *Server) SetTLSConfig(cfg *baseziface.TLSConfig) {
	s.tlsConfig = cfg
}

// SetEncrypt sets application-layer payload encryption for wire messages (after framing header).
// SetEncrypt 设置线协议 payload 加解密（在 12 字节头之后的 body 上，与 TLS 记录正交）。
// Call from actor factory before RunServer/Init; applied in NewNetServer to the underlying ztcp/zws/zkcp server.
// 在 RegisterActorFactory 里、Init 之前调用；NewNetServer 时下发到底层 Server。
func (s *Server) SetEncrypt(enc baseziface.IEncrypt) {
	s.payloadEncrypt = enc
}

// WithNetServerHook registers a callback invoked after the underlying net server (ztcp/zws/zkcp) is created
// and Gate applies shared-send / TLS / payload encrypt. Use it for extra tuning without extending zgate for each knob.
// Call before Init/RunServer. Multiple calls chain in registration order.
func (s *Server) WithNetServerHook(fn func(baseziface.IServer)) {
	if s == nil || fn == nil {
		return
	}
	prev := s.netServerHook
	s.netServerHook = func(srv baseziface.IServer) {
		if prev != nil {
			prev(srv)
		}
		fn(srv)
	}
}

// SetReactorMode enables/disables reactor mode for TCP read.
// When enabled, Gate will call ztcp.ServerReactor if:
// - connType == TCP
// - tlsConfig == nil (transport TLS not supported by reactor)
// - underlying server is *ztcp.Server
func (s *Server) SetReactorMode(enabled bool) {
	if s == nil {
		return
	}
	s.useReactorMode = enabled
}

// SetSharedSendWorkerMode enables/disables shared send worker mode.
// It applies to underlying servers that support this capability (ztcp/zws/zkcp via znet.BaseServer).
func (s *Server) SetSharedSendWorkerMode(enabled bool) {
	if s == nil {
		return
	}
	s.useSharedSendWorkerMode = enabled
}

// SetStandardTLS configures standard TLS (RSA/ECDSA) by cert files.
// SetStandardTLS 使用证书文件配置标准 TLS（RSA/ECDSA）。
// Call before Init/RunServer.
// 需在 Init/RunServer 前调用。
func (s *Server) SetStandardTLS(certFile, keyFile string) error {
	cfg, err := znet.NewStandardTLSConfig(certFile, keyFile)
	if err != nil {
		return err
	}
	s.tlsConfig = cfg
	return nil
}

// SetGMTLS configures GM-TLS (SM2 dual-certificate) by cert files.
// SetGMTLS 使用证书文件配置国密 GM-TLS（SM2 双证书）。
// Call before Init/RunServer.
// 需在 Init/RunServer 前调用。
func (s *Server) SetGMTLS(signCertFile, signKeyFile, encCertFile, encKeyFile string) error {
	cfg, err := znet.NewGMTLSConfig(signCertFile, signKeyFile, encCertFile, encKeyFile)
	if err != nil {
		return err
	}
	s.tlsConfig = cfg
	return nil
}

// SetGMTLSSingle configures GM-TLS with one cert/key pair.
// SetGMTLSSingle 使用单证书配置国密 GM-TLS（签名/加密共用）。
// Call before Init/RunServer.
// 需在 Init/RunServer 前调用。
func (s *Server) SetGMTLSSingle(certFile, keyFile string) error {
	cfg, err := znet.NewGMTLSConfigSingle(certFile, keyFile)
	if err != nil {
		return err
	}
	s.tlsConfig = cfg
	return nil
}

// SetGMTLSCipherSuites 设置国密 TLS 套件列表（须在 SetGMTLS / SetGMTLSSingle 之后、Init / RunServer 之前调用）。
// suites 为 nil 时使用 zgmtls 默认（优先 ECDHE 套件）。未启用 GM-TLS 时调用无效果。
func (s *Server) SetGMTLSCipherSuites(suites []uint16) {
	if s == nil || s.tlsConfig == nil || s.tlsConfig.Mode != baseziface.TLSModeGM || s.tlsConfig.GMConfig == nil {
		return
	}
	s.tlsConfig.GMConfig.SetCipherSuites(suites)
}

// OnNoRoute sets hook for unresolved routing.
// OnNoRoute 设置“无路由”处理钩子。
// It is called when Gate cannot route a message; business decides reply behavior.
// 当 Gate 无法将消息路由到任何 Actor 时会调用该函数，由业务决定是否以及如何回复客户端。
//
// Return contract:
// 返回值约定：
// - handled=false：表示业务不处理，Gate 将仅记录 warning 日志（不回包）
// - handled=true 且 reply!=nil：Gate 将向客户端发送 reply
// - handled=true 且 reply==nil：业务选择不回包，Gate 仅记录 warning 日志
func (s *Server) OnNoRoute(fn func(orig *zmsg.Message) (reply *zmsg.Message, handled bool)) {
	s.noRouteHandler = fn
}

// SetTraceHook sets trace injection hook for incoming client messages.
// SetTraceHook 设置 trace 注入钩子，用于在 Gate 收到客户端消息时写入 TraceId/SpanId 等信息。
// Without hook, Gate uses fast-path IDs to avoid tracing dependencies and overhead.
// 不设置时走快路径（使用快速 ID），避免引入 tracing 依赖和额外开销。
func (s *Server) SetTraceHook(injectGateRecv func(msg *zmsg.Message)) {
	s.traceInjectGateRecv = injectGateRecv
}

// HTTP returns the IHttpServer attached to this gate (actor already set to s).
// HTTP 返回挂在本 Gate 上的 IHttpServer（已 SetActor(s)）。
// Use it to register HTTP routes; when SetHTTPAddr(addr) is non-empty, RunServer auto starts it.
// 用于注册 HTTP 路由；若已 SetHTTPAddr(addr) 且非空，RunServer 时会自动启动 HTTP。
func (s *Server) HTTP() ziface.IHttpServer {
	s.httpOnce.Do(func() {
		s.httpServer = zhttp.NewStdServer()
		s.httpServer.SetActor(s)
	})
	return s.httpServer
}

// Init initializes underlying net server and starts metrics reporter.
// Init 初始化底层网络 Server，并启动指标上报协程。
func (s *Server) Init(ctx context.Context) error {
	err := s.Actor.Init(ctx)
	if err != nil {
		return err
	}
	s.NewNetServer(ctx, s.connType, s.Addr)

	go func() {
		defer s.GetLogger().Recover("GateServer.ReportMetrics")
		s.ReportMetrics(ctx)
	}()
	return nil
}

// NewNetServer creates underlying long-connection server implementation by protocol.
// NewNetServer 按协议创建底层长连接 Server 实现。
func (s *Server) NewNetServer(ctx context.Context, connType znet.ConnProtocol, addr string) {
	handlers := znet.ServerHandlers{
		OnAccept: func(channel baseziface.IChannel) bool { return s.OnAccept(ctx, channel) },
		OnRead:   func(channel baseziface.IChannel, message baseziface.IWireMessage) { s.OnRead(ctx, channel, message) },
	}
	switch connType {
	case znet.TCP:
		s.server = ztcp.NewServer(addr, handlers)
	case znet.WebSocket:
		s.server = zws.NewServer(addr, handlers)
	case znet.KCP:
		s.server = zkcp.NewServer(addr, handlers)
	default:
		s.GetLogger().Panic("RunServer err", zap.Int("connType", int(connType)))
	}
	// Attach connection-level metrics to align zmetrics.Conn* and byte counters.
	// 为底层网络服务器注入连接级与单连接级指标，实现 zmetrics.Conn* 与字节统计对齐。
	if s.server != nil {
		if ssw, ok := s.server.(interface{ SetSharedSendWorkerMode(bool) }); ok {
			ssw.SetSharedSendWorkerMode(s.useSharedSendWorkerMode)
		}
		if s.tlsConfig != nil {
			s.server.SetTLSConfig(s.tlsConfig)
		}
		if s.payloadEncrypt != nil {
			s.server.SetEncrypt(s.payloadEncrypt)
		}
		if s.netServerHook != nil {
			s.netServerHook(s.server)
		}
		attachNetMetrics(s.server)
	}
}

// OnAccept is called when a new connection is accepted.
// OnAccept 在新连接建立时触发。
func (s *Server) OnAccept(ctx context.Context, channel baseziface.IChannel) bool {
	// ✅ 直接使用传入的 channel，无需 GetChannel，避免循环依赖
	if channel == nil {
		return false
	}
	// Update online connection counters.
	// 更新连接统计。
	s.metrics.OnlineUsers.Add(1)
	if s.IsLimiter {
		channel.SetLimit(zlimiter.NewLimiter(s.Rate, s.Burst))
	}
	channel.SetCloseCall(s.channelClose)
	return true
}

// wrapCloseCall 包装 closeCall，在业务回调之前先处理统计
func (s *Server) channelClose(channel baseziface.IChannel) {
	s.metrics.OnlineUsers.Add(-1)
	if s.channelCloseCall != nil {
		s.channelCloseCall(channel)
	}
}

// OnRead is called when a client packet is received from a connection.
// OnRead 在连接收到客户端数据包时触发。
func (s *Server) OnRead(ctx context.Context, channel baseziface.IChannel, netMessage baseziface.IWireMessage) {
	defer func() {
		if x := recover(); x != nil {
			s.GetLogger().Warn("OnRead err", zap.Any("err", x))
		}
	}()

	// ✅ 直接使用传入的 channel，无需 GetChannel，避免 map 查找
	if channel == nil {
		s.GetLogger().Warn("channel is nil")
		return
	}

	// ✅ 限流检查
	if !channel.Allow() {
		s.GetLogger().Warn("channel not allow", zap.Uint64("channelId", channel.GetChannelId()))
		return
	}

	// ✅ 更新心跳时间
	channel.UpdateLastRecTime()

	// ✅ 记录接收统计
	dataLen := len(netMessage.GetMessageData())
	channel.RecordRecv(dataLen)

	// ✅ 生成消息并推送到 Actor
	msg := s.GenCliMsg(channel, netMessage)
	s.Push(msg)

	// 批量更新统计（减少原子操作），per-Server 计数器
	if atomic.AddInt64(&s.localRecvCount, 1)%100 == 0 {
		s.metrics.recvCount.Add(100)
		s.metrics.recvCountTotal.Add(100)
	}

	// FirstPacketTime 仅设置一次，保持原样
	if s.metrics.FirstPacketTime.Load() == 0 {
		s.metrics.FirstPacketTime.CompareAndSwap(0, time.Now().UnixNano())
	}

	// ✅ 使用无锁追踪器 (记录 channelId + seqId，确保唯一性)
	if netMessage.GetSeqId() > 0 {
		s.metrics.RTTTracker.Record(channel.GetChannelId(), netMessage.GetSeqId())
	}
}

// GenCliMsg converts wire message into actor command and fills message envelope fields.
// GenCliMsg 将网络消息转换为 ActorCmd，并填充消息信封字段。
func (s *Server) GenCliMsg(channel baseziface.IChannel, netMessage baseziface.IWireMessage) zmodel.ActorCmd {
	msg := zmsg.GetMessage()
	msg.MsgId = netMessage.GetMsgId()
	msg.SeqId = netMessage.GetSeqId()
	msg.Data = append(msg.Data[:0], netMessage.GetMessageData()...)
	msg.SessionId = channel.GetChannelId()
	msg.FromClient = true
	msg.SrcActor = s.GetActorId()
	if s.traceInjectGateRecv != nil {
		s.traceInjectGateRecv(msg)
	} else {
		msg.TraceIdHi = zid.NextFast()
		msg.SpanId = s.GetActorId()
	}
	return zmodel.ActorCmd{
		Type: zmodel.CmdTypeClient,
		Msg:  msg,
	}
}

// GetAuthIdSession returns authId bound to the channel.
// GetAuthIdSession 返回 channel 绑定的 authId。
func (s *Server) GetAuthIdSession(channelId uint64) uint64 {
	channel := s.server.GetChannel(channelId)
	if channel == nil {
		return 0
	}
	return channel.GetAuthId()
}

// SetSessionAuth sets authId for the channel.
// SetSessionAuth 为 channel 设置 authId。
func (s *Server) SetSessionAuth(channelId, authId uint64) {
	s.server.SetChannelAuth(channelId, authId)
}

// GetChannel returns channel by channelId.
// GetChannel 按 channelId 获取连接 channel。
func (s *Server) GetChannel(channelId uint64) baseziface.IChannel {
	return s.server.GetChannel(uint64(channelId))
}

// OnChannelClose sets business hook invoked after internal close handling.
// OnChannelClose 设置连接关闭回调（内部统计处理后再调用业务回调）。
func (s *Server) OnChannelClose(call func(baseziface.IChannel)) {
	s.channelCloseCall = call
}

// HandleClientMessage handles inbound client message in gate actor.
// HandleClientMessage 在 Gate Actor 内处理客户端消息。
func (s *Server) HandleClientMessage(ctx context.Context, msg *zmsg.Message) {
	// ✅ 改用 GetChannel（msg.SessionId 现在是 channelId）
	channel := s.server.GetChannel(msg.SessionId)
	if channel == nil {
		return
	}

	s.gateHandler(ctx, channel, msg)
}

func (s *Server) gateHandler(ctx context.Context, channel baseziface.IChannel, msg *zmsg.Message) {
	// 1. 优先检查是否由 Gate 自身处理（本 Actor 已注册 handler）
	if _, ok := s.GetMsgList()[msg.MsgId]; ok {
		zmetrics.GateRouteGateSelf.Inc()
		s.GetHandleMgr().HandleClientMessage(ctx, msg)
		return
	}

	// 2. 其次尝试使用本进程内路由表将消息路由到其他本地 Actor。
	//    Router 可以基于完整的 message 做分片/粘性路由。
	if s.router != nil && s.GetGroup() != nil {
		if target, err := s.router.RouteLocal(s.GetGroup(), msg); err == nil && target != nil {
			if target.GetActorId() != s.GetActorId() {
				zmetrics.GateRouteLocal.Inc()
				// 路由到本进程内的其他 Actor；需 Retain，否则 Gate 释放后 IM 收到已失效。
				target.Push(zmodel.ActorCmd{
					Type: zmodel.CmdTypeClient,
					Msg:  msg.Retain(),
				})
				return
			}
		}
	}

	// 3. 再次尝试通过 discovery 构建的全局视图，选择跨进程 Actor。
	if s.GetGroup() != nil && s.GetGroup().GetDiscoverer() != nil {
		if ok := s.routeToRemoteActor(msg); ok {
			return
		}
	}

	// 4. 无任何路由：向客户端回写系统错误，便于调用方感知失败（P1-1 错误处理统一）。
	zmetrics.GateRouteNoRoute.Inc()
	s.sendNoRouteError(msg)
}

// HandleRespMessage handles response messages.
// HandleRespMessage 消息处理。
func (s *Server) HandleRespMessage(ctx context.Context, msg *zmsg.Message) {
	if msg.ToClient {
		s.sendClient(msg)
		return
	}
}

// HandleToClientFastPath implements ziface.IToClientFastPath.
// HandleToClientFastPath 实现 ziface.IToClientFastPath。
// Gate forwards ToClient responses directly to connection and does not depend on actor single-threaded state.
// Gate 对 ToClient 响应的处理是“纯转发到连接”，不依赖 Actor 单线程状态，因此允许直达快路径。
func (s *Server) HandleToClientFastPath(msg *zmsg.Message) bool {
	if msg == nil || !msg.ToClient {
		return false
	}
	s.sendClient(msg)
	return true
}

// routeToRemoteActor tries routing message to remote actors using discovery records.
// routeToRemoteActor 尝试基于 discovery 注册信息，将消息路由到远程 Actor。
// Returns true on successful send; false when no suitable target is found.
// 返回 true 表示已成功选择远程目标并发出；false 表示未找到合适目标，应继续后续 fallback。
func (s *Server) routeToRemoteActor(msg *zmsg.Message) bool {
	group := s.GetGroup()
	if group == nil || group.GetDiscoverer() == nil || msg == nil {
		return false
	}

	// Prefer optional group readonly fast view (allocation-free), else fallback to scan.
	// 优先使用 Group 的可选快表只读视图（无分配）；否则回退到线性扫描。
	var candidates []zmodel.ActorConfig
	if fast, ok := any(group).(ziface.IGroupRemoteRouteTableView); ok {
		candidates = fast.LookupOtherActorConfigsByMsgIDView(msg.MsgId)
	} else {
		// Build candidates for current msgId from discovery configs of other processes.
		// 基于 discovery 中的其他进程 Actor 配置，构建当前 msgId 的候选列表。
		configs := group.GetOtherActorConfigs()
		if len(configs) == 0 {
			return false
		}

		// Pre-grow by config count to reduce append expansion cost.
		// 按当前 configs 数量预扩容，降低 append 扩容成本。
		if cap(s.remoteCandidatesBuf) < len(configs) {
			s.remoteCandidatesBuf = make([]zmodel.ActorConfig, 0, len(configs))
		}
		candidates = s.remoteCandidatesBuf[:0]
		for _, cfg := range configs {
			if len(cfg.SupportedMsgIDs) == 0 {
				continue
			}
			// Linear scan SupportedMsgIDs to check msgId support.
			// 线性扫描 SupportedMsgIDs，判断是否支持当前 msgId。
			for _, id := range cfg.SupportedMsgIDs {
				if id == msg.MsgId {
					candidates = append(candidates, cfg)
					break
				}
			}
		}
		// Store reusable buffer and preserve capacity for next call.
		// 记录复用缓冲（保留容量供下次复用）。
		s.remoteCandidatesBuf = candidates[:0]
	}

	if len(candidates) == 0 {
		return false
	}

	zmetrics.GateRouteRemoteCandidates.Set(int64(len(candidates)))

	// Broadcast via TopicBus to remote process, where peer actor restores ActorCmd.
	// 通过 TopicBus 将消息广播到目标 Actor 所在进程，由对端 Actor 订阅并还原为 ActorCmd。
	if zbus.DefaultBus == nil {
		s.GetLogger().Error("routeToRemoteActor: remote bus is not configured")
		zmetrics.GateRouteRemoteFail.Inc()
		return false
	}

	strategy := s.remoteStrategy
	if strategy == nil {
		strategy = zroute.FirstCandidateStrategy{}
	}
	pickIdx := strategy.PickOne(msg, candidates)
	if pickIdx < 0 || pickIdx >= len(candidates) {
		pickIdx = 0
	}

	// Try preferred candidate first, then remaining in original order (no ordered slice allocation).
	// 先尝试策略选中的首选，再按原顺序尝试其余候选（不构造 ordered 新切片，避免分配）。
	// Reuse encoding buffer while re-encoding per target TarActor to preserve semantics.
	// 编码缓冲复用：保持每次候选重试都按当前 TarActor 编码（语义不变），
	// and avoid MarshalPooled/GetBytesBuffer pool churn.
	// 同时避免每次 MarshalPooled/GetBytesBuffer 带来的池对象分配与回收开销。
	var encoded []byte
	for i := 0; i < len(candidates); i++ {
		var target zmodel.ActorConfig
		if i == 0 {
			target = candidates[pickIdx]
		} else {
			origIdx := i - 1
			if origIdx >= pickIdx {
				origIdx++
			}
			target = candidates[origIdx]
		}
		// Count fallback only when first candidate fails.
		// 只有在“第一个候选失败”时才算一次 fallback。
		if i > 0 {
			zmetrics.GateRouteRemoteFallback.Inc()
		}
		zmetrics.GateRouteRemoteTry.Inc()

		msg.TarActor = target.Id
		topic := target.GetTopic()
		need := msg.Size()
		if cap(encoded) < need {
			encoded = make([]byte, need)
		}
		encoded = encoded[:need]
		n, err := msg.MarshalTo(encoded)
		if err != nil {
			s.GetLogger().Error("routeToRemoteActor: marshal failed",
				zap.Int32("msgId", msg.MsgId),
				zap.Error(err))
			zmetrics.GateRouteRemoteFail.Inc()
			return false
		}
		err = zbus.DefaultBus.Broadcast(topic, encoded[:n])
		if err != nil {
			s.GetLogger().Error("routeToRemoteActor: broadcast failed",
				zap.Int32("msgId", msg.MsgId),
				zap.Uint64("targetActorId", target.Id),
				zap.String("topic", topic),
				zap.Error(err))
			zmetrics.GateRouteRemoteFail.Inc()
			continue
		}
		zmetrics.GateRouteRemote.Inc()
		return true
	}

	// All candidates failed; let caller continue no-route fallback.
	// 候选都失败，交给上层走 no-route fallback。
	return false
}

// sendNoRouteError is called when Gate cannot route to any actor.
// sendNoRouteError 在 Gate 无法将消息路由到任何 Actor 时触发。
// By default it logs warning; with OnNoRoute hook, business controls reply behavior.
// 默认仅记录 warning 日志；如注册了 OnNoRoute 钩子，则由业务决定是否回包以及回包内容。
func (s *Server) sendNoRouteError(orig *zmsg.Message) {
	if orig == nil {
		s.GetLogger().Warn("no route for message (nil message)")
		return
	}

	if s.noRouteHandler != nil {
		if reply, handled := s.noRouteHandler(orig); handled {
			if reply != nil {
				s.sendClient(reply)
			}
			s.GetLogger().Warn("no route for message (handled by hook)",
				zap.Int32("msgId", orig.MsgId),
				zap.Uint32("seqId", orig.SeqId))
			return
		}
	}

	s.GetLogger().Warn("no route for message",
		zap.Int32("msgId", orig.MsgId),
		zap.Uint64("sessionId", orig.SessionId),
		zap.Uint32("seqId", orig.SeqId))
}

func (s *Server) sendClient(msg *zmsg.Message) {
	if msg.SessionId > 0 {
		channel := s.server.GetChannel(msg.SessionId)
		if channel != nil {
			channel.Send(msg.Retain())

			if atomic.AddInt64(&s.localSentCount, 1)%100 == 0 {
				s.metrics.sentCount.Add(100)
				s.metrics.sentCountTotal.Add(100)
			}

			if msg.SeqId > 0 {
				if rtt, found := s.metrics.RTTTracker.Complete(channel.GetChannelId(), msg.SeqId); found {
					s.metrics.RecordRTT(rtt)
				}
			}
		}
	}
}

// Close gracefully shuts down HTTP (if supported), long-connection server, and underlying actor runtime.
// Close 优雅关闭 HTTP（若支持）、长连接 Server 以及底层 Actor 运行时。
func (s *Server) Close(ctx context.Context) error {
	// flush 残留的批量计数
	if r := atomic.SwapInt64(&s.localRecvCount, 0); r > 0 {
		s.metrics.recvCount.Add(r % 100)
		s.metrics.recvCountTotal.Add(r % 100)
	}
	if w := atomic.SwapInt64(&s.localSentCount, 0); w > 0 {
		s.metrics.sentCount.Add(w % 100)
		s.metrics.sentCountTotal.Add(w % 100)
	}

	// 优雅关闭 HTTP（若实现支持）
	if s.httpServer != nil {
		if sh, ok := s.httpServer.(interface{ Shutdown(context.Context) error }); ok {
			_ = sh.Shutdown(ctx)
		}
	}
	// 关闭底层长连接服务
	if s.server != nil {
		s.server.Close()
	}
	// 关闭 Actor：取消 ctx、取消订阅、关闭 mailbox/worker pool 等
	if s.Actor != nil {
		return s.Actor.Close(ctx)
	}
	return nil
}

// RunServer starts long-connection server and optional HTTP server, then runs optional init hook.
// RunServer 启动长连接 Server 与可选 HTTP Server，然后执行可选初始化钩子。
func (s *Server) RunServer(ctx context.Context) error {
	if s.server != nil {
		// Reactor mode is a single-loop TCP read path. Transport TLS is not supported.
		if s.useReactorMode && s.connType == znet.TCP && s.tlsConfig == nil {
			if tcpSrv, ok := s.server.(*ztcp.Server); ok {
				// ServerReactor 在 !linux/!darwin 的 stub 中会 panic；这里显式回退，避免把整个进程打崩。
				if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
					go func() {
						defer s.GetLogger().Recover("GateServer.Reactor")
						tcpSrv.ServerReactor(ctx)
					}()
					s.GetLogger().Info("Gate long-conn listening (reactor)", zap.String("addr", s.Addr))
				} else {
					s.server.Server(ctx)
					s.GetLogger().Info("Gate long-conn listening", zap.String("addr", s.Addr))
				}
			} else {
				s.server.Server(ctx)
				s.GetLogger().Info("Gate long-conn listening", zap.String("addr", s.Addr))
			}
		} else {
			s.server.Server(ctx)
			s.GetLogger().Info("Gate long-conn listening", zap.String("addr", s.Addr))
		}
	}
	if s.httpAddr != "" {
		go func() {
			defer s.GetLogger().Recover("GateServer.HTTP")
			s.GetLogger().Info("Gate HTTP listening", zap.String("addr", s.httpAddr))
			if err := s.HTTP().Run(s.httpAddr); err != nil {
				// 避免在 shutdown 场景下噪音：ctx 已结束则不再报警
				select {
				case <-ctx.Done():
					return
				default:
				}
				s.GetLogger().Error("Gate HTTP server exited", zap.String("addr", s.httpAddr), zap.Error(err))
			}
		}()
	}
	return s.CallInitServer(ctx)
}

// ReportMetrics periodically reports runtime metrics/logs for the gate.
// ReportMetrics 定期报告网关运行期指标/日志。
func (s *Server) ReportMetrics(ctx context.Context) {
	// 5秒打印一次日志
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	lastReportTime := time.Now()

	// 初始化内存快照
	s.metrics.lastMemStats = &runtime.MemStats{}
	runtime.ReadMemStats(s.metrics.lastMemStats)
	s.metrics.lastCollectTime = time.Now()
	lastMemSampleTime := s.metrics.lastCollectTime
	// 初始化缓存
	s.metrics.MemAllocBytes.Store(s.metrics.lastMemStats.Alloc)
	s.metrics.MemSysBytes.Store(s.metrics.lastMemStats.Sys)
	s.metrics.MemNumGC.Store(uint64(s.metrics.lastMemStats.NumGC))

	// STW 风险控制：ReadMemStats 采样不必每 5 秒一次，降低频率即可显著减少抖动。
	const memStatsInterval = 30 * time.Second

	// 缓存的内存/GC派生指标（仅在采样时更新）
	var memAllocMB, memSysMB, allocRateMB float64
	var deltaGC int
	var avgPause time.Duration
	var deltaPauseNs int64

	for {
		select {
		case <-ctx.Done():
			return

		// --- 核心统计报告 ---
		case <-ticker.C:
			// A. 获取本周期流量增量 (Swap 会重置计数器，必须先做)
			deltaRecv := s.metrics.recvCount.Swap(0)
			deltaSent := s.metrics.sentCount.Swap(0)
			deltaTotal := deltaRecv + deltaSent

			// B. 获取在线人数
			onlineCount := s.metrics.OnlineUsers.Load()

			// --- 🛑 静默检测优化 ---
			// 只有当“没人在”且“刚才5秒也没流量”时，才视为真正的空闲。
			// 如果 online=0 但 deltaTotal>0，说明刚好这几秒有人下线/断线，这最后一条日志是有价值的，不能跳过。
			if onlineCount == 0 && deltaTotal == 0 {
				now := time.Now()

				// 1. 重置 QPS 计算的基准时间，防止下次活跃时 QPS 被巨大的 idle 时间稀释
				lastReportTime = now
				s.metrics.lastCollectTime = now
				lastMemSampleTime = now

				// 2. 重置内存基准，防止下次计算 GC 增量时数据异常
				runtime.ReadMemStats(s.metrics.lastMemStats)
				// 重置派生指标
				memAllocMB, memSysMB, allocRateMB = 0, 0, 0
				deltaGC, avgPause, deltaPauseNs = 0, 0, 0
				s.metrics.MemAllocBytes.Store(s.metrics.lastMemStats.Alloc)
				s.metrics.MemSysBytes.Store(s.metrics.lastMemStats.Sys)
				s.metrics.MemNumGC.Store(uint64(s.metrics.lastMemStats.NumGC))

				// 3. 重置 RTT 采样（无锁追踪器自动管理）
				s.metrics.RTTTracker.GetAndResetSamples()

				// 直接跳过本次循环，不打印日志
				continue
			}
			// -----------------------

			now := time.Now()
			duration := now.Sub(lastReportTime).Seconds()
			if duration <= 0 {
				continue
			}
			lastReportTime = now

			// --- C. 计算 QPS (Curr & Global) ---
			// 使用刚才 Swap 出来的值计算
			currTotalQPS := (float64(deltaRecv) + float64(deltaSent)) / duration

			var globalQPS float64
			var activeSeconds float64
			firstTimeNano := s.metrics.FirstPacketTime.Load()
			// 历史总包数
			totalPackets := s.metrics.recvCountTotal.Load() + s.metrics.sentCountTotal.Load()

			if firstTimeNano > 0 {
				activeSeconds = now.Sub(time.Unix(0, firstTimeNano)).Seconds()
				if activeSeconds > 1.0 {
					globalQPS = float64(totalPackets) / activeSeconds
				}
			}

			// --- D. 计算 RTT 延迟 (Curr & Global) ---
			// 1. 取出瞬时样本并计算 (纳秒)
			rawSamples := s.metrics.RTTTracker.GetAndResetSamples()
			rttStats := CalculateStats(rawSamples)

			// 2. 计算全局平均值
			var globalAvgRTT time.Duration
			globalTotalTime := s.metrics.GlobalTotalRTT.Load()
			globalTotalCount := s.metrics.GlobalCountRTT.Load()
			if globalTotalCount > 0 {
				globalAvgRTT = time.Duration(globalTotalTime / globalTotalCount)
			}

			// --- E. GC 与 内存监控 ---
			// 降频采样：仅当达到采样间隔才读取 MemStats 并更新派生指标。
			if now.Sub(lastMemSampleTime) >= memStatsInterval {
				var currentMem runtime.MemStats
				runtime.ReadMemStats(&currentMem)

				// 计算增量 (Delta)
				deltaMemTime := now.Sub(s.metrics.lastCollectTime).Seconds()
				if deltaMemTime <= 0 {
					deltaMemTime = 1
				} // 防御性编程

				// 1. GC 次数
				deltaGC = int(currentMem.NumGC - s.metrics.lastMemStats.NumGC)

				// 2. GC 暂停时间
				deltaPauseNs = int64(currentMem.PauseTotalNs - s.metrics.lastMemStats.PauseTotalNs)
				avgPause = 0
				if deltaGC > 0 {
					avgPause = time.Duration(deltaPauseNs / int64(deltaGC))
				}

				// 3. 内存分配速率
				deltaAllocBytes := int64(currentMem.TotalAlloc - s.metrics.lastMemStats.TotalAlloc)
				allocRateMB = float64(deltaAllocBytes) / 1024 / 1024 / deltaMemTime // MB/s

				// 4. 当前内存（用于展示）
				memAllocMB = float64(currentMem.Alloc) / 1024 / 1024
				memSysMB = float64(currentMem.Sys) / 1024 / 1024
				s.metrics.MemAllocBytes.Store(currentMem.Alloc)
				s.metrics.MemSysBytes.Store(currentMem.Sys)
				s.metrics.MemNumGC.Store(uint64(currentMem.NumGC))

				// 更新快照供下次使用
				*s.metrics.lastMemStats = currentMem
				s.metrics.lastCollectTime = now
				lastMemSampleTime = now
			}

			// --- Bridge to Prometheus ---
			zmetrics.GateOnlineUsers.Set(int64(onlineCount))
			zmetrics.GateRecvQPS.Set(int64(float64(deltaRecv) / duration))
			zmetrics.GateSentQPS.Set(int64(float64(deltaSent) / duration))
			for _, sample := range rawSamples {
				zmetrics.GateRTTAvg.Observe(float64(sample) / 1e6) // ns → ms
			}

			// --- F. 打印日志 ---
			s.GetLogger().Info("[Gate Monitor]",
				// --- QPS 组 ---
				zap.Float64("QPS_Curr", currTotalQPS),
				zap.Float64("QPS_Global", globalQPS),

				// --- RTT 瞬时组 ---
				zap.Int("RTT_Samples", rttStats.Count),
				zap.Duration("RTT_Avg", rttStats.Avg),
				zap.Duration("RTT_P50", rttStats.P50),
				zap.Duration("RTT_P99", rttStats.P99),
				zap.Duration("RTT_Max", rttStats.Max),

				// --- RTT 全局组 ---
				zap.Duration("RTT_Global_Avg", globalAvgRTT),
				zap.Int64("RTT_Total_Count", globalTotalCount),

				// --- 系统资源 (CPU/MEM/GC) ---
				zap.Float64("Mem_Alloc_MB", memAllocMB),
				zap.Float64("Mem_Sys_MB", memSysMB),
				zap.Float64("Alloc_Rate_MB", allocRateMB),

				zap.Int("GC_Count", deltaGC),
				zap.Duration("GC_Pause_Avg", avgPause),
				zap.Float64("GC_Pause_Total_ms", float64(deltaPauseNs)/1e6),

				// --- 基础信息 ---
				zap.Int32("Online", onlineCount),
				zap.Int("Goroutines", runtime.NumGoroutine()),
				zap.Float64("Run_Sec", activeSeconds),
			)
		}
	}
}
