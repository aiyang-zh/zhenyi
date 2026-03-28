// 生成本地 SM2 自签证书（仅测试），供 im_single_demo 国密 GM-TLS 使用。
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
	"github.com/emmansun/gmsm/smx509"
)

func main() {
	outDir := flag.String("out", "", "输出目录（默认：本仓库 examples/im_single_demo/certs）")
	dual := flag.Bool("dual", false, "生成国密双证书：sm2_sign.* + sm2_enc.*（否则只生成单证书 sm2.crt/sm2.key）")
	flag.Parse()

	dir := *outDir
	if dir == "" {
		dir = filepath.Join("examples", "im_single_demo", "certs")
	}
	absDir, err := filepath.Abs(filepath.Clean(dir))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.MkdirAll(absDir, 0o750); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	root, err := os.OpenRoot(absDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer root.Close()

	if *dual {
		if err := writeDual(root); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		printDualHelp(absDir)
		return
	}

	if err := writeSingle(root); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	printSingleHelp(absDir)
}

func randomSerial() (*big.Int, error) {
	return rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
}

func baseTemplate() smx509.Certificate {
	return smx509.Certificate{
		Subject: pkix.Name{
			CommonName:   "zhenyi-im-single-demo.local",
			Organization: []string{"zhenyi-local-test"},
			Country:      []string{"CN"},
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		BasicConstraintsValid: true,
		IsCA:                  false,
		SignatureAlgorithm:    smx509.SM2WithSM3,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.IPv4(127, 0, 0, 1)},
	}
}

// name 必须为单层文件名（不含路径遍历），由调用方保证为常量名。
func writeCertPEM(root *os.Root, name string, der []byte) error {
	if !safeRootFileName(name) {
		return fmt.Errorf("gencert: invalid output name %q", name)
	}
	f, err := root.Create(name)
	if err != nil {
		return err
	}
	defer f.Close()
	return pem.Encode(f, &pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func writeKeyPEM(root *os.Root, name string, priv *sm2.PrivateKey) error {
	if !safeRootFileName(name) {
		return fmt.Errorf("gencert: invalid output name %q", name)
	}
	keyDER, err := smx509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return err
	}
	f, err := root.Create(name)
	if err != nil {
		return err
	}
	defer f.Close()
	return pem.Encode(f, &pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
}

func safeRootFileName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	return filepath.Base(name) == name
}

func writeSingle(root *os.Root) error {
	priv, err := sm2.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	serial, err := randomSerial()
	if err != nil {
		return err
	}
	tpl := baseTemplate()
	tpl.SerialNumber = serial
	tpl.KeyUsage = smx509.KeyUsageDigitalSignature | smx509.KeyUsageKeyEncipherment
	tpl.ExtKeyUsage = []smx509.ExtKeyUsage{smx509.ExtKeyUsageServerAuth}

	der, err := smx509.CreateCertificate(rand.Reader, &tpl, &tpl, &priv.PublicKey, priv)
	if err != nil {
		return err
	}
	if err := writeCertPEM(root, "sm2.crt", der); err != nil {
		return err
	}
	return writeKeyPEM(root, "sm2.key", priv)
}

func writeDual(root *os.Root) error {
	// 签名证书 + 签名私钥
	signKey, err := sm2.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	signSerial, err := randomSerial()
	if err != nil {
		return err
	}
	signTpl := baseTemplate()
	signTpl.SerialNumber = signSerial
	signTpl.Subject.CommonName = "zhenyi-im-single-demo-sign.local"
	signTpl.KeyUsage = smx509.KeyUsageDigitalSignature
	signTpl.ExtKeyUsage = []smx509.ExtKeyUsage{smx509.ExtKeyUsageServerAuth}

	signDER, err := smx509.CreateCertificate(rand.Reader, &signTpl, &signTpl, &signKey.PublicKey, signKey)
	if err != nil {
		return err
	}
	if err := writeCertPEM(root, "sm2_sign.crt", signDER); err != nil {
		return err
	}
	if err := writeKeyPEM(root, "sm2_sign.key", signKey); err != nil {
		return err
	}

	// 加密证书 + 加密私钥
	encKey, err := sm2.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	encSerial, err := randomSerial()
	if err != nil {
		return err
	}
	encTpl := baseTemplate()
	encTpl.SerialNumber = encSerial
	encTpl.Subject.CommonName = "zhenyi-im-single-demo-enc.local"
	encTpl.KeyUsage = smx509.KeyUsageKeyEncipherment | smx509.KeyUsageDataEncipherment
	encTpl.ExtKeyUsage = []smx509.ExtKeyUsage{smx509.ExtKeyUsageServerAuth}

	encDER, err := smx509.CreateCertificate(rand.Reader, &encTpl, &encTpl, &encKey.PublicKey, encKey)
	if err != nil {
		return err
	}
	if err := writeCertPEM(root, "sm2_enc.crt", encDER); err != nil {
		return err
	}
	return writeKeyPEM(root, "sm2_enc.key", encKey)
}

func printSingleHelp(dir string) {
	certPath := filepath.Join(dir, "sm2.crt")
	keyPath := filepath.Join(dir, "sm2.key")
	fmt.Printf("written:\n  %s\n  %s\n", certPath, keyPath)
	fmt.Println("警告: 以上为测试用自签证书，仅本地/联调使用，禁止用于生产或线上。")
	fmt.Println("Warning: test-only self-signed certs — not for production.")
	fmt.Println()
	fmt.Println("单证书服务端:")
	fmt.Printf("  go run ./examples/im_single_demo -conn tcp -addr 127.0.0.1:8001 -gmCert %s -gmKey %s\n", certPath, keyPath)
	fmt.Println("客户端:")
	fmt.Printf("  go run ./examples/im_single_client -gmtls -gmInsecure -addr 127.0.0.1:8001\n")
	fmt.Println()
	fmt.Println("双证书请使用: go run ./examples/im_single_demo/gencert -dual")
}

func printDualHelp(dir string) {
	signC := filepath.Join(dir, "sm2_sign.crt")
	signK := filepath.Join(dir, "sm2_sign.key")
	encC := filepath.Join(dir, "sm2_enc.crt")
	encK := filepath.Join(dir, "sm2_enc.key")
	fmt.Printf("written:\n  %s\n  %s\n  %s\n  %s\n", signC, signK, encC, encK)
	fmt.Println("警告: 以上为测试用自签证书，仅本地/联调使用，禁止用于生产或线上。")
	fmt.Println("Warning: test-only self-signed certs — not for production.")
	fmt.Println()
	fmt.Println("双证书服务端:")
	fmt.Printf("  go run ./examples/im_single_demo -conn tcp -addr 127.0.0.1:8001 -gmSignCert %s -gmSignKey %s -gmEncCert %s -gmEncKey %s\n",
		signC, signK, encC, encK)
	fmt.Println("客户端:")
	fmt.Printf("  go run ./examples/im_single_client -gmtls -gmInsecure -addr 127.0.0.1:8001\n")
}
