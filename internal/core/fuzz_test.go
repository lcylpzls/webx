package core

import (
	"bytes"
	"net/http/httptest"
	"strings"
	"testing"
)

func FuzzBindJSON(f *testing.F) {
	f.Add([]byte(`{"name":"webx","port":8080}`))
	f.Add([]byte(`{`))
	f.Add([]byte(``))

	f.Fuzz(func(t *testing.T, data []byte) {
		var cfg struct {
			Name string `json:"name"`
			Port int    `json:"port"`
		}
		c := NewContext(httptest.NewRecorder(),
			httptest.NewRequest("POST", "/", bytes.NewReader(data)))
		_ = c.BindJSON(&cfg)
	})
}

func FuzzBindForm(f *testing.F) {
	f.Add("name=x&age=1")
	f.Add("count=1&ratio=1.5&on=true")
	f.Add("bad=%zz")

	f.Fuzz(func(t *testing.T, body string) {
		req := httptest.NewRequest("POST", "/", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		c := NewContext(httptest.NewRecorder(), req)
		var out struct {
			Name string `form:"name"`
			Age  int    `form:"age"`
		}
		_ = c.BindForm(&out)
	})
}

func FuzzBindQuery(f *testing.F) {
	f.Add("page=1&limit=10&active=true&tags=a&tags=b")
	f.Add("page=x&active=x")
	f.Add("bad=%zz")

	f.Fuzz(func(t *testing.T, raw string) {
		req := httptest.NewRequest("GET", "/", nil)
		req.URL.RawQuery = raw // 直接注入原始 query，避免请求行解析 panic
		c := NewContext(httptest.NewRecorder(), req)
		var out struct {
			Page   int      `query:"page"`
			Active bool     `query:"active"`
			Tags   []string `query:"tags"`
		}
		_ = c.BindQuery(&out)
	})
}
