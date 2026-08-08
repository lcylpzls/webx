# ginx 迁移指南

webx 与 ginx 功能对等，迁移成本集中在 Handler 签名与配置来源。

## 快速对照

| ginx | webx |
| --- | --- |
| `ginx.NewServer(cfg)` | `webx.NewServer(cfg, logger)`（logger 外部注入） |
| `func(c *gin.Context)` | `func(c *webx.Context)` |
| `c.JSON(200, ginx.StandardizedResponse{...})` | `c.Success(msg, data)` / `c.JSONResponse(status, msg, data)` |
| `c.Param("id")` | `c.Param("id")`（路径写法 `:id`/`*filepath` 兼容） |
| `c.GetString("requestId")` | `c.RequestID()` |
| `c.Set/Get` | `c.Set/Get`（语义一致） |
| `c.AbortWithStatusJSON(status, gin.H{...})` | `c.AbortWithStatusJSON(status, msg, data)` |
| `ginx.Config{...}` 代码构造 | `webx.LoadConfig("config.toml")`（confx TOML） |
| `UseHttp2Listen/UseHttp3Listen/UseUnixSocketListen` | 同名方法，行为一致 |
| `RegisterRoute/RegisterRouteGroup` | 同名，语法兼容 |
| `DisableMiddleware/OverrideMiddleware` | 同名 |
| `EnableRateLimit(RateLimitOptions)` | 同名 |
| `ServeStaticDir/ServeStaticFS/EnableSPA` | 同名 |
| `Stop(ctx)` / 信号优雅关闭 | 同名 |

## 迁移步骤

1. `go get github.com/lcylpzls/webx@v0.6.0`，引入 `github.com/lcylpzls/logx`；
2. 构造 logger 并注入：`webx.NewServer(cfg, logger)`；
3. 把 Handler 签名从 `*gin.Context` 改为 `*webx.Context`：
   - `c.JSON(200, resp)` → `c.Success("ok", data)`；
   - `c.JSON(400, gin.H{...})` → `c.Fail(400, code, msg)`；
   - `c.AbortWithStatusJSON(500, gin.H{...})` → `c.AbortWithStatusJSON(500, "服务器内部错误", nil)`；
4. 配置迁移为 TOML（`webx.LoadConfig`），字段名见
   [api-design.md](api-design.md) 的 Config 章节；
5. 路由与分组代码基本原样可用（`:id`、`*filepath` 写法保留）。

## 行为差异

- 日志由调用方注入 logx，不再使用 ginx 的 Logger 接口；
- 错误统一 errx，配置/启动失败返回结构化错误码；
- HTTP/3 默认关闭，需显式 `UseHttp3Listen`；
- 未启用对应中间件时，`requestId` 为空字符串（与 ginx 一致）。
