package ziface

import "net/http"

// HttpHandlerFunc handles an HTTP request with explicit Actor context.
// HttpHandlerFunc 处理函数，actor 为 IActor（如 Gate 的 Actor），显式传入便于发消息、选 Actor。
type HttpHandlerFunc func(actor IActor, w http.ResponseWriter, r *http.Request) error

// HttpMiddleware wraps HttpHandlerFunc while preserving Actor context.
// HttpMiddleware 中间件，透传 actor。
type HttpMiddleware func(next HttpHandlerFunc) HttpHandlerFunc

// IHttpServer defines HTTP service and route registration capabilities.
// IHttpServer 定义 HTTP 服务与路由注册能力。
// Group returns a grouped server view; SetActor should be called before Run, and actor is passed into every handler.
// Group 返回分组视图；Run 前需 SetActor，每次请求都会把 actor 传入 handler。
type IHttpServer interface {
	SetActor(actor IActor)
	Use(middleware ...HttpMiddleware)
	Group(prefix string) IHttpServer
	GET(path string, handler HttpHandlerFunc)
	POST(path string, handler HttpHandlerFunc)
	PUT(path string, handler HttpHandlerFunc)
	DELETE(path string, handler HttpHandlerFunc)
	PATCH(path string, handler HttpHandlerFunc)
	Run(addr string) error
}
