package middleware

import (
	"net/http"

	"github.com/lcylpzls/errx"
	"github.com/lcylpzls/validx"
	"github.com/lcylpzls/webx/internal/core"
)

const maxRequestBodyBytes = 10 * 1024 * 1024

// validationInput 是 HTTP 请求语义校验输入。
type validationInput struct {
	method      string
	contentType string
	contentLen  int64
}

// init 注册 HTTP 请求语义校验规则到 validx 全局规则表。
func init() {
	_ = validx.RegisterRule("webx_http_request", func(value any, param, path string) error {
		// 内部调用保证 value 为 validationInput。
		in := value.(validationInput)
		if in.method == http.MethodGet || in.method == http.MethodHead || in.method == http.MethodOptions {
			return nil
		}
		if in.contentType == "" {
			return nil
		}
		if !isJSONContentType(in.contentType) && !isMultipartForm(in.contentType) {
			return errx.NewCode(validx.CodeValidationFailed, "请求参数校验失败：Content-Type 必须为 application/json 或 multipart/form-data")
		}
		if in.contentLen > maxRequestBodyBytes {
			return errx.NewCode(validx.CodeValidationFailed, "请求参数校验失败：请求体过大")
		}
		return nil
	})
}

// Validation 返回请求参数校验中间件：校验 Content-Type 是否为 JSON/
// multipart、Content-Length 是否超过 10MB（判定统一走 validx 规则）。
func Validation() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c := core.From(r.Context())
			err := validx.ValidateField(validationInput{
				method:      r.Method,
				contentType: r.Header.Get(canonicalContentType),
				contentLen:  r.ContentLength,
			}, "webx_http_request")
			if err != nil {
				c.AbortWithStatusJSON(http.StatusBadRequest, err.Error(), nil)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// isJSONContentType 检查 Content-Type 是否为 JSON 类型。
func isJSONContentType(ct string) bool {
	if i := stringsIndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	return ct == "application/json"
}

// isMultipartForm 检查 Content-Type 是否为 multipart/form-data。
func isMultipartForm(ct string) bool {
	if i := stringsIndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	return ct == "multipart/form-data"
}

// stringsIndexByte 返回字节在字符串中的位置，避免额外导入 strings。
func stringsIndexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}
