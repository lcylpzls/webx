package webx

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/lcylpzls/webx/v2/internal/core"
)

// healthData 健康检查响应中的 data 字段。
type healthData struct {
	Status  string            `json:"status"`
	Uptime  string            `json:"uptime"`
	Started string            `json:"started"`
	Checks  map[string]string `json:"checks,omitempty"`
}

// healthCheck 是用户注册的健康检查项。
type healthCheck struct {
	name string
	fn   func(context.Context) error
}

// healthHandler 返回健康检查处理器。
// shuttingDown 返回 true 时（就绪探针场景）直接返回 503，表示服务关闭中。
func healthHandler(startTime time.Time, checks []healthCheck, shuttingDown func() bool) HandlerFunc {
	return func(c *core.Context) {
		uptime := time.Since(startTime)
		data := healthData{
			Status:  "运行中",
			Uptime:  formatUptime(uptime),
			Started: startTime.Format(time.RFC3339),
		}
		if shuttingDown != nil && shuttingDown() {
			data.Status = "关闭中"
			c.JSONResponse(http.StatusServiceUnavailable, "服务关闭中", data)
			return
		}
		allOK := true
		if len(checks) > 0 {
			data.Checks = make(map[string]string, len(checks))
			for _, check := range checks {
				if err := check.fn(c.Request().Context()); err != nil {
					allOK = false
					data.Checks[check.name] = err.Error()
					continue
				}
				data.Checks[check.name] = "ok"
			}
		}
		if !allOK {
			data.Status = "异常"
			c.JSONResponse(http.StatusServiceUnavailable, "健康检查未通过", data)
			return
		}
		c.Success("ok", data)
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
