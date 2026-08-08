//go:build windows

package webx

import "testing"

func TestCheckWindowsBuild(t *testing.T) {
	if err := checkWindowsBuild(20000); err != nil {
		t.Errorf("高版本 build 应通过：%v", err)
	}
	if err := checkWindowsBuild(10000); err == nil {
		t.Error("过低 build 应报错")
	}
}
