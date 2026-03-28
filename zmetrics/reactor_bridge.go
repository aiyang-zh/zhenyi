package zmetrics

import "github.com/aiyang-zh/zhenyi-base/zreactor"

// ReactorMetricsBridge bridges zreactor.Metrics callbacks into zmetrics (for Linux reactor/epoll path).
// ReactorMetricsBridge 将 zreactor.Metrics 回调桥接到 zmetrics（供 Linux reactor / epoll 路径使用）。
//
// Note: connection counters are already maintained by znet.BaseServer.AddChannel/RemoveChannel
// 注意：连接数（ConnInc/ConnDec/ConnAccepted）已由 znet.BaseServer.AddChannel/RemoveChannel
// 配合 IMetrics 维护，此处不再设置 OnAccept/OnClose，避免与 attachNetMetrics 重复计数。
// 读字节由 BaseChannel.WriteToReadBuffer 内 BytesRecAdd 维护，此处不设置 OnReadBytes。
//
// This bridge complements scenarios not always covered by channel parse path (e.g., syscall read errors, accept-loop errors).
// 本桥接补齐 syscall 读失败、Accept 循环错误等 channel 解析路径未必覆盖的场景。
func ReactorMetricsBridge() *zreactor.Metrics {
	return &zreactor.Metrics{
		OnReadErrWithKind: func(_ int, _ error, _ zreactor.ReadErrKind) {
			ConnErrors.Inc()
		},
		OnAcceptErr: func(_ error) {
			ConnErrors.Inc()
		},
	}
}
