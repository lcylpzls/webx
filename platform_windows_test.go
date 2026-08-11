//go:build windows

package webx

import (
	"os"
	"strings"
	"testing"
)

// TestProbeUnixSocketFailure 覆盖 AF_UNIX 探测全部候选失败分支。
func TestProbeUnixSocketFailure(t *testing.T) {
	orig := unixProbeCandidates
	defer func() { unixProbeCandidates = orig }()

	unixProbeCandidates = func() []string { return nil }
	if err := probeUnixSocket(); err == nil {
		t.Fatal("全部候选失败应报错")
	}
}

// TestCheckWindowsBuild 覆盖 Windows build 号边界。
func TestCheckWindowsBuild(t *testing.T) {
	if err := checkWindowsBuild(minWindowsBuild - 1); err == nil {
		t.Fatal("低于 1803 应报错")
	}
	if err := checkWindowsBuild(minWindowsBuild); err != nil {
		t.Fatalf("1803 及以上应通过：%v", err)
	}
}

// TestUnixSocketSupportedLowBuild 覆盖版本检查失败分支。
func TestUnixSocketSupportedLowBuild(t *testing.T) {
	orig := getWindowsBuild
	defer func() { getWindowsBuild = orig }()

	getWindowsBuild = func() int { return minWindowsBuild - 1 }
	if err := unixSocketSupported(); err == nil {
		t.Fatal("低版本应报错")
	}
}

// TestShortUnixDirFallback 覆盖短目录候选全部超长时的兜底分支。
func TestShortUnixDirFallback(t *testing.T) {
	orig := unixShortDirCandidates
	defer func() { unixShortDirCandidates = orig }()

	unixShortDirCandidates = func() []string {
		return []string{
			strings.Repeat("x", 200),
			strings.Repeat("y", 200),
			strings.Repeat("z", 200),
		}
	}
	if got := shortUnixDir(); got == "" {
		t.Fatal("兜底目录不应为空")
	}
}

// TestUnixShortDirCandidatesNoSystemRoot 覆盖 SystemRoot 为空时不生成 "\\Temp" 候选。
func TestUnixShortDirCandidatesNoSystemRoot(t *testing.T) {
	orig := os.Getenv("SystemRoot")
	os.Setenv("SystemRoot", "")
	defer os.Setenv("SystemRoot", orig)

	got := unixShortDirCandidates()
	if len(got) != 2 {
		t.Fatalf("SystemRoot 为空时应只有 2 个候选：%v", got)
	}
	for _, c := range got {
		if c == "\\Temp" {
			t.Fatalf("不应出现 \"\\Temp\" 候选：%v", got)
		}
	}
}
