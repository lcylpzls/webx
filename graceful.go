package webx

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lcylpzls/logx"
)

// closeServer 强制关闭服务（测试可替换以覆盖异常分支）。
var closeServer = func(srv *http.Server) error {
	return srv.Close()
}

// GracefulShutdown 监听系统信号并执行优雅关闭。
// 收到 SIGINT/SIGTERM 后调用 httpServer.Shutdown 排空请求。
func GracefulShutdown(
	ctx context.Context,
	logger logx.Logger,
	httpServer *http.Server,
	listener net.Listener,
	shutdownTimeout time.Duration,
	unixSocketPath string,
	cleanupFuncs []func(),
) error {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	return gracefulShutdown(ctx, logger, []*http.Server{httpServer}, []net.Listener{listener},
		shutdownTimeout, unixSocketPath, cleanupFuncs, quit)
}

// gracefulShutdown 内部实现，接受外部注入的信号通道便于测试。
func gracefulShutdown(
	ctx context.Context,
	logger logx.Logger,
	servers []*http.Server,
	listeners []net.Listener,
	shutdownTimeout time.Duration,
	unixSocketPath string,
	cleanupFuncs []func(),
	quit <-chan os.Signal,
) error {
	select {
	case sig := <-quit:
		logger.WithContext(ctx).Info(fmt.Sprintf("webx：收到系统信号 %s，开始优雅关闭", sig), logx.Fields())
		shutdownServers(ctx, logger, servers, listeners, shutdownTimeout, unixSocketPath, cleanupFuncs)
		logger.WithContext(ctx).Info("webx：服务已优雅关闭", logx.Fields())
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// shutdownServers 关闭全部 HTTP Server 与监听器，清理 Unix Socket 并执行清理函数。
func shutdownServers(
	ctx context.Context,
	logger logx.Logger,
	servers []*http.Server,
	listeners []net.Listener,
	shutdownTimeout time.Duration,
	unixSocketPath string,
	cleanupFuncs []func(),
) error {
	var err error
	for _, srv := range servers {
		if srv == nil {
			continue
		}
		shutdownCtx, cancel := context.WithTimeout(ctx, shutdownTimeout)
		if shutdownErr := srv.Shutdown(shutdownCtx); shutdownErr != nil {
			err = errors.Join(err, shutdownErr)
			// 超时未排空：强制关闭残余连接，保证关闭彻底。
			if closeErr := closeServer(srv); closeErr != nil && !errors.Is(closeErr, http.ErrServerClosed) {
				err = errors.Join(err, closeErr)
			}
		}
		cancel()
	}
	for _, ln := range listeners {
		if ln == nil {
			continue
		}
		if closeErr := ln.Close(); closeErr != nil {
			// Shutdown 已关闭监听器时忽略重复关闭错误
			if !errors.Is(closeErr, net.ErrClosed) {
				err = errors.Join(err, closeErr)
			}
		}
	}
	if unixSocketPath != "" {
		if removeErr := os.Remove(unixSocketPath); removeErr != nil && !os.IsNotExist(removeErr) {
			logger.WithContext(ctx).Warn("webx：残留 Socket 文件清理失败",
				logx.Fields(logx.String("路径", unixSocketPath), logx.Any("error", removeErr)))
			err = errors.Join(err, removeErr)
		}
	}
	for _, fn := range cleanupFuncs {
		if fn != nil {
			fn()
		}
	}
	return err
}
