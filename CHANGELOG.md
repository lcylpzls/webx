# Changelog

本项目遵循语义化版本（SemVer）。值得记录的变更统一维护在此文件。

## [Unreleased]

### 规划

- 项目立项：基于标准库实现与 ginx 对等的 HTTP/HTTPS 服务组件库；
- 技术栈确定：net/http + logx + errx + confx + quic-go（唯一直接第三方依赖）；
- 文档阶段：架构、API 草案、迭代计划、ADR 已就绪。

### 实现

- 自研 `Context` 与中间件链（Next/Abort），零第三方类型泄漏；
- 基于 `http.ServeMux` 的路由：gin 风格 `:id`/`*filepath` 自动翻译、
  404/405 标准化 JSON、尾斜杠重定向、路由冲突安全报错；
- 多通道监听：HTTP/2（TLS）、HTTP/3（QUIC）、Unix Socket，支持启动回滚；
- 内置中间件：Recovery / RequestID / Timeout / CORS / Validation /
  RateLimit / AccessLog；
- 标准化响应信封、健康检查、静态文件与 SPA 回退、优雅关闭；
- 配置经 confx 从 TOML 加载，错误统一 errx，日志接入 logx；
- 质量：三包语句覆盖率 100%、fuzz 三个目标、CI 三平台矩阵（待仓库推送后生效）。
