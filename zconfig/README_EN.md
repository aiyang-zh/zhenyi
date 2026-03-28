# zconfig

**Configuration Management Module**: Provides TOML configuration loading and callback-enabled manual reload capability.

## Module Positioning

- Unified configuration file reading entry, reducing business duplicate parsing logic
- Supports `Loader` form, convenient for attaching reload callbacks
- Current reload is manually triggered; file watching is combined by business

## Core API

- `Load(path, dest)`: One-time config loading
- `NewLoader(path)`: Create reusable Loader
- `(*Loader).Load(dest)`: Load to target object
- `(*Loader).OnReload(cb)`: Register reload callback
- `(*Loader).Reload()`: Trigger reload and callback

## Minimal Usage

```go
type AppConf struct {
    Name string `toml:"name"`
}

var cfg AppConf
if err := zconfig.Load("config.toml", &cfg); err != nil {
    return err
}
```

## Related Documentation

- Module API navigation: `../docs/MODULE_API.md`
- Beginner's guide: `../docs/BEGINNER_GUIDE.md`
