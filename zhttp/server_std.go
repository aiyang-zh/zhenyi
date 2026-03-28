package zhttp

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/aiyang-zh/zhenyi/ziface"
)

func routeKey(method, path string) string { return method + " " + path }

type route struct {
	middlewares []ziface.HttpMiddleware
	handler     ziface.HttpHandlerFunc
}

// StdServer is net/http-based IHttpServer implementation with ziface.IActor-aware handlers.
// StdServer 基于 net/http 的 IHttpServer 实现，handler 固定接收 ziface.IActor。
// Contract: SetActor/Use/route registration should happen before Run and remain immutable after start.
// 约定：SetActor/Use/路由注册仅在 Run 前调用，Run 后不再修改。
type StdServer struct {
	actor       ziface.IActor
	middlewares []ziface.HttpMiddleware
	routes      map[string]*route

	mu  sync.Mutex // Protect concurrent access to routes/middlewares/srv / 保护 routes/middlewares/srv 的并发访问
	srv *http.Server
}

// NewStdServer creates a standard-library based IHttpServer.
// NewStdServer 基于 Go 标准库创建一个 IHttpServer 实现；Run 前需先调用 SetActor(actor)。
func NewStdServer() ziface.IHttpServer {
	return &StdServer{routes: make(map[string]*route)}
}

// SetActor sets the actor instance used when invoking HTTP handlers.
// SetActor 设置用于调用 HTTP handler 的 actor 实例。
func (s *StdServer) SetActor(actor ziface.IActor) { s.actor = actor }

// Use appends one or more HTTP middlewares to the server.
// Use 追加一个或多个 HTTP 中间件到服务器。
func (s *StdServer) Use(middleware ...ziface.HttpMiddleware) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.middlewares = append(s.middlewares, middleware...)
}

// Group creates a sub-router with the given URL prefix.
// Group 创建带有给定 URL 前缀的子路由器。
func (s *StdServer) Group(prefix string) ziface.IHttpServer {
	return &stdGroup{server: s, prefix: strings.TrimSuffix(prefix, "/"), middlewares: nil}
}

// GET registers an HTTP GET route handler.
// GET 注册 HTTP GET 路由处理器。
func (s *StdServer) GET(path string, handler ziface.HttpHandlerFunc) {
	s.addRoute(http.MethodGet, path, nil, handler)
}

// POST registers an HTTP POST route handler.
// POST 注册 HTTP POST 路由处理器。
func (s *StdServer) POST(path string, handler ziface.HttpHandlerFunc) {
	s.addRoute(http.MethodPost, path, nil, handler)
}

// PUT registers an HTTP PUT route handler.
// PUT 注册 HTTP PUT 路由处理器。
func (s *StdServer) PUT(path string, handler ziface.HttpHandlerFunc) {
	s.addRoute(http.MethodPut, path, nil, handler)
}

// DELETE registers an HTTP DELETE route handler.
// DELETE 注册 HTTP DELETE 路由处理器。
func (s *StdServer) DELETE(path string, handler ziface.HttpHandlerFunc) {
	s.addRoute(http.MethodDelete, path, nil, handler)
}

// PATCH registers an HTTP PATCH route handler.
// PATCH 注册 HTTP PATCH 路由处理器。
func (s *StdServer) PATCH(path string, handler ziface.HttpHandlerFunc) {
	s.addRoute(http.MethodPatch, path, nil, handler)
}

// addRoute registers one route into internal routing table.
// addRoute 注册一条路由到内部路由表。
func (s *StdServer) addRoute(method, path string, groupMiddleware []ziface.HttpMiddleware, handler ziface.HttpHandlerFunc) {
	path = normalizePath(path)
	s.mu.Lock()
	defer s.mu.Unlock()
	all := append(append([]ziface.HttpMiddleware(nil), s.middlewares...), groupMiddleware...)
	s.routes[routeKey(method, path)] = &route{middlewares: all, handler: handler}
}

// Run starts the HTTP server and blocks until it exits.
// Run 启动 HTTP 服务并阻塞直到退出。
func (s *StdServer) Run(addr string) error {
	s.mu.Lock()
	handlers := make(map[string]ziface.HttpHandlerFunc, len(s.routes))
	for key, r := range s.routes {
		handlers[key] = s.chain(r.middlewares, r.handler)
	}
	actor := s.actor
	s.mu.Unlock()

	// Aggregate by path, then dispatch by method, and let standard ServeMux match routes.
	// 按 path 聚合，同一 path 下按 method 分发，交给标准库 ServeMux 做路径匹配。
	byPath := make(map[string]map[string]ziface.HttpHandlerFunc)
	for key, h := range handlers {
		method, path := splitRouteKey(key)
		if byPath[path] == nil {
			byPath[path] = make(map[string]ziface.HttpHandlerFunc)
		}
		byPath[path][method] = h
	}

	mux := http.NewServeMux()
	for path, methodHandlers := range byPath {
		p := path
		mh := methodHandlers
		mux.HandleFunc(p, func(w http.ResponseWriter, req *http.Request) {
			handler, ok := mh[req.Method]
			if !ok {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			if err := handler(actor, w, req); err != nil {
				// Avoid exposing internal error details directly to client.
				// 避免将内部错误细节直接暴露给客户端。
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
		})
	}

	// Prevent slowloris by limiting request-header read time.
	// 防止 slowloris：限制读取请求头的超时时间。
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	s.mu.Lock()
	s.srv = srv
	s.mu.Unlock()

	return srv.ListenAndServe()
}

// Shutdown gracefully stops the HTTP server.
// Shutdown 优雅关闭 HTTP 服务。
//
// Note: IHttpServer interface does not include this method; callers should use type assertion.
// 注意：`IHttpServer` 接口不包含该方法，调用方需要通过类型断言使用。
func (s *StdServer) Shutdown(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	s.mu.Lock()
	srv := s.srv
	s.mu.Unlock()
	if srv == nil {
		return nil
	}
	return srv.Shutdown(ctx)
}

// splitRouteKey splits "GET /ping" into method and path.
// splitRouteKey 把 "GET /ping" 拆成 method 和 path。
func splitRouteKey(key string) (method, path string) {
	i := 0
	for i < len(key) && key[i] != ' ' {
		i++
	}
	if i < len(key) {
		return key[:i], key[i+1:]
	}
	return key, "/"
}

func (s *StdServer) chain(middlewares []ziface.HttpMiddleware, final ziface.HttpHandlerFunc) ziface.HttpHandlerFunc {
	f := final
	for i := len(middlewares) - 1; i >= 0; i-- {
		f = middlewares[i](f)
	}
	return f
}

func normalizePath(p string) string {
	if p == "" {
		return "/"
	}
	if p[0] != '/' {
		p = "/" + p
	}
	return p
}

type stdGroup struct {
	server      *StdServer
	prefix      string
	middlewares []ziface.HttpMiddleware
}

// SetActor sets the actor instance used when invoking HTTP handlers.
// SetActor 设置用于调用 HTTP handler 的 actor 实例。
func (g *stdGroup) SetActor(actor ziface.IActor) { g.server.SetActor(actor) }

// Run starts the HTTP server.
// Run 启动 HTTP 服务。
func (g *stdGroup) Run(addr string) error { return g.server.Run(addr) }

// Shutdown gracefully stops the HTTP server.
// Shutdown 优雅关闭 HTTP 服务。
func (g *stdGroup) Shutdown(ctx context.Context) error {
	return g.server.Shutdown(ctx)
}

// Use appends one or more HTTP middlewares into this group.
// Use 追加一个或多个 HTTP 中间件到当前分组。
func (g *stdGroup) Use(middleware ...ziface.HttpMiddleware) {
	g.middlewares = append(g.middlewares, middleware...)
}

// Group creates a nested group with composed prefix.
// Group 创建嵌套分组（组合前缀）。
func (g *stdGroup) Group(prefix string) ziface.IHttpServer {
	prefix = strings.TrimSuffix(prefix, "/")
	if g.prefix != "" {
		prefix = g.prefix + prefix
	}
	return &stdGroup{server: g.server, prefix: prefix, middlewares: g.middlewares}
}

// GET registers an HTTP GET route handler under this group's prefix.
// GET 在当前分组前缀下注册 HTTP GET 路由处理器。
func (g *stdGroup) GET(path string, handler ziface.HttpHandlerFunc) {
	g.server.addRoute(http.MethodGet, g.prefix+normalizePath(path), g.middlewares, handler)
}

// POST registers an HTTP POST route handler under this group's prefix.
// POST 在当前分组前缀下注册 HTTP POST 路由处理器。
func (g *stdGroup) POST(path string, handler ziface.HttpHandlerFunc) {
	g.server.addRoute(http.MethodPost, g.prefix+normalizePath(path), g.middlewares, handler)
}

// PUT registers an HTTP PUT route handler under this group's prefix.
// PUT 在当前分组前缀下注册 HTTP PUT 路由处理器。
func (g *stdGroup) PUT(path string, handler ziface.HttpHandlerFunc) {
	g.server.addRoute(http.MethodPut, g.prefix+normalizePath(path), g.middlewares, handler)
}

// DELETE registers an HTTP DELETE route handler under this group's prefix.
// DELETE 在当前分组前缀下注册 HTTP DELETE 路由处理器。
func (g *stdGroup) DELETE(path string, handler ziface.HttpHandlerFunc) {
	g.server.addRoute(http.MethodDelete, g.prefix+normalizePath(path), g.middlewares, handler)
}

// PATCH registers an HTTP PATCH route handler under this group's prefix.
// PATCH 在当前分组前缀下注册 HTTP PATCH 路由处理器。
func (g *stdGroup) PATCH(path string, handler ziface.HttpHandlerFunc) {
	g.server.addRoute(http.MethodPatch, g.prefix+normalizePath(path), g.middlewares, handler)
}

var _ ziface.IHttpServer = (*StdServer)(nil)
var _ ziface.IHttpServer = (*stdGroup)(nil)
