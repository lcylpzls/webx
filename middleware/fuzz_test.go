package middleware

import "testing"

func FuzzRateLimit(f *testing.F) {
	f.Add("1.2.3.4", "10.0.0.0/8")
	f.Add("bad-ip", "bad-cidr")

	f.Fuzz(func(t *testing.T, ip, cidr string) {
		rl := NewRateLimiter(5, 1, []string{cidr})
		_ = rl.Allow(ip)
	})
}
