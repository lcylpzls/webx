//go:build !windows

package webx

import (
	"os"

	"github.com/lcylpzls/errx"
)

// maxUnixPathLen 在非 Windows 平台为标准 Unix socket 路径长度限制（108）。
const maxUnixPathLen = 108

// unixSocketSupported 在非 Windows 平台始终支持 Unix Socket。
func unixSocketSupported() error {
	return nil
}

// validateUnixPath 在非 Windows 平台校验标准 Unix socket 路径长度限制。
func validateUnixPath(path string) error {
	if len(path) > maxUnixPathLen {
		return errx.NewCodef(CodeListenFailed,
			"webx：Unix Socket 路径过长（%d＞%d 字节），标准 Unix 路径限制 ≤108 字节",
			len(path), maxUnixPathLen)
	}
	return nil
}

// shortUnixDir 返回短临时目录（非 Windows 下直接返回 os.TempDir()）。
func shortUnixDir() string {
	return os.TempDir()
}
