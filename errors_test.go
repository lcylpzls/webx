package webx

import (
	"testing"

	"github.com/lcylpzls/errx"
	"github.com/lcylpzls/testx"
)

func TestErrorCodesRegistered(t *testing.T) {
	cases := map[errx.Code]string{
		CodeConfigInvalid:    "webx 配置校验失败",
		CodeConfigLoadFailed: "webx 配置文件加载失败",
		CodeListenFailed:     "webx 监听器创建失败",
		CodeStartFailed:      "webx 服务启动失败",
		CodeShutdownFailed:   "webx 优雅关闭失败",
		CodePanic:            "webx 请求处理发生 panic",
	}
	for code, desc := range cases {
		testx.RequireEqual(t, errx.Describe(code), desc)
	}
}
