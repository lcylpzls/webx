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
		AccessLog(logger, false),
		func(c *core.Context) { c.Success("ok", nil) },
	})
	c.Run()
	if buf.Len() != 0 {
		t.Errorf("默认不应记录成功请求：%s", buf.String())
	}

	// 失败请求：记录 Warn
	rec = httptest.NewRecorder()
	c = core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/fail", nil))
	c.SetHandlers([]core.HandlerFunc{
		AccessLog(logger, false),
		func(c *core.Context) { c.JSONResponse(http.StatusNotFound, "不存在", nil) },
	})
	c.Run()
	if !strings.Contains(buf.String(), "访问日志") || !strings.Contains(buf.String(), "404") {
		t.Errorf("失败请求应记录：%s", buf.String())
	}
	if !strings.Contains(buf.String(), "duration_ms") || !strings.Contains(buf.String(), "user_agent") {
		t.Errorf("访问日志应包含增强字段：%s", buf.String())
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
		AccessLog(logger, true),
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
