# zconfig

**配置管理模块**：提供 TOML 配置加载与可回调的手动重载能力。

## 模块定位

- 统一配置文件读取入口，减少业务重复解析逻辑
- 支持 `Loader` 形态，便于挂接重载回调
- 当前重载为手动触发，文件监听由业务自行组合

## 核心 API

- `Load(path, dest)`：一次性加载配置
- `NewLoader(path)`：创建可复用 Loader
- `(*Loader).Load(dest)`：加载到目标对象
- `(*Loader).OnReload(cb)`：注册重载回调
- `(*Loader).Reload()`：触发重载并回调

## 最小用法

```go
type AppConf struct {
    Name string `toml:"name"`
}

var cfg AppConf
if err := zconfig.Load("config.toml", &cfg); err != nil {
    return err
}
```

## 相关文档

- 模块 API 导航：`../docs/MODULE_API.md`
- 新手教程：`../docs/BEGINNER_GUIDE.md`
