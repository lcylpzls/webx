package webx

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lcylpzls/errx"
	"github.com/lcylpzls/webx/internal/core"
)

func TestStatusForError(t *testing.T) {
	if got := StatusForError(errx.New(errx.KindNotFound, "N", "n")); got != http.StatusNotFound {
		t.Errorf("errx 状态映射不符：%d", got)
	}
	if got := StatusForError(errx.New(errx.KindUnavailable, "U", "u")); got != http.StatusServiceUnavailable {
		t.Errorf("errx 状态映射不符：%d", got)
	}
	if got := StatusForError(errors.New("普通错误")); got != http.StatusInternalServerError {
		t.Errorf("普通错误应 500：%d", got)
	}
}

func TestRespondError(t *testing.T) {
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	RespondError(c, errx.New(errx.KindNotFound, "USER_NOT_FOUND", "用户不存在"))
	if rec.Code != http.StatusNotFound {
		t.Errorf("状态码不符：%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "USER_NOT_FOUND") {
		t.Errorf("响应体应含错误码：%s", rec.Body.String())
	}
}
