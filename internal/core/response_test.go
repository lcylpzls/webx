package core

import (
	"encoding/json"
	"github.com/lcylpzls/testx"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStandardizedResponseFormat(t *testing.T) {
	// Success：nil data 省略 data 字段
	rec := httptest.NewRecorder()
	c := NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.Success("ok", nil)
	var got map[string]any
	testx.RequireNoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	if got["code"] != float64(0) || got["msg"] != "ok" || got["timestamp"] == nil {
		t.Errorf("Success 信封不符：%s", rec.Body.String())
	}
	if _, ok := got["data"]; ok {
		t.Errorf("nil data 应省略：%s", rec.Body.String())
	}

	// 空字符串 data 同样省略（与 omitempty 对齐）
	rec = httptest.NewRecorder()
	c = NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.Success("ok", "")
	if strings.Contains(rec.Body.String(), `"data"`) {
		t.Errorf("空字符串 data 应省略：%s", rec.Body.String())
	}

	// 非空 data 保留
	rec = httptest.NewRecorder()
	c = NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.Success("ok", map[string]string{"a": "b"})
	if !strings.Contains(rec.Body.String(), `"data":{"a":"b"}`) {
		t.Errorf("data 字段不符：%s", rec.Body.String())
	}

	// JSON 字符串转义与 HTML 敏感字符
	rec = httptest.NewRecorder()
	c = NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.Success(`引号"与<HTML>&`, nil)
	body := rec.Body.String()
	for _, want := range []string{`\"`, `\u003c`, `\u003e`, `\u0026`} {
		testx.RequireTrue(t, strings.Contains(body, want))
	}

	// Fail 业务码与 JSONResponse 状态码
	rec = httptest.NewRecorder()
	c = NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.Fail(http.StatusBadRequest, 1001, "参数错误")
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["code"] != float64(1001) || rec.Code != http.StatusBadRequest {
		t.Errorf("Fail 信封不符：%s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	c = NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.JSONResponse(http.StatusCreated, "创建成功", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["code"] != float64(http.StatusCreated) || rec.Code != http.StatusCreated {
		t.Errorf("JSONResponse 信封不符：%s", rec.Body.String())
	}
}

func TestStandardizedResponseOmitEmpty(t *testing.T) {
	cases := []any{
		false,
		0,
		int64(0),
		uint(0),
		0.0,
		[]string{},
		map[string]string{},
	}
	for _, data := range cases {
		rec := httptest.NewRecorder()
		c := NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		c.Success("ok", data)
		if strings.Contains(rec.Body.String(), `"data"`) {
			t.Errorf("空值 data 应省略（%T）：%s", data, rec.Body.String())
		}
	}

	// 结构体恒不省略（与 encoding/json omitempty 一致）
	rec := httptest.NewRecorder()
	c := NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.Success("ok", struct{}{})
	if !strings.Contains(rec.Body.String(), `"data":{}`) {
		t.Errorf("结构体 data 应保留：%s", rec.Body.String())
	}
}

func TestStandardizedResponseEscaping(t *testing.T) {
	rec := httptest.NewRecorder()
	c := NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.Success("a\nb\tc", nil)
	body := rec.Body.String()
	if !strings.Contains(body, `\u000a`) || !strings.Contains(body, `\u0009`) {
		t.Errorf("控制字符转义缺失：%s", body)
	}
}

func TestStandardizedResponseMarshalError(t *testing.T) {
	rec := httptest.NewRecorder()
	c := NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.Success("ok", make(chan int))
	if !strings.Contains(rec.Body.String(), `"data":null`) {
		t.Errorf("不可序列化 data 应降级为 null：%s", rec.Body.String())
	}
}

func TestSetContentTypeReuse(t *testing.T) {
	rec := httptest.NewRecorder()
	c := NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	c.Success("ok", nil)
	c.Success("again", nil) // 第二次写入复用已有 Content-Type 切片
	if rec.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Errorf("Content-Type 不符：%v", rec.Header())
	}
}
