//go:build windows

package webx

import (
	"net"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"github.com/lcylpzls/errx"
)

// minWindowsBuild 是支持 AF_UNIX 的最低 Windows build 号（1803）。
const minWindowsBuild = 17134

// getWindowsBuild 通过 RtlGetNtVersionNumbers 获取真实 Windows build 号（可注入测试）。
var getWindowsBuild = func() int {
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

// unixSocketSupported 检查 Windows 是否支持 AF_UNIX。
// 先检查系统版本号，再通过实际 bind/unlink 探测确认可用
// （Windows AF_UNIX 的 sockaddr_un 限制路径 ≤63 wchar_t，
// TempDir 过长时 bind 会失败，探测可捕获此类场景）。
func unixSocketSupported() error {
	if err := checkWindowsBuild(getWindowsBuild()); err != nil {
		return err
	}
	return probeUnixSocket()
}

// probeUnixSocket 尝试在短路径上 bind/unlink 以探测 AF_UNIX 是否可用。
// Windows AF_UNIX 的 sockaddr_un 限制路径 ≤63 wchar_t，os.TempDir() 可能被
// 容器/会话重写为长路径；依次尝试多个候选，有一个成功即认为系统支持。
func probeUnixSocket() error {
	candidates := unixProbeCandidates()
	for _, probe := range candidates {
		ln, err := net.Listen("unix", probe)
		if err == nil {
			ln.Close()
			os.Remove(probe)
			return nil
		}
	}
	return errx.NewCodef(CodeListenFailed,
		"webx：当前系统 AF_UNIX 不可用（已尝试 %d 个候选路径均失败），"+
			"请将临时目录移至短路径或使用 Windows 10 1803+", len(candidates))
}

// unixProbeCandidates 返回探测 AF_UNIX 的候选短路径（按优先级，可注入测试）。
var unixProbeCandidates = func() []string {
	shortName := "ux"
	candidates := make([]string, 0, 3)
	if td := os.TempDir(); len(td)+len("/ux") <= maxUnixPathLen {
		candidates = append(candidates, filepath.Join(td, shortName))
	}
	if home := os.Getenv("USERPROFILE"); home != "" && len(home) <= maxUnixPathLen-4 {
		candidates = append(candidates, filepath.Join(home, shortName+".sock"))
	}
	// 最后兜底：系统 Temp 目录（通常 C:\Windows\Temp，约 17 字符）
	if sysRoot := os.Getenv("SystemRoot"); sysRoot != "" {
		p := filepath.Join(sysRoot, "Temp", shortName+".sock")
		if len(p) <= maxUnixPathLen {
			candidates = append(candidates, p)
		}
	}
	return candidates
}

// unixShortDirCandidates 返回短目录候选（可注入测试）。
var unixShortDirCandidates = func() []string {
	return []string{os.TempDir(), os.Getenv("USERPROFILE"), os.Getenv("SystemRoot") + "\\Temp"}
}

// shortUnixDir 返回一个长度足够短的目录（用于测试中创建 Unix Socket）。
// 优先使用 os.TempDir()，过长时回退到 %USERPROFILE%。
func shortUnixDir() string {
	for _, c := range unixShortDirCandidates() {
		if c != "" && len(filepath.Join(c, "w", "webx.sock")) <= maxUnixPathLen {
			return c
		}
	}
	return os.TempDir()
}

// maxUnixPathLen 是 Windows AF_UNIX 路径最大字符数（MS afunix.h wchar_t Path[63]，留 3 字节余量）。
const maxUnixPathLen = 60

// validateUnixPath 校验 Unix Socket 路径（Windows 上检查长度限制）。
func validateUnixPath(path string) error {
	if len(path) > maxUnixPathLen {
		return errx.NewCodef(CodeListenFailed,
			"webx：Unix Socket 路径过长（%d＞%d 字节），Windows AF_UNIX 限制路径 ≤60 字节，"+
				"请缩短路径或使用短目录（如 C:\\Temp\\app.sock）", len(path), maxUnixPathLen)
	}
	return nil
}

// checkWindowsBuild 判断 build 号是否满足 AF_UNIX 最低要求。
func checkWindowsBuild(build int) error {
	if build < minWindowsBuild {
		return errx.NewCodef(
			CodeConfigInvalid,
			"webx：当前 Windows 系统版本过低（build %d），不支持 Unix Socket 监听，"+
				"需要 Windows 10 build 1803 (10.0.17134) 或更高版本",
			build,
		)
	}
	return nil
}
