//go:build windows

package webx

import (
	"fmt"
	"syscall"
	"unsafe"
)

// minWindowsBuild 是支持 AF_UNIX 的最低 Windows build 号（1803）。
const minWindowsBuild = 17134

// getWindowsBuild 通过 RtlGetNtVersionNumbers 获取真实 Windows build 号。
func getWindowsBuild() int {
	ntdll := syscall.NewLazyDLL("ntdll.dll")
	proc := ntdll.NewProc("RtlGetNtVersionNumbers")
	var major, minor, build uint32
	proc.Call(
		uintptr(unsafe.Pointer(&major)),
		uintptr(unsafe.Pointer(&minor)),
		uintptr(unsafe.Pointer(&build)),
	)
	return int(build & 0xFFFF)
}

// unixSocketSupported 检查 Windows 是否支持 AF_UNIX，不支持时返回错误。
func unixSocketSupported() error {
	return checkWindowsBuild(getWindowsBuild())
}

// checkWindowsBuild 判断 build 号是否满足 AF_UNIX 最低要求。
func checkWindowsBuild(build int) error {
	if build < minWindowsBuild {
		return fmt.Errorf(
			"webx：当前 Windows 系统版本过低（build %d），不支持 Unix Socket 监听，"+
				"需要 Windows 10 build 1803 (10.0.17134) 或更高版本",
			build,
		)
	}
	return nil
}
