# zstartup

**应用启动编排模块**：当前提供 `App` 启动器，用于创建 Group、注册 Actor 工厂并运行全部 Actor。

## 模块定位

- 以 `AppConfig` 描述“本进程要启动的 Actor 列表”
- 通过 `ActorType -> ActorFactory` 映射创建 `IServerActor`
- 自动完成 `AddActor` 与按 `GetMsgList()` 注册路由

## 核心能力

- `NewApp(ctx, cfg AppConfig) *App`
- `(a *App) RegisterActorFactory(actorType, factory)`
- `(a *App) Run() error`

## 最小用法

```go
app := zstartup.NewApp(ctx, zstartup.AppConfig{
    Process:  1,
    IsSingle: true,
    ConnType: znet.TCP,
    Actors:   actors,
})

if err := app.RegisterActorFactory(actorType, factory); err != nil {
    return err
}
return app.Run()
```

## 使用建议

- 每个 `ActorType` 必须先注册工厂，否则 `Run()` 会返回初始化错误
- 工厂返回值不能为 `nil`，否则初始化失败
- 在 `Run()` 前先完成总线/发现等全局依赖注入

## 相关文档

- 模块 API 导航：`../docs/MODULE_API.md`
- 新手教程：`../docs/BEGINNER_GUIDE.md`
- 启动检查：`../zcheck/README.md`
