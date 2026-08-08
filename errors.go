package webx

import "github.com/lcylpzls/errx"

// webx 错误码：统一使用 errx 结构化错误。
const (
	// CodeConfigInvalid 配置校验失败。
	CodeConfigInvalid errx.Code = "WEBX_CONFIG_INVALID"
	// CodeConfigLoadFailed 配置文件加载失败。
	CodeConfigLoadFailed errx.Code = "WEBX_CONFIG_LOAD_FAILED"
	// CodeListenFailed 监听器创建失败。
	CodeListenFailed errx.Code = "WEBX_LISTEN_FAILED"
	// CodeStartFailed 服务启动失败。
	CodeStartFailed errx.Code = "WEBX_START_FAILED"
	// CodeShutdownFailed 优雅关闭失败。
	CodeShutdownFailed errx.Code = "WEBX_SHUTDOWN_FAILED"
	// CodePanic 请求处理发生 panic（Recovery 中间件捕获）。
	CodePanic errx.Code = "WEBX_PANIC"
)

func init() {
	errx.RegisterCode(CodeConfigInvalid, "webx 配置校验失败")
	errx.RegisterCode(CodeConfigLoadFailed, "webx 配置文件加载失败")
	errx.RegisterCode(CodeListenFailed, "webx 监听器创建失败")
	errx.RegisterCode(CodeStartFailed, "webx 服务启动失败")
	errx.RegisterCode(CodeShutdownFailed, "webx 优雅关闭失败")
	errx.RegisterCode(CodePanic, "webx 请求处理发生 panic")
}
