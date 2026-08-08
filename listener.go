package webx

import (
	"crypto/tls"
	"net"
	"os"
	"time"

	"github.com/lcylpzls/errx"
	"github.com/quic-go/quic-go"
)

// removePath 与 chmodPath 是可注入的文件操作函数（测试可替换以覆盖异常分支）。
var removePath = os.Remove
var chmodPath = os.Chmod

// createTLSListener 创建 TLS over TCP 监听器，支持 HTTP/2 与 HTTP/1.1。
func createTLSListener(addr, certFile, keyFile string) (net.Listener, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, errx.Wrap(err, errx.KindUnavailable, CodeListenFailed, "TLS 证书加载失败")
	}
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
		NextProtos:   []string{"h2", "http/1.1"},
	}
	ln, err := tls.Listen("tcp", addr, tlsConfig)
	if err != nil {
		return nil, errx.Wrap(err, errx.KindUnavailable, CodeListenFailed, "TLS 监听失败，地址 "+addr)
	}
	return ln, nil
}

// createQUICListener 创建 QUIC over UDP 监听器，用于 HTTP/3。
func createQUICListener(addr, certFile, keyFile string) (*quic.Listener, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, errx.Wrap(err, errx.KindUnavailable, CodeListenFailed, "QUIC 证书加载失败")
	}
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"h3"},
	}
	ln, err := quic.ListenAddr(addr, tlsConfig, &quic.Config{MaxIdleTimeout: 30 * time.Second})
	if err != nil {
		return nil, errx.Wrap(err, errx.KindUnavailable, CodeListenFailed, "QUIC 监听失败，地址 "+addr)
	}
	return ln, nil
}

// createUnixListener 创建 Unix Socket 监听器。
// 非 Windows 下先清理残留 Socket 文件，再监听并设置权限。
func createUnixListener(path string, perm os.FileMode) (net.Listener, error) {
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return nil, errx.Newf(errx.KindUnavailable, CodeListenFailed, "Unix Socket 路径为目录而非 Socket 文件，路径 %s", path)
	}
	if err := removePath(path); err != nil && !os.IsNotExist(err) {
		return nil, errx.Wrap(err, errx.KindUnavailable, CodeListenFailed, "Unix Socket 残留文件清理失败，路径 "+path)
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, errx.Wrap(err, errx.KindUnavailable, CodeListenFailed, "Unix Socket 监听失败，路径 "+path)
	}
	if err := chmodPath(path, perm); err != nil {
		ln.Close()
		return nil, errx.Wrap(err, errx.KindUnavailable, CodeListenFailed, "Unix Socket 权限设置失败，路径 "+path)
	}
	return ln, nil
}
