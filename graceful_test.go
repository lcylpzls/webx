package webx

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lcylpzls/logx"
)

func TestGracefulShutdownSignal(t *testing.T) {
	logger := DefaultLogger(logx.InfoLevel)
	defer logger.Close()
	quit := make(chan os.Signal, 1)
	quit <- os.Interrupt
	err := gracefulShutdown(context.Background(), logger, nil, nil, time.Second, "", nil, quit)
	if err != nil {
		t.Errorf("信号关闭应成功：%v", err)
	}
}

func TestGracefulShutdownContextCanceled(t *testing.T) {
	logger := DefaultLogger(logx.InfoLevel)
	defer logger.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := gracefulShutdown(ctx, logger, nil, nil, time.Second, "", nil, make(chan os.Signal))
	if !errors.Is(err, context.Canceled) {
		t.Errorf("取消上下文应返回 Canceled：%v", err)
	}
}

func TestGracefulShutdownPublic(t *testing.T) {
	logger := DefaultLogger(logx.InfoLevel)
	defer logger.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- GracefulShutdown(ctx, logger, nil, nil, time.Second, "", nil)
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("GracefulShutdown 应返回 Canceled：%v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("GracefulShutdown 未返回")
	}
}

func TestShutdownServersErrors(t *testing.T) {
	logger := DefaultLogger(logx.InfoLevel)
	defer logger.Close()

	// 构造：nil Server、活跃连接阻塞的 Server、nil Listener、Close 失败的 Listener、
	// 非空目录（os.Remove 失败）、清理函数
	block := make(chan struct{})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-block })}
	go srv.Serve(ln)
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: x\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)

	nonEmptyDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(nonEmptyDir, "x"), []byte("x"), 0o600)
	badLN := &errCloseListener{Listener: ln}
	called := 0
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	err = shutdownServers(shutdownCtx, logger,
		[]*http.Server{nil, srv}, []net.Listener{nil, badLN}, time.Second, nonEmptyDir,
		[]func(){func() { called++ }})
	cancel()
	close(block)
	conn.Close()
	if err == nil {
		t.Error("超时/监听器/文件清理错误应被聚合返回")
	}
	if called != 1 {
		t.Errorf("清理函数应执行：%d", called)
	}
	ln.Close()
}

// errCloseListener 是 Close 必然失败的假监听器。
type errCloseListener struct {
	net.Listener
}

func (l *errCloseListener) Close() error {
	return errors.New("自定义关闭失败")
}

func TestShutdownServersClean(t *testing.T) {
	logger := DefaultLogger(logx.InfoLevel)
	defer logger.Close()
	path := filepath.Join(t.TempDir(), "s.sock")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	called := 0
	err := shutdownServers(context.Background(), logger, nil, nil, time.Second, path,
		[]func(){func() { called++ }})
	if err != nil {
		t.Errorf("清理关闭应成功：%v", err)
	}
	if called != 1 {
		t.Errorf("清理函数应执行：%d", called)
	}
}
