package middleware

import (
	"encoding/binary"
	"fmt"
	"math/rand/v2"

	"github.com/lcylpzls/webx/internal/core"
)

// RequestID 返回请求 ID 生成中间件。
// 优先使用请求头 X-Request-ID，否则生成 UUID v4。
func RequestID() core.HandlerFunc {
	return func(c *core.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = newUUIDV4()
		}
		c.Set("requestId", requestID)
		c.Header("X-Request-ID", requestID)
		// 出站透传：转发到上游时保留同一条请求 ID。
		c.Request().Header.Set("X-Request-ID", requestID)
		c.Next()
	}
}

// newUUIDV4 自研 UUID v4 生成器，避免引入第三方 UUID 依赖。
func newUUIDV4() string {
	var b [16]byte
	binary.BigEndian.PutUint64(b[0:8], rand.Uint64())
	binary.BigEndian.PutUint64(b[8:16], rand.Uint64())
	b[6] = (b[6] & 0x0f) | 0x40 // 版本 4
	b[8] = (b[8] & 0x3f) | 0x80 // 变体 RFC 4122
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
