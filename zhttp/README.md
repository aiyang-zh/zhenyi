# zhttp

**HTTP 服务模块**：提供 `IHttpServer` 标准实现，支持路由分组与中间件，可独立运行或挂载到 Gate。

## 模块定位

- 提供轻量 HTTP 路由能力（GET/POST/PUT/DELETE/PATCH）
- 支持 `Group` 分组与 `Use` 中间件链
- 与 `zgate` 集成时可复用 Actor 处理上下文

## 最小用法

### 挂在 Gate 上（推荐）

```go
gate := zgate.NewServer(cfg, znet.TCP)
httpSrv := gate.HTTP()
httpSrv.GET("/ping", func(_ ziface.IActor, w http.ResponseWriter, r *http.Request) error {
    w.Write([]byte("pong"))
    return nil
})
httpSrv.Group("/api").POST("/login", loginHandler)
gate.SetHTTPAddr(":8080")  // RunServer 时自动起 HTTP
```

### 独立使用

```go
srv := zhttp.NewStdServer()
srv.SetActor(actor)
srv.GET("/health", handler)
srv.Run(":8080")
```

## 使用建议

- 生产环境建议使用 `Shutdown(ctx)` 做优雅关闭
- 挂在 Gate 上时，先注册路由再 `RunServer`
- 复杂业务建议将 HTTP handler 逻辑转发到 Actor 层，保持处理模型一致

## 相关文档

- 模块 API 导航：`../docs/MODULE_API.md`
- 网关协作：`../zgate/README.md`
