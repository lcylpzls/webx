package webx

import (
	"crypto/tls"
	"net"
	"os"
	"sync"
	"time"

	"github.com/lcylpzls/errx"
	"github.com/quic-go/quic-go"
)

// removePath 与 chmodPath 是可注入的文件操作函数（测试可替换以覆盖异常分支）。
var removePath = os.Remove
var chmodPath = os.Chmod

// certificateProvider 按需加载证书，文件变化（mtime）时自动重载，支持证书轮换。
type certificateProvider struct {
	certFile  string
	keyFile   string
	mu        sync.Mutex
	cert      *tls.Certificate
	mtimeCert time.Time
	mtimeKey  time.Time
}

// newCertificateProvider 创建证书提供器。
func newCertificateProvider(certFile, keyFile string) *certificateProvider {
	return &certificateProvider{certFile: certFile, keyFile: keyFile}
}

// getCertificate 实现 tls.Config.GetCertificate：文件未变化时返回缓存证书。
func (p *certificateProvider) getCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	infoCert, err := os.Stat(p.certFile)
	if err != nil {
		return nil, err
	}
	infoKey, err := os.Stat(p.keyFile)
	if err != nil {
		return nil, err
	}
	if p.cert != nil && infoCert.ModTime().Equal(p.mtimeCert) && infoKey.ModTime().Equal(p.mtimeKey) {
		return p.cert, nil
	}
	cert, err := tls.LoadX509KeyPair(p.certFile, p.keyFile)
	if err != nil {
		return nil, err
	}
	p.cert = &cert
	p.mtimeCert = infoCert.ModTime()
	p.mtimeKey = infoKey.ModTime()
	return p.cert, nil
}

// buildTLSConfig 构建服务端 TLS 配置。
func buildTLSConfig(getCert func(*tls.ClientHelloInfo) (*tls.Certificate, error), minVersion uint16, nextProtos []string) *tls.Config {
	return &tls.Config{
		GetCertificate: getCert,
		MinVersion:     minVersion,
		NextProtos:     nextProtos,
	}
}

// createTLSListener 创建 TLS over TCP 监听器，支持 HTTP/2 与 HTTP/1.1。
func createTLSListener(addr string, getCert func(*tls.ClientHelloInfo) (*tls.Certificate, error), minVersion uint16) (net.Listener, error) {
	ln, err := tls.Listen("tcp", addr, buildTLSConfig(getCert, minVersion, []string{"h2", "http/1.1"}))
	if err != nil {
		return nil, errx.Wrap(err, errx.KindUnavailable, CodeListenFailed, "TLS 监听失败，地址 "+addr)
	}
	return ln, nil
}

// createQUICListener 创建 QUIC over UDP 监听器，用于 HTTP/3。
func createQUICListener(addr string, getCert func(*tls.ClientHelloInfo) (*tls.Certificate, error), minVersion uint16, maxIdle time.Duration, maxStreams int64) (*quic.Listener, error) {
	ln, err := quic.ListenAddr(addr, buildTLSConfig(getCert, minVersion, []string{"h3"}), &quic.Config{
		MaxIdleTimeout:     maxIdle,
		MaxIncomingStreams: maxStreams,
	})
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
