// im_single_client：与 examples/im_single_demo 联调的示例客户端。
//
// 与服务端「加密」相关的三类配置（改服务端时客户端按层对齐）：
//  1. 传输层：是否走 GM-TLS —— 服务端启 -gmCert/-gmKey 时，客户端加 -gmtls；否则明文 TCP。
//  2. GM-TLS 套件 —— 服务端 -gmCipherSuite 若收窄为仅 ecdhe / 仅 ecc，客户端应使用相同 -gmCipherSuite（默认 default 一般与库默认服务端仍有交集，显式成对更稳）。
//  3. 线协议 payload —— 服务端 -payloadEncKey 非空时，客户端 -payloadEncKey 必须与之一致（SM4-GCM，与 TLS 记录层独立）。
//
// 自签证书：-gmInsecure，或 -gmRoot 指向 examples/im_single_demo/testdata/server.pem；
// 连 127.0.0.1 且校验证书时请加 -gmServerName（与证书 CN 一致，demo 默认为 im-single-demo-local）。
package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync/atomic"

	"github.com/aiyang-zh/zhenyi-base/zencrypt"
	"github.com/aiyang-zh/zhenyi-base/zgmtls"
	"github.com/aiyang-zh/zhenyi-base/ziface"
	"github.com/aiyang-zh/zhenyi-base/znet"
	"github.com/aiyang-zh/zhenyi-base/ztcp"
)

func main() {
	var (
		addr     = flag.String("addr", "127.0.0.1:8001", "gate addr")
		userID   = flag.Int64("user", 10001, "user id")
		nickname = flag.String("nick", "alice", "nickname")
		room     = flag.String("room", "lobby", "room")

		useGMTLS      = flag.Bool("gmtls", false, "使用国密 GM-TLS（服务端需已 -gmCert/-gmKey 等）")
		gmInsecure    = flag.Bool("gmInsecure", false, "国密：跳过校验服务端证书（自签/演示；生产请配 -gmRoot）")
		gmRoot        = flag.String("gmRoot", "", "国密：信任根 PEM 路径（与 gmInsecure 二选一）。联调 demo 自签：-gmRoot examples/im_single_demo/testdata/server.pem（在 zhenyi 根目录执行时）")
		gmServerName  = flag.String("gmServerName", "im-single-demo-local", "国密：TLS ServerName / 证书主机名校验名。连 127.0.0.1 且证书无 IP SAN 时必须与 CN 一致；demo 证书 CN 即此值。仅 -gmInsecure 时可设为空")
		gmCipherSuite = flag.String("gmCipherSuite", "default", "国密 ClientHello 套件（须与服务端可协商集合有交集；与 im_single_demo 同义）：default|ecdhe|ecc|both")
		gmInfo        = flag.Bool("gmInfo", true, "国密连接成功后打印协商的 cipher suite（含 SM4/SM3）")
		payloadEncKey = flag.String("payloadEncKey", "", "线协议 payload 国密 SM4-GCM；须与服务端 -payloadEncKey 完全一致（与 TLS 套件独立）")

		msgLogin = flag.Int("msgLogin", 1, "login request msg id")
		msgJoin  = flag.Int("msgJoin", 2, "join room request msg id")
		msgLeave = flag.Int("msgLeave", 3, "leave room request msg id")
		msgSend  = flag.Int("msgSend", 4, "send room message request msg id")
	)
	flag.Parse()

	var client ziface.IClient
	if *useGMTLS {
		tlsCfg := znet.NewClientTLSConfig()
		applyClientGMTLSCipherSuite(tlsCfg, *gmCipherSuite)
		if *gmInsecure {
			tlsCfg.GMConfig.SetInsecureSkipVerify(true)
		} else if sn := strings.TrimSpace(*gmServerName); sn != "" {
			tlsCfg.GMConfig.SetServerName(sn)
		}
		if *gmRoot != "" {
			b, err := os.ReadFile(*gmRoot)
			if err != nil {
				panic(err)
			}
			if err := tlsCfg.GMConfig.SetRootCAsPEM(b); err != nil {
				panic(err)
			}
		}
		conn, err := znet.DialTLS("tcp", *addr, tlsCfg)
		if err != nil {
			panic(err)
		}
		if *gmInfo {
			if gc, ok := conn.(*gmtls.Conn); ok {
				st := gc.ConnectionState()
				fmt.Printf("[gm-tls] cipher suite: %s (0x%04x) — 记录层 SM4，MAC/PRF SM3\n", gmCipherSuiteName(st.CipherSuite), st.CipherSuite)
			}
		}
		tc := &ztcp.TClient{BaseClient: znet.NewBaseClient(znet.WithAsyncMode())}
		tc.SetConn(conn)
		client = tc
	} else {
		var err error
		client, err = ztcp.NewClient(*addr, znet.WithAsyncMode())
		if err != nil {
			panic(err)
		}
	}
	if k := strings.TrimSpace(*payloadEncKey); k != "" {
		client.SetEncrypt(zencrypt.NewSM4GcmEncrypt(k))
	}
	defer client.Close()

	var seq atomic.Uint32
	send := func(msgID int32, payload any) {
		b, err := json.Marshal(payload)
		if err != nil {
			fmt.Printf("marshal payload failed: %v\n", err)
			return
		}
		m := znet.GetNetMessage()
		defer m.Release()
		m.MsgId = msgID
		m.SeqId = seq.Add(1)
		m.SetDataCopy(b)
		client.SendMsg(m)
	}

	client.SetReadCall(func(w ziface.IWireMessage) {
		raw := w.GetMessageData()
		if note := verifyChatBroadcastSM3(raw); note != "" {
			fmt.Println(note)
		}
		fmt.Printf("[recv] msgId=%d seq=%d data=%q\n", w.GetMsgId(), w.GetSeqId(), string(raw))
	})
	client.Read()

	// Login + join room for cross-actor chat flow (gate routes by msgId to business actor).
	// 登录 + 进房：适配聊天室跨 actor 场景（gate 收到后按 msgId 路由到业务 actor）。
	send(int32(*msgLogin), map[string]any{
		"userId":   *userID,
		"nickname": *nickname,
	})
	send(int32(*msgJoin), map[string]any{
		"room":     *room,
		"nickname": *nickname,
	})

	fmt.Printf("connected to %s user=%d nick=%s room=%s\n", *addr, *userID, *nickname, *room)
	fmt.Println("输入聊天内容回车发送；/join 房间；/leave；/quit")

	in := bufio.NewScanner(os.Stdin)
	for in.Scan() {
		line := strings.TrimSpace(in.Text())
		switch line {
		case "":
			continue
		case "/quit":
			fmt.Println("bye")
			return
		case "/leave":
			send(int32(*msgLeave), map[string]any{"room": *room})
			continue
		default:
			if strings.HasPrefix(line, "/join ") {
				nextRoom := strings.TrimSpace(strings.TrimPrefix(line, "/join "))
				if nextRoom == "" {
					fmt.Println("usage: /join <room>")
					continue
				}
				send(int32(*msgJoin), map[string]any{
					"room":     nextRoom,
					"nickname": *nickname,
				})
				*room = nextRoom
				continue
			}
			send(int32(*msgSend), map[string]any{
				"room": *room,
				"text": line,
			})
		}
	}
	if err := in.Err(); err != nil {
		fmt.Printf("stdin error: %v\n", err)
	}
}

// verifyChatBroadcastSM3 与 im_single_demo 发送端一致：对不含 sign 的 payload 做 JSON 再 SM3，与 sign 比对。
func verifyChatBroadcastSM3(raw []byte) string {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		return ""
	}
	typ, _ := m["type"].(string)
	if typ != "chat_broadcast" {
		return ""
	}
	sigHex, ok := m["sign"].(string)
	if !ok || sigHex == "" {
		return ""
	}
	delete(m, "sign")
	canon, err := json.Marshal(m)
	if err != nil {
		return fmt.Sprintf("[chat_broadcast] SM3 校验: 重编码失败: %v", err)
	}
	want := zencrypt.SM3Bytes(canon)
	got, err := hex.DecodeString(sigHex)
	if err != nil || len(got) != len(want) {
		return "[chat_broadcast] SM3 sign 非法"
	}
	if !bytes.Equal(want, got) {
		return "[chat_broadcast] SM3 摘要不匹配（内容可能已变或与服务端算法不一致）"
	}
	return "[chat_broadcast] SM3 摘要校验 OK"
}

func gmCipherSuiteName(id uint16) string {
	switch id {
	case gmtls.GMTLS_ECDHE_SM2_WITH_SM4_SM3:
		return "GMTLS_ECDHE_SM2_WITH_SM4_SM3"
	case gmtls.GMTLS_SM2_WITH_SM4_SM3:
		return "GMTLS_SM2_WITH_SM4_SM3"
	default:
		return fmt.Sprintf("unknown(0x%04x)", id)
	}
}

// applyClientGMTLSCipherSuite 设置客户端 GM-TLS CipherSuites，与服务端 im_single_demo -gmCipherSuite 同含义。
func applyClientGMTLSCipherSuite(cfg *ziface.TLSConfig, mode string) {
	if cfg == nil || cfg.GMConfig == nil {
		return
	}
	m := strings.ToLower(strings.TrimSpace(mode))
	switch m {
	case "", "default":
		return
	case "ecdhe":
		cfg.GMConfig.SetCipherSuites([]uint16{gmtls.GMTLS_ECDHE_SM2_WITH_SM4_SM3})
	case "ecc":
		cfg.GMConfig.SetCipherSuites([]uint16{gmtls.GMTLS_SM2_WITH_SM4_SM3})
	case "both":
		cfg.GMConfig.SetCipherSuites([]uint16{gmtls.GMTLS_ECDHE_SM2_WITH_SM4_SM3, gmtls.GMTLS_SM2_WITH_SM4_SM3})
	default:
		panic(fmt.Sprintf("未知 -gmCipherSuite %q（可用 default|ecdhe|ecc|both）", mode))
	}
}
