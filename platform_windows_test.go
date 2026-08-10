//go:build windows

package webx

import (
	"testing"

	"github.com/lcylpzls/testx"
)

func TestCheckWindowsBuild(t *testing.T) {
	testx.RequireNoError(t, checkWindowsBuild(20000))
	testx.RequireError(t, checkWindowsBuild(10000))
}
