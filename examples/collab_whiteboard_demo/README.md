# collab_whiteboard_demo

`collab_whiteboard_demo` 是一个最小协作白板示例：

- 服务端：`zhenyi` 单进程 Gate + Whiteboard Actor
- 客户端：纯 `HTML + JavaScript`（浏览器 WebSocket）
- 功能：加入房间、实时线段广播、清空白板、重连/新加入快照同步

> 说明：本示例只用于验证 `zhenyi` 协作广播能力，不引入脏标记系统。

## 运行

在仓库根目录执行：

```bash
go run ./examples/collab_whiteboard_demo -conn ws -addr 127.0.0.1:8011
```

浏览器打开：

`http://127.0.0.1:8081/collab_whiteboard_demo/web/`

可开多个标签页或浏览器窗口模拟多人协作。

### 说明

- 示例默认会同时启动静态页面服务（`-web 127.0.0.1:8081`），不依赖 Python。
- 静态服务默认以 `./examples` 作为根目录（`-webRoot` 可改），以便复用公共前端 SDK：`/_shared/web/zhenyi-ws-sdk.js`。
