package webx

import "testing"

func FuzzTranslatePattern(f *testing.F) {
	f.Add("/api/users/:id")
	f.Add("/assets/*filepath")
	f.Add("/x/:")
	f.Add("/x/*p/rest")

	f.Fuzz(func(t *testing.T, path string) {
		_, _, _ = translateGinPattern(path)
	})
}
