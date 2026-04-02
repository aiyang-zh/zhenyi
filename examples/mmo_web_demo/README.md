# mmo_web_demo

`mmo_web_demo` 是一个最小 MMO 同步示例：

- 服务端：`zhenyi` 单进程 Gate + MMO Actor
- 客户端：纯 `HTML + JavaScript`（浏览器 WebSocket）
- 功能：登录、进入房间、移动同步、**近战攻击**（伤害 / 冷却 / 距离判定）、阵亡与**延迟重生**、血条与战斗日志
- **AOI**：服务端用 `zaoi` 包（`WorldManager` 挂载按房间划分的 `Zone`，每区 `StaticAoi` 九宫格 + 视距）；`world_snapshot` 与 `combat_event` 只下发给视野内相关客户端（每人一条快照，列表不含远处玩家）

## 运行

在仓库根目录执行：

```bash
go run ./examples/mmo_web_demo -conn ws -addr 127.0.0.1:8001
```

再启动一个静态文件服务（任选其一）：

```bash
python3 -m http.server 8080 -d ./examples/mmo_web_demo/web
```

浏览器打开：

`http://127.0.0.1:8080/`

可开多个标签页模拟多玩家。
