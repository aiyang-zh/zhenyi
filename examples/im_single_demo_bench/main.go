package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/http/pprof"
	"runtime"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi-base/znet"
	"github.com/aiyang-zh/zhenyi-base/zserialize"
	"github.com/aiyang-zh/zhenyi/zactor"
	"github.com/aiyang-zh/zhenyi/zcodec"
	"github.com/aiyang-zh/zhenyi/zgate"
	"github.com/aiyang-zh/zhenyi/ziface"
	"github.com/aiyang-zh/zhenyi/zmodel"
	"github.com/aiyang-zh/zhenyi/zmsg"
	"github.com/aiyang-zh/zhenyi/zpyroscope"
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

// fixedBytesMessage is a zero-allocation-ish IMessage adapter for constant payload bytes.
// In framework mode we pre-encode constant reply bytes once and then reuse them.
type fixedBytesMessage struct {
	msgID int32
	data  []byte
}

func (m *fixedBytesMessage) UnmarshalVT([]byte) error { return nil }
func (m *fixedBytesMessage) MarshalVT() ([]byte, error) {
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

// noOpPyroscopeLogger silences pyroscope internal debug/info logs.
type noOpPyroscopeLogger struct{}

func (noOpPyroscopeLogger) Debugf(string, ...interface{}) {}
func (noOpPyroscopeLogger) Infof(string, ...interface{})  {}
func (noOpPyroscopeLogger) Errorf(string, ...interface{}) {}

func main() {
	var (
		codec            = flag.String("codec", "json", "payload codec: json|msgpack")
		benchMode        = flag.String("benchMode", "business", "bench mode: business|framework")
		reactor          = flag.Bool("reactor", false, "enable reactor mode for gate tcp server (tcp + no tls)")
		sharedSendWorker = flag.Bool("sharedSendWorker", false, "enable shared send worker mode for gate server")
		watchdogMs       = flag.Int("watchdogMs", 0, "enable watchdog: dump stack when handler blocked > watchdogMs (0=off)")
		gateAddr         = flag.String("addr", "127.0.0.1:8001", "gate listen addr for this demo")
		pprofAddr        = flag.String("pprofAddr", "", "enable pprof http server on addr (e.g. 127.0.0.1:6060), empty=off")
		pyroscopeAddr    = flag.String("pyroscopeAddr", "", "enable pyroscope continuous profiling, e.g. http://127.0.0.1:4040, empty=off")
		lowLatencyPreset = flag.Bool("lowLatencyPreset", false, "enable low-latency tuning preset (batch/threshold only, no framework code changes)")

		// Send-loop tuning (znet BaseChannel.runSend): expose for tail-latency experiments.
		sendBatchMin         = flag.Int("sendBatchMin", 0, "znet send-loop batcher minBatch (0=default)")
		sendBatchMax         = flag.Int("sendBatchMax", 32, "znet send-loop batcher maxBatch (C-group default)")
		sendBatchTargetMean  = flag.Int("sendBatchTargetMeanMs", 1, "znet send-loop batcher target mean latency in ms (C-group default)")
		sendBackoffFirst     = flag.Int("sendBackoffFirst", 0, "znet send-loop backoff first threshold (0=default)")
		sendBackoffSecond    = flag.Int("sendBackoffSecond", 0, "znet send-loop backoff second threshold (0=default)")
		sendBackoffSleepUs   = flag.Int("sendBackoffSleepUs", 0, "znet send-loop backoff sleep in microseconds (0=default)")
		reactorMaxQueuedMsgs = flag.Int("reactorMaxQueuedMsgs", 0, "znet shared-send per-connection queue soft cap (0=default)")
		reactorFlushBatches  = flag.Int("reactorFlushBatchesPerTurn", 0, "znet shared-send fairness quota: max batches per channel turn (0=default)")
		mutexProfileFraction = flag.Int("mutexProfileFraction", 0, "runtime mutex profile fraction (0=off, 1=all events)")
		blockProfileRate     = flag.Int("blockProfileRate", 0, "runtime block profile rate in ns (<=0=off)")
	)
	flag.Parse()

	selectedCodec := strings.ToLower(*codec)
	if selectedCodec != "json" && selectedCodec != "msgpack" {
		panic(fmt.Sprintf("unsupported codec: %s (expect json|msgpack)", *codec))
	}
	selectedBenchMode := strings.ToLower(*benchMode)
	if selectedBenchMode != "business" && selectedBenchMode != "framework" {
		panic(fmt.Sprintf("unsupported benchMode: %s (expect business|framework)", *benchMode))
	}

	// Framework mode: pre-encode fixed reply bytes to avoid per-message map/json allocations.
	// Framework mode：预编码固定 reply bytes，避免每条消息的 map/json 编解码分配。
	type okResp struct {
		Ok bool `json:"ok" msgpack:"ok"`
	}
	var okReplyBytes []byte
	if selectedCodec == "msgpack" {
		var err error
		okReplyBytes, err = zserialize.MarshalMsgPack(okResp{Ok: true})
		if err != nil {
			panic(err)
		}
	} else {
		var err error
		okReplyBytes, err = zserialize.MarshalJson(okResp{Ok: true})
		if err != nil {
			panic(err)
		}
	}

	replyLoginOK := &fixedBytesMessage{msgID: MsgLoginReq, data: okReplyBytes}
	replyJoinOK := &fixedBytesMessage{msgID: MsgJoinReq, data: okReplyBytes}
	replyLeaveOK := &fixedBytesMessage{msgID: MsgLeaveReq, data: okReplyBytes}
	replySendOK := &fixedBytesMessage{msgID: MsgSendReq, data: okReplyBytes}
	// Initialize default logger so zactor/zgate can clone prefixed loggers.
	// 初始化默认日志器，便于 zactor/zgate 克隆带前缀的 Logger。
	logConfig := zlog.NewDefaultLoggerConfig()
	logConfig.WithOptions(zlog.WithConsole(true))
	zlog.NewDefaultLoggerWithConfig(logConfig)

	if *pprofAddr != "" {
		go func() {
			zlog.CloneDefaultLog("pprof").Info("pprof server started", zap.String("addr", *pprofAddr))
			mux := http.NewServeMux()
			mux.HandleFunc("/debug/pprof/", pprof.Index)
			mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
			mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
			mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
			mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
			srv := &http.Server{
				Addr:              *pprofAddr,
				Handler:           mux,
				ReadHeaderTimeout: 5 * time.Second,
			}
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				zlog.CloneDefaultLog("pprof").Warn("pprof server stopped", zap.Error(err))
			}
		}()
	}

	// Enable runtime profiling sources for pyroscope mutex/block profiles.
	runtime.SetMutexProfileFraction(*mutexProfileFraction)
	runtime.SetBlockProfileRate(*blockProfileRate)
	zlog.CloneDefaultLog("profile").Info("runtime profile rates configured",
		zap.Int("mutexProfileFraction", *mutexProfileFraction),
		zap.Int("blockProfileRate", *blockProfileRate))

	// Apply znet send-loop tuning early (before channels are created).
	if *sendBatchMin > 0 || *sendBatchMax > 0 || *sendBatchTargetMean > 0 || *sendBackoffFirst > 0 || *sendBackoffSecond > 0 || *sendBackoffSleepUs > 0 || *reactorMaxQueuedMsgs > 0 || *reactorFlushBatches > 0 {
		t := znet.SendLoopTuning{
			BatchMin:      *sendBatchMin,
			BatchMax:      *sendBatchMax,
			BackoffFirst:  *sendBackoffFirst,
			BackoffSecond: *sendBackoffSecond,
		}
		if *sendBatchTargetMean > 0 {
			t.BatchTargetMean = time.Duration(*sendBatchTargetMean) * time.Millisecond
		}
		if *sendBackoffSleepUs > 0 {
			t.BackoffSleep = time.Duration(*sendBackoffSleepUs) * time.Microsecond
		}
		if *reactorMaxQueuedMsgs > 0 {
			t.ReactorMaxQueuedMsgs = *reactorMaxQueuedMsgs
		}
		if *reactorFlushBatches > 0 {
			t.ReactorFlushBatchesPerTurn = *reactorFlushBatches
		}
		znet.SetSendLoopTuning(t)
		zlog.CloneDefaultLog("tuning").Info("znet send-loop tuning applied",
			zap.Int("BatchMin", znet.GetSendLoopTuning().BatchMin),
			zap.Int("BatchMax", znet.GetSendLoopTuning().BatchMax),
			zap.Duration("BatchTargetMean", znet.GetSendLoopTuning().BatchTargetMean),
			zap.Int("BackoffFirst", znet.GetSendLoopTuning().BackoffFirst),
			zap.Int("BackoffSecond", znet.GetSendLoopTuning().BackoffSecond),
			zap.Duration("BackoffSleep", znet.GetSendLoopTuning().BackoffSleep),
			zap.Int("ReactorMaxQueuedMsgs", znet.GetSendLoopTuning().ReactorMaxQueuedMsgs),
			zap.Int("ReactorFlushBatchesPerTurn", znet.GetSendLoopTuning().ReactorFlushBatchesPerTurn),
		)
	}

	if *pyroscopeAddr != "" {
		// Enable continuous Pyroscope profiling (CPU/memory/goroutine/mutex/block) for flamegraph bottleneck analysis.
		// Pyroscope 持续采集：CPU/内存/Goroutine/Mutex/Block 等，便于从火焰图直接定位瓶颈。
		profiler, err := zpyroscope.Start(zpyroscope.Config{
			ApplicationName: "im_single_demo.0",
			ServerAddress:   *pyroscopeAddr,
			Tags:            map[string]string{"service": "im_single_demo"},
			Logger:          noOpPyroscopeLogger{},
			// Enable all ProfileTypes supported by v1.2.7 for multi-dimensional framework analysis.
			// v1.2.7 支持的 ProfileType 全量开启（框架优化场景：尽量多维观测）。
			ProfileTypes: []zpyroscope.ProfileType{
				zpyroscope.ProfileCPU,
				zpyroscope.ProfileAllocObjects,
				zpyroscope.ProfileAllocSpace,
				zpyroscope.ProfileInuseObjects,
				zpyroscope.ProfileInuseSpace,
				zpyroscope.ProfileGoroutines,
				zpyroscope.ProfileMutexCount,
				zpyroscope.ProfileMutexDuration,
				zpyroscope.ProfileBlockCount,
				zpyroscope.ProfileBlockDuration,
			},
		})
		if err != nil {
			zlog.CloneDefaultLog("pyroscope").Warn("pyroscope start failed", zap.Error(err), zap.String("addr", *pyroscopeAddr))
		} else {
			zlog.CloneDefaultLog("pyroscope").Info("pyroscope profiler started", zap.String("addr", *pyroscopeAddr))
			defer profiler.Stop()
		}
	}

	// Only tune framework parameters before actors are created (no framework code changes).
	if *lowLatencyPreset {
		def := zmodel.DefaultFrameworkTuning
		zmodel.SetFrameworkTuning(zmodel.FrameworkTuning{
			ActorWorkSizeDefault: def.ActorWorkSizeDefault,
			// Best observed tuning for tail latency (our earlier 8006 run).
			ActorBatchMin: 8,
			ActorBatchMax: 64,
			// Lower target makes the adaptive batcher back off sooner.
			ActorBatchTargetP99: 3 * time.Millisecond,
			SlowLogThreshold:    5 * time.Millisecond,
			SlowBatchThreshold:  10 * time.Millisecond,
			RTTBufferSize:       def.RTTBufferSize,
			RTTMaxSamples:       def.RTTMaxSamples,
		})
		zlog.CloneDefaultLog("preset").Info("lowLatencyPreset enabled",
			zap.Int("ActorBatchMin", 8),
			zap.Int("ActorBatchMax", 64),
			zap.Duration("ActorBatchTargetP99", 3*time.Millisecond),
			zap.Duration("SlowLogThreshold", 5*time.Millisecond),
			zap.Duration("SlowBatchThreshold", 10*time.Millisecond))
	}
	app := zstartup.NewApp(context.Background(), zstartup.AppConfig{
		Process:  0,
		IsSingle: true,
		ConnType: 1,
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

	// Enable runtime handler-block watchdog (debug aid for tail latency).
	if *watchdogMs > 0 {
		if g, ok := app.Group.(*zactor.Group); ok {
			g.EnableWatchdog(time.Duration(*watchdogMs) * time.Millisecond)
		}
	}

	err := app.RegisterActorFactory(ActorTypeGate, func(a *zstartup.App, c zmodel.ActorConfig) ziface.IServerActor {
		s := zgate.NewServer(c, a.ConnType)
		s.SetReactorMode(*reactor)
		s.SetSharedSendWorkerMode(*sharedSendWorker)
		s.GetHandleMgr().RegisterHandle(MsgLoginReq, func(ctx context.Context, msg *zmsg.Message) {
			if selectedBenchMode == "framework" {
				s.SendToClient(msg, replyLoginOK)
				return
			}
			var req struct {
				UserID int64 `json:"userId"`
			}

			if selectedCodec == "msgpack" {
				_ = zserialize.UnmarshalMsgPack(msg.Data, &req)
			} else {
				_ = zserialize.UnmarshalJson(msg.Data, &req)
			}

			payload := map[string]any{
				"ok":        true,
				"type":      "login_ack",
				"sessionId": msg.SessionId,
				"userId":    req.UserID,
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
	err = app.Run()
	if err != nil {
		panic(err)
	}
}
