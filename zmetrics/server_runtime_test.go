package zmetrics

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/aiyang-zh/zhenyi/zpoolobs"
)

func TestRuntimeCollector_DefaultIntervalAndCollect(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	StartRuntimeCollector(ctx, 0)
	// direct call to cover collectRuntime
	var ms runtime.MemStats
	collectRuntime(&ms, true)
	cancel()
}

func TestServerHandlers(t *testing.T) {
	s := NewServer("127.0.0.1:0")
	s.SetReady()

	rr := httptest.NewRecorder()
	s.handleReady(rr, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "ready") {
		t.Fatalf("ready: code=%d body=%s", rr.Code, rr.Body.String())
	}

	s.SetDraining()
	rr = httptest.NewRecorder()
	s.handleReady(rr, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("draining ready code=%d", rr.Code)
	}

	rr = httptest.NewRecorder()
	s.handleHealth(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rr.Code != http.StatusServiceUnavailable || !strings.Contains(rr.Body.String(), "draining") {
		t.Fatalf("health draining: code=%d body=%s", rr.Code, rr.Body.String())
	}

	// unhealthy check
	s.status.Store(int32(StatusReady))
	s.RegisterHealthCheck("db", func() error { return errors.New("down") })
	rr = httptest.NewRecorder()
	s.handleHealth(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rr.Code != http.StatusServiceUnavailable || !strings.Contains(rr.Body.String(), "unhealthy") {
		t.Fatalf("health unhealthy: code=%d body=%s", rr.Code, rr.Body.String())
	}

	// ok — health 聚合所有检查项；需先修复 db，再增加另一项通过检查
	s.RegisterHealthCheck("db", func() error { return nil })
	s.RegisterHealthCheck("ok", func() error { return nil })
	rr = httptest.NewRecorder()
	s.handleHealth(rr, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "ok") {
		t.Fatalf("health ok: code=%d body=%s", rr.Code, rr.Body.String())
	}

	// metrics handler
	rr = httptest.NewRecorder()
	s.handleMetrics(rr, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rr.Header().Get("Content-Type") == "" || rr.Body.Len() == 0 {
		t.Fatalf("metrics response missing")
	}
}

func TestEnableAndShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	t.Cleanup(func() { zpoolobs.SetObserver(nil) })
	s := Enable(ctx, "127.0.0.1:0")
	if s == nil {
		t.Fatalf("expected server")
	}
	shutCtx, c2 := context.WithTimeout(context.Background(), time.Second)
	defer c2()
	if err := s.Shutdown(shutCtx); err != nil {
		t.Fatalf("shutdown err: %v", err)
	}
}
