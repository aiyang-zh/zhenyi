# im_single_demo

单机聊天室 **服务端示例**：由 **Gate**（长连接接入、可选 TLS/GM-TLS、消息路由）与 **IM**（房间与聊天业务）组成，便于理解 **按 `msgId` 路由**、**房间广播**、以及可选的**国密传输**与**消息摘要**用法。

> 说明：zhenyi 的 Gate 能配置 **标准 TLS（RSA/ECDSA）** 与 **国密 GM-TLS（SM2/SM3/SM4）**。本示例主要演示 GM-TLS 与 payload 加密相关参数；标准 TLS 可参考 `zgate` 文档与 API（如 `SetStandardTLS`）。

## 运行环境

在已获取的 **zhenyi 源码树**中，进入 **zhenyi** 目录后执行（以下命令均在该目录下运行）：

```bash
go run ./examples/im_single_demo
```

## 示例包含什么

| 内容 | 说明 |
|------|------|
| `main.go` | Gate / IM 逻辑；聊天广播（`msgId=4`）对 JSON 正文计算 **SM3 摘要**，放入字段 **`sign`** 下发 |
| `gencert/` | 生成本地 SM2 自签证书（单证或双证），用于体验 **GM-TLS** |
| `certs/` | 证书默认输出位置（也可用 `-out` 指定）；**测试证书与私钥不得用于生产环境** |

## 启动与默认参数

```bash
go run ./examples/im_single_demo
```

默认监听 **`127.0.0.1:8001`**，传输为 **TCP**。若使用 **Web 示例页面**，请使用 **WebSocket**：

```bash
go run ./examples/im_single_demo -conn ws -addr 127.0.0.1:8001
```

配套命令行客户端（与示例配套）：

```bash
go run ./examples/im_single_client -addr 127.0.0.1:8001
```

连接成功后依次完成登录、进房，再输入文字发送聊天。支持命令：`/join <房间>`、`/leave`、`/quit`。更多客户端参数见 **`examples/im_single_client/README.md`**。

## 能力说明

### 消息路由

客户端上行消息通过 **`msgId`** 分发至对应处理逻辑（本示例中登录由 Gate 处理，聊天等由 IM 处理）。房间内推送类消息常见 **`SeqId=0`**，请用 JSON 中的 **`type`** 区分业务类型。

### 聊天广播与 SM3 摘要（`msgId=4`）

服务端先组装 **`chat_broadcast`** 所需字段（**不含** `sign`），对 **`json.Marshal` 得到的字节**做 **SM3** 摘要，将 **十六进制字符串**写入 **`sign`** 后下发。

配套 **`im_single_client`** 在收到后去掉 `sign`、按相同规则重算摘要并比对，终端会提示校验是否通过。该机制用于**演示**「内容摘要与一致性校验」思路；**不等同于**基于数字证书的抗抵赖签名。

### 可选：线协议 payload 加密

设置 **`payloadEncKey`** 时，服务端对 **帧头之后的业务体** 使用 **国密 SM4-GCM**（密钥由 SM3 派生），与是否启用 **GM-TLS** 无关；客户端需使用**相同**口令。

### 可选：国密 GM-TLS

在 Gate 上启用 **GM-TLS** 时，传输层为国密套件（记录层 **SM4** 等）。自签证书生成与参数说明见 **`certs/README.md`**。使用 `im_single_client -gmtls` 时，默认会打印协商到的套件（可用 `-gmInfo=false` 关闭）。

---

## 快速体验

### 明文 TCP（不启用 TLS）

服务端与客户端均**不要**传入证书相关参数；客户端**不要**加 `-gmtls`。

### 国密 GM-TLS（自签证书）

**单证书**（一对 `sm2.crt` / `sm2.key`）：

```bash
go run ./examples/im_single_demo/gencert
```

```bash
CERT=$(pwd)/examples/im_single_demo/certs/sm2.crt
KEY=$(pwd)/examples/im_single_demo/certs/sm2.key
go run ./examples/im_single_demo -conn tcp -addr 127.0.0.1:8001 -gmCert "$CERT" -gmKey "$KEY"
```

```bash
go run ./examples/im_single_client -gmtls -gmInsecure -addr 127.0.0.1:8001
```

**双证书**（`gencert -dual` 生成四套文件）：

```bash
go run ./examples/im_single_demo/gencert -dual
D=$(pwd)/examples/im_single_demo/certs
go run ./examples/im_single_demo -conn tcp -addr 127.0.0.1:8001 \
  -gmSignCert "$D/sm2_sign.crt" -gmSignKey "$D/sm2_sign.key" \
  -gmEncCert "$D/sm2_enc.crt" -gmEncKey "$D/sm2_enc.key"
```

客户端命令同上。**单证模式与双证模式请勿混用参数。**

### 线协议 SM4-GCM（与 TLS 可同时使用）

```bash
go run ./examples/im_single_demo -addr 127.0.0.1:8001 -payloadEncKey '请与服务端约定一致'
go run ./examples/im_single_client -addr 127.0.0.1:8001 -payloadEncKey '请与服务端约定一致'
```

---

## 服务端常用参数

| 参数 | 含义 |
|------|------|
| `-addr` | 监听地址（默认 `127.0.0.1:8001`） |
| `-conn` | `tcp`（默认）或 `ws`（配合 `im_single_client/web` 等 WebSocket 客户端） |
| `-reactor` | 开启 TCP reactor 模式（仅 macOS/Linux；且 `-conn=tcp` 且未启用 TLS/GM-TLS 时生效） |
| `-sharedSendWorker` | 开启共享发送 worker 模式（兼容非 reactor / reactor；默认关闭） |
| `-gmCert` / `-gmKey` | 国密 **单证书** PEM，启用 Gate **GM-TLS** |
| `-gmSignCert` 等四个参数 | 国密 **双证书**，须 **同时** 指定 |
| `-payloadEncKey` | 线协议 payload **SM4-GCM** 口令（须与客户端一致） |

---

## 下行消息约定（`msgId` 1–4）

| msgId | 含义 |
|------|------|
| 1 | 登录 / `login_ack` |
| 2 | 进房 / `join_ack`、`room_notify`（进房） |
| 3 | 离房 / `leave_ack`、`room_notify`（离房） |
| 4 | 聊天 / **`chat_broadcast`**（含 `fromSessionId`、`nickname`、`text`、**`sign`**（SM3 摘要 hex）） |

除点对点 `*_ack` 外，房间内其他成员可能收到推送（常见 `SeqId=0`），请以 **`type`** 区分。

---

## 适用场景说明

本示例用于：

- 展示最小可运行的聊天室服务端形态；
- 说明 Gate 与业务逻辑拆分、按 `msgId` 路由与广播下发；
- 演示可选的 **GM-TLS**、**payload 国密加密**、以及广播 JSON 上的 **SM3 摘要字段**。

## 其它客户端

- 交互式：`go run ./examples/im_single_client`（见该目录说明）
- 并发压测：`go run ./examples/im_multi_client_load`
