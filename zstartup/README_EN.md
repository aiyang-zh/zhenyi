# zstartup

**Application Startup Orchestration Module**: Currently provides `App` starter, used to create Group, register Actor factories, and run all Actors.

## Module Positioning

- Describes "list of Actors this process should start" with `AppConfig`
- Creates `IServerActor` via `ActorType -> ActorFactory` mapping
- Automatically completes `AddActor` and route registration by `GetMsgList()`

## Core Capabilities

- `NewApp(ctx, cfg AppConfig) *App`
- `(a *App) RegisterActorFactory(actorType, factory)`
- `(a *App) Run() error`

## Minimal Usage

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

## Usage Suggestions

- Each `ActorType` must register factory first, otherwise `Run()` returns initialization error
- Factory return value cannot be `nil`, otherwise initialization fails
- Complete bus/discovery and other global dependency injection before `Run()`

## Related Documentation

- Module API navigation: `../docs/MODULE_API.md`
- Beginner's guide: `../docs/BEGINNER_GUIDE.md`
- Startup checks: `../zcheck/README.md`
