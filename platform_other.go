//go:build !windows

package webx

// unixSocketSupported 在非 Windows 平台始终支持 Unix Socket。
func unixSocketSupported() error {
	return nil
}
