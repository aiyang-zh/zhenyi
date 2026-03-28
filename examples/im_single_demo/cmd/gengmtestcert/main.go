// 生成 im_single_demo 用的国密 SM2 自签测试证书（仅供本地/联调，勿用于生产）。
// 用法：在 zhenyi 模块根目录执行
//
//	go run ./examples/im_single_demo/cmd/gengmtestcert/ -out examples/im_single_demo/testdata
package main

import (
	"crypto/rand"
	"crypto/x509/pkix"
	"encoding/pem"
	"flag"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/emmansun/gmsm/sm2"
	x509 "github.com/emmansun/gmsm/smx509"
)

func main() {
	outDir := flag.String("out", "", "输出目录（默认当前目录下的 testdata）")
	flag.Parse()

	dir := *outDir
	if dir == "" {
		dir = "testdata"
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	priv, err := sm2.GenerateKey(rand.Reader)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	template := &x509.Certificate{
		SerialNumber:       big.NewInt(1),
		Subject:            pkix.Name{CommonName: "im-single-demo-local"},
		NotBefore:          time.Now().Add(-time.Hour),
		NotAfter:           time.Now().Add(365 * 24 * time.Hour),
		SignatureAlgorithm: x509.SM2WithSM3,
		KeyUsage:           x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		// 新版 x509 要求 SAN；无 SAN 仅 CN 会报 “relies on legacy Common Name field”。
		DNSNames:    []string{"im-single-demo-local", "localhost"},
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1)},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})

	certPath := filepath.Join(dir, "server.pem")
	keyPath := filepath.Join(dir, "server.key")
	if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s\nwrote %s\n", certPath, keyPath)
	fmt.Println("警告: 以上为测试用自签证书，仅本地/联调使用，禁止用于生产或线上。")
	fmt.Println("Warning: test-only self-signed certs — not for production.")
}
