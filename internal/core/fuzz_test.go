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
