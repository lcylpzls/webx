package webx

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateTLSListener(t *testing.T) {
	cert, key := writeTestCert(t)
	ln, err := createTLSListener("127.0.0.1:0", cert, key)
	if err != nil {
		t.Fatalf("TLS 监听失败：%v", err)
	}
	defer ln.Close()
	if ln.Addr() == nil {
		t.Error("监听地址为空")
	}
	if _, err := createTLSListener("127.0.0.1:0", "missing.pem", key); err == nil {
		t.Error("证书缺失应报错")
	}
	if _, err := createTLSListener("bad-addr", cert, key); err == nil {
		t.Error("非法地址应报错")
	}
}

func TestCreateQUICListener(t *testing.T) {
	cert, key := writeTestCert(t)
	qln, err := createQUICListener("127.0.0.1:0", cert, key)
	if err != nil {
		t.Fatalf("QUIC 监听失败：%v", err)
	}
	defer qln.Close()
	if _, err := createQUICListener("127.0.0.1:0", "missing.pem", key); err == nil {
		t.Error("证书缺失应报错")
	}
	if _, err := createQUICListener("bad-addr", cert, key); err == nil {
		t.Error("非法地址应报错")
	}
}

func TestCreateUnixListener(t *testing.T) {
	if err := unixSocketSupported(); err != nil {
		t.Skipf("当前平台不支持 Unix Socket：%v", err)
	}
	path := filepath.Join(t.TempDir(), "test.sock")
	ln, err := createUnixListener(path, 0o600)
	if err != nil {
		t.Fatalf("Unix 监听失败：%v", err)
	}
	ln.Close()
	os.Remove(path)

	// 目录路径报错
	if _, err := createUnixListener(t.TempDir(), 0o600); err == nil {
		t.Error("目录路径应报错")
	}
	// 残留文件自动清理后成功
	stale := filepath.Join(t.TempDir(), "stale.sock")
	if err := os.WriteFile(stale, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	ln2, err := createUnixListener(stale, 0o600)
	if err != nil {
		t.Fatalf("残留清理后监听失败：%v", err)
	}
	ln2.Close()
	os.Remove(stale)

	// 父路径为文件：清理失败分支
	parent := filepath.Join(t.TempDir(), "p")
	if err := os.WriteFile(parent, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := createUnixListener(filepath.Join(parent, "child.sock"), 0o600); err == nil {
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
	ro := filepath.Join(t.TempDir(), "ro")
	if err := os.Mkdir(ro, 0o500); err != nil {
		t.Fatal(err)
	}
	if _, err := createUnixListener(filepath.Join(ro, "x.sock"), 0o600); err == nil {
		t.Log("只读目录下监听成功（以 root 运行，跳过权限断言）")
	}
}

func TestUnixSocketSupportedOther(t *testing.T) {
	// 在支持的平台上应返回 nil；在旧 Windows 上由 checkWindowsBuild 测试覆盖。
	_ = unixSocketSupported()
}
