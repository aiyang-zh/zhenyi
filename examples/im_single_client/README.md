# im_single_client

交互式聊天室客户端示例（单连接，手工输入消息）。与 **`examples/im_single_demo`** 网关联调。

以下命令均在 **zhenyi 模块根目录** 执行（`go run ./examples/...`）。

## 启动

```bash
go run ./examples/im_single_client
```

## 与服务端加密相关的三层配置

改服务端「加密方式」时，客户端按层对齐（彼此独立）：

| 层级 | 含义 | 客户端 |
|------|------|--------|
| 传输层 | 是否走 GM-TLS | 服务端若已 `-gmCert`/`-gmKey`（或双证），客户端加 **`-gmtls`**；否则保持明文 TCP，不要加 `-gmtls` |
| GM-TLS 套件 | ECDHE / ECC 协商 | **`-gmCipherSuite`**：`default`（库默认，优先 ECDHE）\|`ecdhe`\|`ecc`\|`both`，建议与 **`im_single_demo -gmCipherSuite`** 一致 |
| 线协议 payload | SM4-GCM 应用层加密 | **`-payloadEncKey`** 须与服务端 **完全一致**（与 TLS 套件无关） |

证书校验（自签 / 演示）：

- **`-gmInsecure`**：跳过校验（仅本地演示）。
- **`-gmRoot <PEM>`**：信任根；联调 demo 自签证书示例：  
  `-gmRoot examples/im_single_demo/testdata/server.pem`
- **`-gmServerName`**：TLS 校验用的主机名（默认 **`im-single-demo-local`**，与 `gengmtestcert` 生成的证书 CN 一致）。连接 **`127.0.0.1`** 且证书**没有 IP SAN** 时，必须用此名做校验，否则会报 `doesn't contain any IP SANs`。

## 常用参数

```bash
go run ./examples/im_single_client \
  -addr 127.0.0.1:8001 \
  -user 10001 \
  -nick alice \
  -room lobby \
  -msgLogin 1 \
  -msgJoin 2 \
  -msgLeave 3 \
  -msgSend 4
```

- `-addr`：Gate 地址  
- `-user` / `-nick` / `-room`：用户与房间  
- `-msgLogin` / `-msgJoin` / `-msgLeave` / `-msgSend`：与各服务端 handler 约定的消息 ID  

国密与线协议加密：

- `-gmtls`：使用国密 GM-TLS（需 Gate 已配置国密证书）  
- `-gmInsecure`：跳过校验服务端证书（自签/演示）  
- `-gmRoot`：信任根 PEM（与 `-gmInsecure` 二选一更佳）  
- `-gmServerName`：证书主机名校验名 / SNI（默认 `im-single-demo-local`；连 IP 且无 IP SAN 时必与 CN 一致）  
- `-gmCipherSuite`：`default|ecdhe|ecc|both`，与 `im_single_demo` 同义，须与服务端可协商套件有交集  
- `-gmInfo`：连接成功后打印协商的 cipher suite（默认 `true`；不需要时 `-gmInfo=false`）  
- `-payloadEncKey`：线协议 payload SM4-GCM，须与服务端 `-payloadEncKey` 一致  

## 与 im_single_demo 联调示例

**1）明文 TCP（无 GM-TLS）**

```bash
# 终端 1
go run ./examples/im_single_demo -addr 127.0.0.1:8001

# 终端 2
go run ./examples/im_single_client -addr 127.0.0.1:8001
```

**2）GM-TLS + demo 自签证书（校验证书）**

```bash
# 终端 1（证书见 im_single_demo/testdata/，可用 cmd/gengmtestcert 重新生成）
go run ./examples/im_single_demo \
  -addr 127.0.0.1:8001 \
  -gmCert examples/im_single_demo/testdata/server.pem \
  -gmKey examples/im_single_demo/testdata/server.key

# 终端 2
go run ./examples/im_single_client \
  -addr 127.0.0.1:8001 \
  -gmtls \
  -gmRoot examples/im_single_demo/testdata/server.pem
```

（默认 `-gmServerName=im-single-demo-local` 已与 demo 证书 CN 一致，一般无需再写。）

**3）GM-TLS + 自签跳过校验 + 与服务端相同套件 / payload 密钥**

```bash
go run ./examples/im_single_client \
  -addr 127.0.0.1:8001 \
  -gmtls -gmInsecure \
  -gmCipherSuite ecdhe \
  -payloadEncKey "your-shared-secret"
```

（`-gmCipherSuite`、`-payloadEncKey` 须分别与 `im_single_demo` 上对应参数一致。）

## 交互命令

- 输入普通文本：发送聊天消息  
- `/join <room>`：切换并加入房间  
- `/leave`：离开当前房间  
- `/quit`：退出客户端  

## Web 页面（直连 Gate WebSocket）

1. 启动服务端（必须 `-conn ws`，路径为 `/`）：

   ```bash
   go run ./examples/im_single_demo -conn ws -addr 127.0.0.1:8001
   ```

2. 用浏览器打开 `examples/im_single_client/web/index.html`（本地文件即可），WS URL 填 `ws://127.0.0.1:8001/` 后连接。

帧格式与 `ztcp` 客户端相同：12 字节大端头 + JSON body。若服务端启用 **GM-TLS**，浏览器原生 WebSocket 无法完成国密握手，请使用 **本 Go 客户端（`-gmtls`）** 或 TCP 联调。

## 说明

客户端启动后会自动发送：

- `login`：`{"userId":..., "nickname":...}`
- `join`：`{"room":"..."}`
