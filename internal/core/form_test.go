package core

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type formModel struct {
	Name   string   `form:"name"`
	Age    int      `form:"age"`
	Active bool     `form:"active"`
	Tags   []string `form:"tags"`
	Skip   string   `form:"-"`
}

func TestBindFormURLEncoded(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/",
		strings.NewReader("name=webx&age=3&active=true&tags=a&tags=b&skip=1"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c := NewContext(httptest.NewRecorder(), req)
	var m formModel
	if err := c.BindForm(&m); err != nil {
		t.Fatalf("BindForm 失败：%v", err)
	}
	if m.Name != "webx" || m.Age != 3 || !m.Active || len(m.Tags) != 2 || m.Tags[1] != "b" {
		t.Errorf("表单绑定不符：%+v", m)
	}
}

type formModelMore struct {
	Count uint    `form:"count"`
	Ratio float64 `form:"ratio"`
	On    bool    `form:"on"`
}

type formModelBad struct {
	IDs []int `form:"ids"`
}

type formModelUnsupported struct {
	T struct{} `form:"t"`
}

type formModelHidden struct {
	hidden string `form:"hidden"`
}

func TestBindFormMoreTypes(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/",
		strings.NewReader("count=5&ratio=1.5&on=true"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c := NewContext(httptest.NewRecorder(), req)
	var m formModelMore
	if err := c.BindForm(&m); err != nil {
		t.Fatalf("BindForm 失败：%v", err)
	}
	if m.Count != 5 || m.Ratio != 1.5 || !m.On {
		t.Errorf("更多类型绑定不符：%+v", m)
	}
}

func TestBindFormTypeErrors(t *testing.T) {
	cases := []struct {
		name string
		body string
		out  any
	}{
		{"uint 非法", "count=x", &formModelMore{}},
		{"float 非法", "ratio=x", &formModelMore{}},
		{"bool 非法", "on=x", &formModelMore{}},
		{"切片非字符串", "ids=1", &formModelBad{}},
		{"类型不支持", "t=x", &formModelUnsupported{}},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		c := NewContext(httptest.NewRecorder(), req)
		if err := c.BindForm(tc.out); err == nil {
			t.Errorf("%s：应返回错误", tc.name)
		}
	}
}

func TestBindFormUnexportedField(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("hidden=x"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c := NewContext(httptest.NewRecorder(), req)
	var m formModelHidden
	if err := c.BindForm(&m); err != nil {
		t.Fatalf("BindForm 失败：%v", err)
	}
	if m.hidden != "" {
		t.Error("未导出字段不应被绑定")
	}
}

func TestBindFormMultipart(t *testing.T) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("name", "多部分")
	_ = mw.WriteField("age", "7")
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	c := NewContext(httptest.NewRecorder(), req)
	var m formModel
	if err := c.BindForm(&m); err != nil {
		t.Fatalf("BindForm 失败：%v", err)
	}
	if m.Name != "多部分" || m.Age != 7 {
		t.Errorf("multipart 绑定不符：%+v", m)
	}
}

func TestBindFormErrors(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("age=abc"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c := NewContext(httptest.NewRecorder(), req)
	var m formModel
	if err := c.BindForm(&m); err == nil {
		t.Error("非法数值应报错")
	}
	if err := bindValues(nil, url.Values{}, "form"); err == nil {
		t.Error("nil 目标应报错")
	}
	if err := bindValues(m, url.Values{}, "form"); err == nil {
		t.Error("非指针目标应报错")
	}
}

func TestBindFormMaxBytes(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/",
		strings.NewReader("name="+strings.Repeat("x", 100)))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	c := NewContext(rec, req)
	c.SetMaxBodyBytes(10)
	var m formModel
	if err := c.BindForm(&m); err == nil {
		t.Error("超过最大请求体应报错")
	}
}

func TestBindFormMalformedMultipart(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("这不是 multipart 内容"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	c := NewContext(httptest.NewRecorder(), req)
	var m formModel
	if err := c.BindForm(&m); err == nil {
		t.Error("损坏的 multipart 应报错")
	}
}

type queryModel struct {
	Keyword string   `query:"keyword"`
	Page    int      `query:"page"`
	Limit   uint     `query:"limit"`
	Ratio   float64  `query:"ratio"`
	Active  bool     `query:"active"`
	Tags    []string `query:"tags"`
	Skip    string   `query:"-"`
}

func TestBindQuery(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?keyword=webx&page=3&limit=10&ratio=1.5&active=true&tags=a&tags=b&skip=1", nil)
	c := NewContext(httptest.NewRecorder(), req)
	var m queryModel
	if err := c.BindQuery(&m); err != nil {
		t.Fatalf("BindQuery 失败：%v", err)
	}
	if m.Keyword != "webx" || m.Page != 3 || m.Limit != 10 || m.Ratio != 1.5 || !m.Active {
		t.Errorf("查询绑定不符：%+v", m)
	}
	if len(m.Tags) != 2 || m.Tags[1] != "b" || m.Skip != "" {
		t.Errorf("切片/忽略字段不符：%+v", m)
	}
}

type queryModelBad struct {
	IDs []int `query:"ids"`
}

type queryModelUnsupported struct {
	T struct{} `query:"t"`
}

type queryModelHidden struct {
	hidden string `query:"hidden"`
}

func TestBindQueryErrors(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		out  any
	}{
		{"int 非法", "page=x", &queryModel{}},
		{"uint 非法", "limit=x", &queryModel{}},
		{"float 非法", "ratio=x", &queryModel{}},
		{"bool 非法", "active=x", &queryModel{}},
		{"切片非字符串", "ids=1", &queryModelBad{}},
		{"类型不支持", "t=x", &queryModelUnsupported{}},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(http.MethodGet, "/?"+tc.raw, nil)
		c := NewContext(httptest.NewRecorder(), req)
		if err := c.BindQuery(tc.out); err == nil {
			t.Errorf("%s：应返回错误", tc.name)
		}
	}
	if err := bindValues(nil, url.Values{}, "query"); err == nil {
		t.Error("nil 目标应报错")
	}
}

func TestBindQueryUnexportedField(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/?hidden=x", nil)
	c := NewContext(httptest.NewRecorder(), req)
	var m queryModelHidden
	if err := c.BindQuery(&m); err != nil {
		t.Fatalf("BindQuery 失败：%v", err)
	}
	if m.hidden != "" {
		t.Error("未导出字段不应被绑定")
	}
}
