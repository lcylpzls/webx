# webx API 设计定稿（v1.0.0）

> 本文是评审稿的定稿记录。所有 API 决策点已确认并冻结；
> 最终签名以 [api-v1.0.0.md](api-v1.0.0.md) 为准。

## 决策记录（原 D-1 ~ D-7）

| 编号 | 问题 | 结论 |
| --- | --- | --- |
| D-1 | Handler 签名 | `func(*webx.Context)`，不依赖第三方框架类型 |
| D-2 | 路由写法 | 对外兼容 `:id` / `*filepath`，内部翻译为 `{id}` / `{path...}` |
| D-3 | HTTP/3 默认 | 默认关闭，`UseHttp3Listen` 显式开启 |
| D-4 | 配置来源 | Config 直接构造 + `LoadConfig`（TOML）都支持 |
| D-5 | 日志抽象 | 直接使用 logx.Logger，不另设接口 |
| D-6 | 响应 code | 沿用 int（0=成功，HTTP 语义错误码） |
| D-7 | AccessLog | 提供可选的访问日志中间件 |

## 定稿说明

- 路由基于自研 radix 匹配树（v0.17 起替代 ServeMux），匹配与分发完全自主；
- 内置中间件顺序默认固定但可通过 `SetMiddlewareOrder` 调整；
- 全部公开 API 以每版本生成的 `docs/api-vX.Y.Z.md` 基线为准；
- v1.0.0 起 API 冻结，后续变更需通过 apidiff 与基线比对。
