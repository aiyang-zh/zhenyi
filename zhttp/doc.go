// Package zhttp provides a standard net/http-based IHttpServer implementation.
// Package zhttp 提供基于 net/http 的 IHttpServer 标准实现，handler 固定接收 ziface.IActor。
//
// Recommended: mount HTTP on Gate so long-connection and HTTP share one gateway.
// 推荐：HTTP 挂在 Gate 上，网关统一（长连接 + HTTP 共用同一 Gate）。
// Enable/disable HTTP with SetHTTPAddr:
// 用参数控制是否启 HTTP：
//
//	gate := zgate.NewServer(actorConfig, znet.TCP, "0.0.0.0", 9001)
//	gate.HTTP().GET("/ping", ...)
//	gate.HTTP().Group("/api").POST("/login", loginHandler)
//	gate.SetHTTPAddr(":8080")   // RunServer 时自动起 HTTP，无需再调 HTTP().Run()
//	// 再跑 Gate；若未 SetHTTPAddr 或设为空，则不启 HTTP
//
// Without Gate, you can use NewStdServer + SetActor(actor), then register routes and Run.
// 若不用 Gate，也可直接 NewStdServer + SetActor(actor) 后注册路由并 Run。
package zhttp
