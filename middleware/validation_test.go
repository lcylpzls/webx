package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lcylpzls/testx"
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
		testx.RequireTrue(t, reached)
		testx.RequireEqual(t, rec.Code, http.StatusOK)
	}
}

func TestValidationNoContentType(t *testing.T) {
	rec, reached := runValidation(t, httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("{}")))
	testx.RequireTrue(t, reached)
	testx.RequireEqual(t, rec.Code, http.StatusOK)
}

func TestValidationWrongContentType(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("a=1"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec, reached := runValidation(t, req)
	testx.RequireFalse(t, reached)
	testx.RequireEqual(t, rec.Code, http.StatusBadRequest)
	testx.RequireTrue(t, contains(rec.Body.String(), "Content-Type 必须为 application/json 或 multipart/form-data"))
}

func TestValidationBodyTooLarge(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.ContentLength = 11 * 1024 * 1024
	rec, reached := runValidation(t, req)
	testx.RequireFalse(t, reached)
	testx.RequireEqual(t, rec.Code, http.StatusBadRequest)
}

func TestValidationPasses(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"a":1}`))
	req.Header.Set("Content-Type", "application/json")
	rec, reached := runValidation(t, req)
	testx.RequireTrue(t, reached)
	testx.RequireEqual(t, rec.Code, http.StatusOK)
}

func TestValidationMultipart(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("a=1"))
	req.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	rec, reached := runValidation(t, req)
	testx.RequireTrue(t, reached)
	testx.RequireEqual(t, rec.Code, http.StatusOK)

	req = httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("x"))
	req.Header.Set("Content-Type", "multipart/form-data")
	req.ContentLength = 11 * 1024 * 1024
	rec, reached = runValidation(t, req)
	testx.RequireFalse(t, reached)
	testx.RequireEqual(t, rec.Code, http.StatusBadRequest)
}

func TestIsJSONContentType(t *testing.T) {
	testx.RequireTrue(t, isJSONContentType("application/json"))
	testx.RequireTrue(t, isJSONContentType("application/json; charset=utf-8"))
	testx.RequireFalse(t, isJSONContentType("text/plain"))
	testx.RequireTrue(t, isMultipartForm("multipart/form-data"))
	testx.RequireTrue(t, isMultipartForm("multipart/form-data; boundary=x"))
	testx.RequireFalse(t, isMultipartForm("application/json"))
}
