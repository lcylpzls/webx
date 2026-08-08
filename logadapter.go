package webx

import (
	"io"

	"github.com/lcylpzls/logx"
)

// buildDefaultLogger 是可注入的默认 Logger 构建函数（测试可替换以覆盖失败分支）。
var buildDefaultLogger = func(level logx.Level) (logx.Logger, error) {
	return logx.NewBuilder().EnableConsole(level).Build()
}

// DefaultLogger 构建默认 logx.Logger：控制台输出，级别由调用方指定。
// 构建失败时回退为静默 Logger，保证服务仍可启动。
func DefaultLogger(level logx.Level) logx.Logger {
	l, err := buildDefaultLogger(level)
	if err != nil {
		l, _ = logx.NewBuilder().EnableWriter(io.Discard, logx.OffLevel).Build()
	}
	return l
}

// parseLogLevel 将配置中的日志级别字符串转为 logx.Level。
// 非法级别默认返回 InfoLevel（配置校验已保证合法）。
func parseLogLevel(level string) logx.Level {
	switch level {
	case "debug":
		return logx.DebugLevel
	case "info":
		return logx.InfoLevel
	case "warn":
		return logx.WarnLevel
	case "error":
		return logx.ErrorLevel
	default:
		return logx.InfoLevel
	}
}
