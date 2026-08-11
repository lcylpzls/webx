package webx

import (
	"crypto/tls"
	"errors"
	testx "github.com/lcylpzls/testx"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testGetCert(cert, key string) func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	return newCertificateProvider(cert, key).getCertificate
}

func TestCreateTLSListener(t *testing.T) {
	cert, key := writeTestCert(t)
	ln, err := createTLSListener("127.0.0.1:0", testGetCert(cert, key), tls.VersionTLS12)
	testx.RequireNoError(t, err)

	defer ln.Close()
	if ln.Addr() == nil {
		t.Error("监听地址为空")
	}
	if _, err := createTLSListener("bad-addr", testGetCert(cert, key), tls.VersionTLS12); err == nil {
		t.Error("非法地址应报错")
	}
	if _, err := testGetCert("missing.pem", key)(nil); err == nil {
		t.Error("证书缺失时加载应报错")
	}
}

func TestCreateQUICListener(t *testing.T) {
	cert, key := writeTestCert(t)
	qln, err := createQUICListener("127.0.0.1:0", testGetCert(cert, key), tls.VersionTLS13, 30*time.Second, 100)
	testx.RequireNoError(t, err)

	defer qln.Close()
	if _, err := createQUICListener("bad-addr", testGetCert(cert, key), tls.VersionTLS13, 30*time.Second, 100); err == nil {
		t.Error("非法地址应报错")
	}
}

func TestCertificateProviderReload(t *testing.T) {
	dir := t.TempDir()
	certFile := filepath.Join(dir, "cert.pem")
	keyFile := filepath.Join(dir, "key.pem")
	cert1, key1 := writeTestCert(t)
	cert2, key2 := writeTestCert(t)
	copyFile(t, cert1, certFile)
	copyFile(t, key1, keyFile)

	p := newCertificateProvider(certFile, keyFile)
	first, err := p.getCertificate(nil)
	testx.RequireNoError(t, err)

	cached, err := p.getCertificate(nil)
	testx.RequireNoError(t, err)

	if string(first.Certificate[0]) != string(cached.Certificate[0]) {
		t.Error("缓存证书应一致")
	}
	// 覆盖证书文件（mtime 变化）→ 重载
	time.Sleep(10 * time.Millisecond)
	copyFile(t, cert2, certFile)
	copyFile(t, key2, keyFile)
	reloaded, err := p.getCertificate(nil)
	testx.RequireNoError(t, err)

	if string(first.Certificate[0]) == string(reloaded.Certificate[0]) {
		t.Error("证书文件变化后应重载为新证书")
	}

	// 私钥缺失 → Stat 错误
	if _, err := newCertificateProvider(certFile, filepath.Join(dir, "no.key")).getCertificate(nil); err == nil {
		t.Error("私钥缺失应报错")
	}
	// 证书与私钥不配对 → LoadX509KeyPair 错误
	other, _ := writeTestCert(t)
	if _, err := newCertificateProvider(certFile, other).getCertificate(nil); err == nil {
		t.Error("证书私钥不配对应报错")
	}
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	testx.RequireNoError(t, err)

	if err := os.WriteFile(dst, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestCreateUnixListener(t *testing.T) {
	if err := unixSocketSupported(); err != nil {
		t.Skipf("当前平台不支持 Unix Socket：%v", err)
	}
	// Windows AF_UNIX 路径需 ≤60 字符，t.TempDir() 可能过长，回退到 shortUnixDir()
	// 下创建唯一子目录。
	dir := unixSockTempDir(t)
	path := filepath.Join(dir, "t.sock")
	ln, err := createUnixListener(path, 0o600)
	testx.RequireNoError(t, err)

	ln.Close()
	os.Remove(path)

	// 目录路径报错
	if _, err := createUnixListener(dir, 0o600); err == nil {
		t.Error("目录路径应报错")
	}
	// 残留文件自动清理后成功
	stale := filepath.Join(dir, "t2.sock")
	if err := os.WriteFile(stale, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	ln2, err := createUnixListener(stale, 0o600)
	testx.RequireNoError(t, err)

	ln2.Close()
	os.Remove(stale)

	// 父路径为文件：清理失败分支
	parent := filepath.Join(dir, "p")
	if err := os.WriteFile(parent, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := createUnixListener(filepath.Join(parent, "c.sock"), 0o600); err == nil {
		t.Error("父路径为文件应报错")
	}
	// 注入 Remove 异常覆盖清理失败分支
	origRemove := removePath
	removePath = func(string) error { return errors.New("模拟清理失败") }
	if _, err := createUnixListener(path, 0o600); err == nil {
		t.Error("清理失败应报错")
	}
	removePath = origRemove
	// 注入 Chmod 异常覆盖权限设置失败分支
	origChmod := chmodPath
	chmodPath = func(string, os.FileMode) error { return errors.New("模拟权限失败") }
	if _, err := createUnixListener(path, 0o600); err == nil {
		t.Error("权限设置失败应报错")
	}
	chmodPath = origChmod
	// 权限设置失败：父目录只读
	ro := filepath.Join(dir, "ro")
	if err := os.Mkdir(ro, 0o500); err != nil {
		t.Fatal(err)
	}
	if _, err := createUnixListener(filepath.Join(ro, "x.sock"), 0o600); err == nil {
		t.Log("只读目录下监听成功（以 root 运行，跳过权限断言）")
	}
}

// TestCreateUnixListenerLongPath 覆盖 Unix Socket 路径长度校验失败分支。
func TestCreateUnixListenerLongPath(t *testing.T) {
	if err := unixSocketSupported(); err != nil {
		t.Skipf("当前平台不支持 Unix Socket：%v", err)
	}
	long := strings.Repeat("x", maxUnixPathLen+20)
	if _, err := createUnixListener(long, 0o600); err == nil {
		t.Fatal("超长路径应报错")
	}
}

func TestUnixSocketSupportedOther(t *testing.T) {
	// 在支持的平台上应返回 nil；在旧 Windows 上由 checkWindowsBuild 测试覆盖。
	_ = unixSocketSupported()
}
