package webx

import "github.com/lcylpzls/errx"

// StatusForError 返回 errx 错误对应的 HTTP 状态码；非 errx 错误返回 500。
func StatusForError(err error) int {
	return errx.KindHTTPStatus(errx.KindOf(err))
}

// RespondError 将 errx 错误映射为标准化错误响应。
// 状态码由 Kind 映射（如 KindNotFound → 404），响应体为统一 JSON 信封。
func RespondError(c *Context, err error) {
	status := StatusForError(err)
	c.Fail(status, status, err.Error())
}
