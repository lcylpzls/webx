package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lcylpzls/logx"
	"github.com/lcylpzls/webx/internal/core"
)

func TestAccessLogSuccessOnly(t *testing.T) {
	var buf bytes.Buffer
	logger, err := logx.NewBuilder().EnableWriter(&buf, logx.InfoLevel).Build()
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	// 成功请求且 successOnly=false：不记录
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/ok", nil))
	c.SetHandlers([]core.HandlerFunc{
		AccessLog(logger, AccessLogOptions{}),
		func(c *core.Context) { c.Success("ok", nil) },
	})
	c.Run()
	if buf.Len() != 0 {
		t.Errorf("默认不应记录成功请求：%s", buf.String())
	}

	// 失败请求：记录 Warn
	rec = httptest.NewRecorder()
	failReq := httptest.NewRequest(http.MethodGet, "/fail", nil)
	failReq.Proto = "HTTP/3.0"
	c = core.NewContext(rec, failReq)
	c.SetHandlers([]core.HandlerFunc{
		AccessLog(logger, AccessLogOptions{}),
		func(c *core.Context) { c.JSONResponse(http.StatusNotFound, "不存在", nil) },
	})
	c.Run()
	if !strings.Contains(buf.String(), "访问日志") || !strings.Contains(buf.String(), "404") {
		t.Errorf("失败请求应记录：%s", buf.String())
	}
	if !strings.Contains(buf.String(), "duration_ms") || !strings.Contains(buf.String(), "user_agent") {
		t.Errorf("访问日志应包含增强字段：%s", buf.String())
	}
	if !strings.Contains(buf.String(), "bytes=") {
		t.Errorf("访问日志应包含响应字节数：%s", buf.String())
	}
	if !strings.Contains(buf.String(), "proto=HTTP/3") {
		t.Errorf("访问日志应包含可读协议字段：%s", buf.String())
	}
	if strings.Contains(buf.String(), "proto=HTTP/3.0") {
		t.Errorf("协议字段不应保留版本点：%s", buf.String())
	}
}

func TestAccessLogLogAll(t *testing.T) {
	var buf bytes.Buffer
	logger, err := logx.NewBuilder().EnableWriter(&buf, logx.InfoLevel).Build()
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	rec := httptest.NewRecorder()
	c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/all", nil))
	c.SetHandlers([]core.HandlerFunc{
		AccessLog(logger, AccessLogOptions{LogSuccess: true}),
		func(c *core.Context) { c.Success("ok", nil) },
	})
	c.Run()
	if !strings.Contains(buf.String(), "/all") {
		t.Errorf("successOnly=true 应记录全部请求：%s", buf.String())
	}
	if !strings.Contains(buf.String(), "duration_ms") {
		t.Errorf("成功请求日志应包含 duration_ms：%s", buf.String())
	}
}

func TestAccessLogRedaction(t *testing.T) {
	var buf bytes.Buffer
	logger, err := logx.NewBuilder().EnableWriter(&buf, logx.InfoLevel).Build()
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	req := httptest.NewRequest(http.MethodGet, "/s?token=abc123&name=ok", nil)
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, req)
	c.SetHandlers([]core.HandlerFunc{
		AccessLog(logger, AccessLogOptions{LogSuccess: true, RedactKeys: []string{"token"}}),
		func(c *core.Context) { c.Success("ok", nil) },
	})
	c.Run()
	if strings.Contains(buf.String(), "abc123") {
		t.Errorf("token 应被脱敏：%s", buf.String())
	}
	if !strings.Contains(buf.String(), "%2A%2A%2A") {
		t.Errorf("脱敏占位应出现：%s", buf.String())
	}
}

func TestAccessLogSampling(t *testing.T) {
	var buf bytes.Buffer
	logger, err := logx.NewBuilder().EnableWriter(&buf, logx.InfoLevel).Build()
	if err != nil {
		t.Fatal(err)
	}
	defer logger.Close()

	origRand := sampleRand
	defer func() { sampleRand = origRand }()

	// 采样命中：记录
	sampleRand = func(int) int { return 0 }
	buf.Reset()
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/s", nil))
	c.SetHandlers([]core.HandlerFunc{
		AccessLog(logger, AccessLogOptions{LogSuccess: true, SampleRate: 10}),
		func(c *core.Context) { c.Success("ok", nil) },
	})
	c.Run()
	if !strings.Contains(buf.String(), "访问日志") {
		t.Error("采样命中应记录")
	}

	// 采样未命中：跳过
	sampleRand = func(int) int { return 1 }
	buf.Reset()
	rec = httptest.NewRecorder()
	c = core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/s", nil))
	c.SetHandlers([]core.HandlerFunc{
		AccessLog(logger, AccessLogOptions{LogSuccess: true, SampleRate: 10}),
		func(c *core.Context) { c.Success("ok", nil) },
	})
	c.Run()
	if buf.Len() != 0 {
		t.Errorf("采样未命中应跳过：%s", buf.String())
	}
}

func TestRedactQuery(t *testing.T) {
	if got := redactQuery("", []string{"a"}); got != "" {
		t.Errorf("空 query 应原样返回：%s", got)
	}
	if got := redactQuery("a=1", nil); got != "a=1" {
		t.Errorf("无脱敏键应原样返回：%s", got)
	}
	if got := redactQuery("a=1", []string{"b"}); got != "a=1" {
		t.Errorf("无关键不应变化：%s", got)
	}
	if got := redactQuery("token=x&a=1", []string{"token"}); strings.Contains(got, "x") {
		t.Errorf("token 应被脱敏：%s", got)
	}
	if got := redactQuery("bad=%zz", []string{"a"}); got != "bad=%zz" {
		t.Errorf("非法 query 应原样返回：%s", got)
	}
}

func TestFriendlyProto(t *testing.T) {
	if got := friendlyProto("HTTP/2.0"); got != "HTTP/2" {
		t.Errorf("HTTP/2 转换不符：%s", got)
	}
	if got := friendlyProto("HTTP/3.0"); got != "HTTP/3" {
		t.Errorf("HTTP/3 转换不符：%s", got)
	}
	if got := friendlyProto("HTTP/1.1"); got != "HTTP/1.1" {
		t.Errorf("HTTP/1.1 应原样返回：%s", got)
	}
	if got := friendlyProto("unknown"); got != "unknown" {
		t.Errorf("未知协议应原样返回：%s", got)
	}
}

func TestCountingWriter(t *testing.T) {
	rec := httptest.NewRecorder()
	cw := &countingWriter{ResponseWriter: rec}
	if cw.Unwrap() != rec {
		t.Error("Unwrap 不符")
	}
	n, err := cw.Write([]byte("hello"))
	if err != nil || n != 5 || cw.n != 5 {
		t.Errorf("写入计数不符：n=%d total=%d err=%v", n, cw.n, err)
	}
}
