package webx

import (
	"errors"
	testx "github.com/lcylpzls/testx"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lcylpzls/errx"
	"github.com/lcylpzls/webx/v2/internal/core"
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
	testx.Equal(t, rec.Code, http.StatusNotFound)

	if !strings.Contains(rec.Body.String(), "USER_NOT_FOUND") {
		t.Errorf("响应体应含错误码：%s", rec.Body.String())
	}
}

func TestRespondErrorWithData(t *testing.T) {
	rec := httptest.NewRecorder()
	c := core.NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	RespondErrorWithData(c, errx.New(errx.KindNotFound, "USER_NOT_FOUND", "用户不存在"),
		map[string]string{"hint": "检查参数"})
	testx.Equal(t, rec.Code, http.StatusNotFound)

	if !strings.Contains(rec.Body.String(), "hint") || !strings.Contains(rec.Body.String(), "检查参数") {
		t.Errorf("响应体应含业务数据：%s", rec.Body.String())
	}
}
