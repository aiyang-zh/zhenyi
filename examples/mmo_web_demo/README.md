# mmo_web_demo

`mmo_web_demo` 是一个最小 MMO 同步示例：

- 服务端：`zhenyi` 单进程 Gate + MMO Actor
- 客户端：纯 `HTML + JavaScript`（浏览器 WebSocket）
- 功能：登录、进入房间、移动同步、**近战攻击**（伤害 / 冷却 / 距离判定）、阵亡与**延迟重生**、血条与战斗日志
- **AOI**：服务端用 `zaoi` 包（`WorldManager` 挂载按房间划分的 `Zone`，每区 `StaticAoi` 九宫格 + 视距）；`world_snapshot` 与 `combat_event` 只下发给视野内相关客户端（每人一条快照，列表不含远处玩家）

> 说明：为方便站桩体验，本 demo 通过 `zgate.WithNetServerHook` 对底层 `znet.BaseServer` **禁用**空闲读超时（避免无操作约 30 秒后断开）。生产环境建议开启并配合客户端心跳。

## 运行

在仓库根目录执行：

```bash
go run ./examples/mmo_web_demo -conn ws -addr 127.0.0.1:8001
```

浏览器打开：

`http://127.0.0.1:8080/mmo_web_demo/web/`

可开多个标签页模拟多玩家。

### 说明

- 示例默认会同时启动静态页面服务（`-web 127.0.0.1:8080`），不依赖 Python。
- 静态服务默认以 `./examples` 作为根目录（`-webRoot` 可改），以便复用公共前端 SDK：`/_shared/web/zhenyi-ws-sdk.js`。
