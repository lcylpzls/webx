package middleware

import (
	"net/http"

	"github.com/lcylpzls/webx/internal/core"
)

// stdToCore 将标准中间件适配为 core.HandlerFunc，供既有测试链使用。
func stdToCore(mw func(http.Handler) http.Handler) core.HandlerFunc {
	return func(c *core.Context) {
		r := c.Request().WithContext(core.NewContextWith(c.Request().Context(), c))
		c.SetRequest(r)
		called := false
		var next http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r2 *http.Request) {
			called = true
			c.SetWriter(w)
			c.SetRequest(r2)
			c.Next()
		})
		mw(next).ServeHTTP(c.Writer(), r)
		if !called {
			c.Abort()
		}
	}
}
