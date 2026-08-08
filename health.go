package webx

import (
	"fmt"
	"time"

	"github.com/lcylpzls/webx/internal/core"
)

// healthData 健康检查响应中的 data 字段。
type healthData struct {
	Status  string `json:"status"`
	Uptime  string `json:"uptime"`
	Started string `json:"started"`
}

// healthHandler 返回健康检查处理器。
func healthHandler(startTime time.Time) HandlerFunc {
	return func(c *core.Context) {
		uptime := time.Since(startTime)
		c.Success("ok", healthData{
			Status:  "运行中",
			Uptime:  formatUptime(uptime),
			Started: startTime.Format(time.RFC3339),
		})
	}
}

// formatUptime 将 duration 格式化为中文运行时间。
func formatUptime(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d秒", int(d.Seconds()))
	}
	if d < time.Hour {
		m := int(d.Minutes())
		s := int(d.Seconds()) % 60
		if s > 0 {
			return fmt.Sprintf("%d分钟%d秒", m, s)
		}
		return fmt.Sprintf("%d分钟", m)
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if m > 0 {
		return fmt.Sprintf("%d小时%d分钟", h, m)
	}
	return fmt.Sprintf("%d小时", h)
}
