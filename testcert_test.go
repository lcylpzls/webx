package webx

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	testx "github.com/lcylpzls/testx"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// testHelper 抽象 testing.T/B 的公共方法，便于测试与基准共用证书生成。
type testHelper interface {
	Helper()
	TempDir() string
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
	Cleanup(func())
}

// writeTestCert 生成自签名证书与私钥，返回文件路径。
func writeTestCert(t testHelper) (string, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	testx.RequireNoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "webx-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:     []string{"localhost"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	testx.RequireNoError(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalECPrivateKey(key)
	testx.RequireNoError(t, err)

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	dir := t.TempDir()
	certFile := filepath.Join(dir, "cert.pem")
	keyFile := filepath.Join(dir, "key.pem")
	testx.RequireNoError(t, os.WriteFile(certFile, certPEM, 0o600))
	testx.RequireNoError(t, os.WriteFile(keyFile, keyPEM, 0o600))
	return certFile, keyFile
}
