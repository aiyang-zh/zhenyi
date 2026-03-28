# zhttp

**HTTP Service Module**: Provides `IHttpServer` standard implementation, supports route grouping and middleware, can run independently or mount to Gate.

## Module Positioning

- Provides lightweight HTTP routing capability (GET/POST/PUT/DELETE/PATCH)
- Supports `Group` grouping and `Use` middleware chain
- When integrated with `zgate`, can reuse Actor processing context

## Minimal Usage

### Mounted on Gate (Recommended)

```go
gate := zgate.NewServer(cfg, znet.TCP)
httpSrv := gate.HTTP()
httpSrv.GET("/ping", func(_ ziface.IActor, w http.ResponseWriter, r *http.Request) error {
    w.Write([]byte("pong"))
    return nil
})
httpSrv.Group("/api").POST("/login", loginHandler)
gate.SetHTTPAddr(":8080")  // HTTP automatically starts when RunServer
```

### Standalone

```go
srv := zhttp.NewStdServer()
srv.SetActor(actor)
srv.GET("/health", handler)
srv.Run(":8080")
```

## Usage Suggestions

- Production environment recommends using `Shutdown(ctx)` for graceful shutdown
- When mounted on Gate, register routes before `RunServer`
- For complex business, forward HTTP handler logic to Actor layer to keep processing model consistent

## Related Documentation

- Module API navigation: `../docs/MODULE_API.md`
- Gateway collaboration: `../zgate/README.md`
