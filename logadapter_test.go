package webx

import (
	"errors"
	"testing"

	"github.com/lcylpzls/logx"
)

func TestDefaultLogger(t *testing.T) {
	l := DefaultLogger(logx.InfoLevel)
	if l == nil {
		t.Fatal("DefaultLogger 不应返回 nil")
	}
	l.Info("测试日志", logx.Fields())
	_ = l.Close()
}

func TestDefaultLoggerFallback(t *testing.T) {
	orig := buildDefaultLogger
	buildDefaultLogger = func(level logx.Level) (logx.Logger, error) {
		return nil, errors.New("模拟构建失败")
	}
	defer func() { buildDefaultLogger = orig }()

	l := DefaultLogger(logx.InfoLevel)
	if l == nil {
		t.Fatal("构建失败时应回退为可用 Logger")
	}
	l.Info("回退日志", logx.Fields())
	_ = l.Close()
}

func TestParseLogLevel(t *testing.T) {
	cases := map[string]logx.Level{
		"debug": logx.DebugLevel,
		"info":  logx.InfoLevel,
		"warn":  logx.WarnLevel,
		"error": logx.ErrorLevel,
		"":      logx.InfoLevel,
		"other": logx.InfoLevel,
	}
	for in, want := range cases {
		if got := parseLogLevel(in); got != want {
			t.Errorf("parseLogLevel(%q) 不符：got %v, want %v", in, got, want)
		}
	}
}
