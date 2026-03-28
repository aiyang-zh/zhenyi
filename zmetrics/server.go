package zmetrics

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/aiyang-zh/zhenyi-base/zcoll"
	"github.com/aiyang-zh/zhenyi-base/zlog"
	"github.com/aiyang-zh/zhenyi/zmonitor"
	"github.com/aiyang-zh/zhenyi/zpoolobs"
	"go.uber.org/zap"
)

// HealthStatus represents health state.
// HealthStatus 健康状态。
type HealthStatus int32

const (
	StatusStarting HealthStatus = 0
	StatusReady    HealthStatus = 1
	StatusDraining HealthStatus = 2
)

// Server hosts metrics and health-check HTTP endpoints.
// Server 指标与健康检查 HTTP 服务。
type Server struct {
	addr     string
	registry *Registry
	server   *http.Server
	status   atomic.Int32
	checks   *zcoll.SyncMap[string, func() error] // name -> func() error
}

// NewServer creates a metrics/health HTTP server instance bound to global registry.
// NewServer 创建指标/健康检查 HTTP 服务实例（绑定全局 Registry）。
func NewServer(addr string) *Server {
	s := &Server{
		addr:     addr,
		registry: globalRegistry,
		checks:   zcoll.NewSyncMap[string, func() error](),
	}
	s.status.Store(int32(StatusStarting))
	return s
}

// EnableOptions configures EnableWithOptions behavior.
// EnableOptions 配置 EnableWithOptions 的启动行为。
type EnableOptions struct {
	// RuntimeCollectorEnabled controls whether Go runtime metrics collector is started.
	// RuntimeCollectorEnabled 是否启动 Go runtime 指标采集器。
	// 默认 true；若关闭，则仍可使用 /metrics 暴露框架自身指标（但不包含 Go runtime 指标的定时采集）。
	RuntimeCollectorEnabled bool

	// RuntimeCollectorInterval is runtime metric collection interval (<=0 uses default 5s).
	// RuntimeCollectorInterval runtime 指标采集间隔。<=0 时使用默认值（5s）。
	// 仅当 RuntimeCollectorEnabled=true 时生效。
	RuntimeCollectorInterval time.Duration

	// InstallPoolObserver installs zmetrics.GlobalPoolObserver when true and global observer is nil.
	// InstallPoolObserver 为 true 且 zpoolobs 全局 observer 仍为 nil 时，安装 zmetrics.GlobalPoolObserver()，
	// 使 zhenyi_zpool_* 与 /metrics 同源。若业务已 SetObserver，则不会覆盖。
	InstallPoolObserver bool

	// MonitorManager, when non-nil, is registered to export zhenyi_monitor_snapshot in /metrics.
	// MonitorManager 非 nil 时，在启动时注册，供 /metrics 末尾 zhenyi_monitor_snapshot 导出。
	MonitorManager *zmonitor.Manager
}

// defaultEnableOptions returns default EnableOptions.
// defaultEnableOptions 返回默认 EnableOptions。
func defaultEnableOptions() EnableOptions {
	return EnableOptions{
		RuntimeCollectorEnabled:  true,
		RuntimeCollectorInterval: 30 * time.Second,
		InstallPoolObserver:      true,
	}
}

// Enable starts Prometheus metrics service in non-blocking mode.
// Enable 一键启动 Prometheus 指标服务（非阻塞）。
//
// Usage (one-line integration in business main.go):
// 用法（业务 main.go 中一行接入）：
//
//	srv := metrics.Enable(ctx, ":9090")   // 启动后访问 http://host:9090/metrics
//	defer srv.Shutdown(context.Background())
//
// Prometheus 配置（prometheus.yml）：
//
//	scrape_configs:
//	  - job_name: 'zhenyi'
//	    scrape_interval: 5s
//	    static_configs:
//	      - targets: ['<host>:9090']
//
// Endpoints:
// 端点说明：
//   - GET /metrics  — Prometheus 标准 text exposition format（zhenyi_* / go_* 预注册指标 + per-handler + 对象池 + zmonitor 快照，详见 docs/monitoring.md）
//   - GET /healthz  — 存活探针（K8s liveness probe）
//   - GET /readyz   — 就绪探针（K8s readiness probe）
func Enable(ctx context.Context, addr string) *Server {
	return EnableWithOptions(ctx, addr, defaultEnableOptions())
}

// EnableWithOptions 一键启动 Prometheus 指标服务（可配置是否启用 runtime collector）。
func EnableWithOptions(ctx context.Context, addr string, opt EnableOptions) *Server {
	if opt.InstallPoolObserver && zpoolobs.GetObserver() == nil {
		zpoolobs.SetObserver(GlobalPoolObserver())
	}
	if opt.RuntimeCollectorEnabled {
		interval := opt.RuntimeCollectorInterval
		if interval <= 0 {
			interval = 5 * time.Second
		}
		StartRuntimeCollector(ctx, interval)
	}
	EnsureActorPanicHook()
	srv := NewServer(addr)
	if opt.MonitorManager != nil {
		srv.SetMonitorManager(opt.MonitorManager)
	}
	_ = srv.Start(ctx)
	srv.SetReady()
	return srv
}

// SetReady marks server as ready (readiness probe will return ready).
// SetReady 标记服务为 ready（就绪探针返回 ready）。
func (s *Server) SetReady() { s.status.Store(int32(StatusReady)) }

// SetDraining marks server as draining (health/ready probes may return draining).
// SetDraining 标记服务为 draining（探针可能返回 draining）。
func (s *Server) SetDraining() { s.status.Store(int32(StatusDraining)) }

// RegisterHealthCheck registers health check item (nil means healthy).
// RegisterHealthCheck 注册健康检查项（返回 nil 表示健康）。
func (s *Server) RegisterHealthCheck(name string, check func() error) {
	s.checks.Store(name, check)
}

// SetMonitorManager sets zmonitor.Manager used by zhenyi_monitor_snapshot export in /metrics.
// SetMonitorManager 设置 zmonitor.Manager，供 /metrics 中 zhenyi_monitor_snapshot 导出；可在 Start 之前或之后调用。
func (s *Server) SetMonitorManager(m *zmonitor.Manager) {
	RegisterMonitorManager(m)
}

// Start launches HTTP service asynchronously.
// Start 非阻塞启动 HTTP 服务。
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", s.handleMetrics)
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/readyz", s.handleReady)

	s.server = &http.Server{
		Addr:              s.addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			zlog.Error("metrics server listen error", zap.Error(err), zap.String("addr", s.addr))
		}
	}()

	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = s.server.Shutdown(shutCtx)
	}()

	return nil
}

// Shutdown gracefully stops HTTP server.
// Shutdown 优雅关闭 HTTP 服务。
func (s *Server) Shutdown(ctx context.Context) error {
	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}

func (s *Server) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	var b strings.Builder
	b.Grow(4096)
	s.registry.WritePrometheus(&b)
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = w.Write([]byte(b.String()))
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	status := HealthStatus(s.status.Load())
	if status == StatusDraining {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"status":"draining"}`))
		return
	}

	var errs []string
	s.checks.Range(func(name string, check func() error) bool {
		if err := check(); err != nil {
			errs = append(errs, name+": "+err.Error())
		}
		return true
	})

	if len(errs) > 0 {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "unhealthy",
			"errors": errs,
		})
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleReady(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	status := HealthStatus(s.status.Load())
	switch status {
	case StatusReady:
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	case StatusStarting:
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"status":"starting"}`))
	default:
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"status":"draining"}`))
	}
}
