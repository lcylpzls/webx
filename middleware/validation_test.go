package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lcylpzls/webx/internal/core"
)

func runValidation(t *testing.T, req *http.Request) (*httptest.ResponseRecorder, bool) {
	t.Helper()
	rec := httptest.NewRecorder()
	var reached bool
	c := core.NewContext(rec, req)
	c.SetHandlers([]core.HandlerFunc{
		Validation(),
		func(c *core.Context) { reached = true; c.Success("ok", nil) },
	})
	c.Run()
	return rec, reached
}

func TestValidationSkipsBodylessMethods(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		rec, reached := runValidation(t, httptest.NewRequest(method, "/", nil))
		if !reached || rec.Code != http.StatusOK {
			t.Errorf("%s 应直接放行：%v %d", method, reached, rec.Code)
		}
	}
}

func TestValidationNoContentType(t *testing.T) {
	rec, reached := runValidation(t, httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("{}")))
	if !reached || rec.Code != http.StatusOK {
		t.Errorf("无 Content-Type 应放行：%v %d", reached, rec.Code)
	}
}

func TestValidationWrongContentType(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("a=1"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec, reached := runValidation(t, req)
	if reached || rec.Code != http.StatusBadRequest {
		t.Errorf("错误 Content-Type 应 400：%v %d", reached, rec.Code)
	}
	if !contains(rec.Body.String(), "Content-Type 必须为 application/json 或 multipart/form-data") {
		t.Errorf("400 消息不符：%s", rec.Body.String())
	}
}

func TestValidationBodyTooLarge(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.ContentLength = 11 * 1024 * 1024
	rec, reached := runValidation(t, req)
	if reached || rec.Code != http.StatusBadRequest {
		t.Errorf("超大请求体应 400：%v %d", reached, rec.Code)
	}
}

func TestValidationPasses(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"a":1}`))
	req.Header.Set("Content-Type", "application/json")
	rec, reached := runValidation(t, req)
	if !reached || rec.Code != http.StatusOK {
		t.Errorf("合法 JSON 应放行：%v %d", reached, rec.Code)
	}
}

func TestValidationMultipart(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("a=1"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	rec, reached := runValidation(t, req)
	if !reached || rec.Code != http.StatusOK {
		t.Errorf("合法 multipart 应放行：%v %d", reached, rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("x"))
	req.Header.Set("Content-Type", "multipart/form-data")
	req.ContentLength = 11 * 1024 * 1024
	rec, reached = runValidation(t, req)
	if reached || rec.Code != http.StatusBadRequest {
		t.Errorf("超大 multipart 应 400：%v %d", reached, rec.Code)
	}
}

func TestIsJSONContentType(t *testing.T) {
	if !isJSONContentType("application/json") || !isJSONContentType("application/json; charset=utf-8") {
		t.Error("JSON Content-Type 判定不符")
	}
	if isJSONContentType("text/plain") {
		t.Error("非 JSON Content-Type 判定不符")
	}
	if !isMultipartForm("multipart/form-data") || !isMultipartForm("multipart/form-data; boundary=x") {
		t.Error("multipart Content-Type 判定不符")
	}
	if isMultipartForm("application/json") {
		t.Error("非 multipart 判定不符")
	}
}
