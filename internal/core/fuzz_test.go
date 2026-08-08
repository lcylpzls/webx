package core

import (
	"bytes"
	"net/http/httptest"
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
