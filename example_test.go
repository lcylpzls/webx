package webx_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/lcylpzls/errx"
	"github.com/lcylpzls/webx/v2"
)

func ExampleRespondError() {
	rec := httptest.NewRecorder()
	c := webx.NewContext(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	webx.RespondError(c, errx.New(errx.KindNotFound, "USER_NOT_FOUND", "用户不存在"))
	fmt.Println(rec.Code)
	// Output:
	// 404
}

func ExampleStatusForError() {
	err := errx.New(errx.KindForbidden, "NO_PERMISSION", "无权限")
	fmt.Println(webx.StatusForError(err))
	// Output:
	// 403
}

func ExampleNewRouter() {
	rt := webx.NewRouter(webx.NoRouteHandler, webx.NoMethodHandler)
	_ = rt.Handle("GET", "/ping", []webx.HandlerFunc{
		func(c *webx.Context) { c.Success("pong", nil) },
	})
	rec := httptest.NewRecorder()
	rt.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ping", nil))
	fmt.Println(rec.Code)
	// Output:
	// 200
}
