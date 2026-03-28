package zgate

import (
	baseziface "github.com/aiyang-zh/zhenyi-base/ziface"
	"github.com/aiyang-zh/zhenyi-base/ztcp"
	"github.com/aiyang-zh/zhenyi/zmetrics"
	"github.com/aiyang-zh/zhenyi/zmonitor"
)

// netServerMetrics 实现 zhenyi-base 层的 IMetrics，用于把连接级指标桥接到 zmetrics。
type netServerMetrics struct{}

func (m *netServerMetrics) ConnInc() {
	zmetrics.ConnActive.Inc()
	zmetrics.ConnAccepted.Inc()
}

func (m *netServerMetrics) ConnDec() {
	zmetrics.ConnActive.Dec()
}

func (m *netServerMetrics) ConnRejectedInc() {
	zmetrics.ConnRejected.Inc()
}

// netChannelMetrics 实现单连接维度的 IChannelMetrics，将字节/错误等统计桥接到 zmetrics。
type netChannelMetrics struct{}

func (m *netChannelMetrics) BytesRecAdd(delta int64) {
	if delta <= 0 {
		return
	}
	zmetrics.BytesRecv.Add(delta)
}

func (m *netChannelMetrics) BytesSentAdd(delta int64) {
	if delta <= 0 {
		return
	}
	zmetrics.BytesSent.Add(delta)
}

func (m *netChannelMetrics) ConnErrorsInc() {
	zmetrics.ConnErrors.Inc()
}

func (m *netChannelMetrics) ConnHeartbeatTimeoutInc() {
	zmetrics.ConnHeartbeatTimeout.Inc()
}

// metricServer 是 BaseServer 的公共子集接口，用于在不暴露具体协议类型的前提下注入指标收集器。
type metricServer interface {
	SetMetrics(m baseziface.IMetrics)
	SetChannelMetrics(m baseziface.IChannelMetrics)
}

// sessionStatsAttachable 与 znet.BaseServer.SetSessionStatsFactory 对齐（由 *ztcp/zkcp/zws.Server 提升）。
type sessionStatsAttachable interface {
	SetSessionStatsFactory(f func() baseziface.ISessionStats)
}

var (
	_serverMetrics  = &netServerMetrics{}
	_channelMetrics = &netChannelMetrics{}
)

// attachNetMetrics 尝试为底层 IServer 注入连接级与单连接级指标、每连接会话统计，以及 TCP reactor 专用桥接。
func attachNetMetrics(s baseziface.IServer) {
	if ms, ok := s.(metricServer); ok {
		ms.SetMetrics(_serverMetrics)
		ms.SetChannelMetrics(_channelMetrics)
	}
	if sa, ok := s.(sessionStatsAttachable); ok {
		sa.SetSessionStatsFactory(func() baseziface.ISessionStats {
			return zmonitor.NewSessionStats()
		})
	}
	if ts, ok := s.(*ztcp.Server); ok {
		ts.SetReactorMetrics(zmetrics.ReactorMetricsBridge())
	}
}
