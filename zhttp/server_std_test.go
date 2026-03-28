package zhttp

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aiyang-zh/zhenyi/ziface"
)

func TestNormalizePath(t *testing.T) {
	cases := map[string]string{
		"":      "/",
		"/":     "/",
		"ping":  "/ping",
		"/ping": "/ping",
	}
	for in, want := range cases {
		if got := normalizePath(in); got != want {
			t.Fatalf("normalizePath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSplitRouteKey(t *testing.T) {
	method, path := splitRouteKey("GET /ping")
	if method != "GET" || path != "/ping" {
		t.Fatalf("splitRouteKey returned %q %q", method, path)
	}
	method, path = splitRouteKey("WEIRD")
	if method != "WEIRD" || path != "/" {
		t.Fatalf("splitRouteKey fallback returned %q %q", method, path)
	}
}

func TestMiddlewareChainOrder(t *testing.T) {
	s := NewStdServer().(*StdServer)
	var seq []string

	mw := func(tag string) ziface.HttpMiddleware {
		return func(next ziface.HttpHandlerFunc) ziface.HttpHandlerFunc {
			return func(actor ziface.IActor, w http.ResponseWriter, r *http.Request) error {
				seq = append(seq, "pre:"+tag)
				err := next(actor, w, r)
				seq = append(seq, "post:"+tag)
				return err
			}
		}
	}

	s.Use(mw("A"), mw("B"))
	s.GET("/ping", func(_ ziface.IActor, _ http.ResponseWriter, _ *http.Request) error {
		seq = append(seq, "handler")
		return nil
	})

	key := routeKey(http.MethodGet, "/ping")
	r := s.routes[key]
	if r == nil {
		t.Fatalf("route not registered: %s", key)
	}

	h := s.chain(r.middlewares, r.handler)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	if err := h(nil, rr, req); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	want := []string{"pre:A", "pre:B", "handler", "post:B", "post:A"}
	if len(seq) != len(want) {
		t.Fatalf("seq len=%d want=%d seq=%v", len(seq), len(want), seq)
	}
	for i := range want {
		if seq[i] != want[i] {
			t.Fatalf("seq[%d]=%q want=%q seq=%v", i, seq[i], want[i], seq)
		}
	}
}

func BenchmarkChainTwoMiddlewares(b *testing.B) {
	s := NewStdServer().(*StdServer)
	mw := func(next ziface.HttpHandlerFunc) ziface.HttpHandlerFunc {
		return func(actor ziface.IActor, w http.ResponseWriter, r *http.Request) error {
			return next(actor, w, r)
		}
	}
	final := func(_ ziface.IActor, _ http.ResponseWriter, _ *http.Request) error { return nil }
	h := s.chain([]ziface.HttpMiddleware{mw, mw}, final)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = h(nil, rr, req)
	}
}
