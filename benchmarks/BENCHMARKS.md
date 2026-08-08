# webx 性能对比基准

## 环境

- 操作系统：Windows 10（10.0.19045）
- CPU：AMD Ryzen 7000 系列（Zen 4，6 核 12 线程）
- Go：1.26.5 windows/amd64
- 日期：2026-08-08
- 命令：`go test -bench . -benchmem -benchtime=2s -count=3 -run '^$' .`
- 数据：每组取 3 轮最优值

## 一、路由分发基准（内存内，不含网络/TLS）

同一套 500 条参数化路由（`/api/v1/resource/{n}/:id`），请求
`GET /api/v1/resource/25/42`，Handler 均写入固定文本 `hello`。

| 框架 | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| net/http ServeMux（标准库参照） | 149.8 | 32 | 1 |
| webx | 283.4 | 703 | 5 |
| gin | 86.3 | 58 | 1 |
| echo | 90.7 | 18 | 1 |
| fasthttprouter | 175.0 | 56 | 3 |

说明：webx 每次分发包含上下文池获取/归还、路由参数 map 构建与中间件链
装配，分配数（5 allocs）高于 gin/echo；单次分发耗时仍在同数量级
（约 0.28µs），对真实服务影响可忽略。

## 二、端到端服务基准（TLS + HTTP/1.1 keep-alive，12 并发）

四者均为本地回环 TLS 服务，Handler 写入 `hello`；客户端统一使用
`net/http` keep-alive 连接池（并行预热后计时）。

| 框架 | ns/op | 约合 req/s | B/op | allocs/op |
| --- | ---: | ---: | ---: | ---: |
| webx | 9 879 | ≈ 101k | 5 851 | 67 |
| gin | 9 958 | ≈ 100k | 5 753 | 66 |
| echo | 10 506 | ≈ 95k | 5 729 | 66 |
| fasthttp | 7 417 | ≈ 135k | 3 277 | 46 |

说明：在 TLS + keep-alive 的真实请求路径上，webx 与 gin 基本持平，
略快于 echo；fasthttp 因零拷贝架构领先约 35%。

## 注意事项

- webx 设计上**强制 TLS**（生产默认 HTTP/2 + HTTP/3），本基准统一走
  TLS + HTTP/1.1，未测 HTTP/2/HTTP/3；
- 路由基准只衡量内存内分发，不反映网络、TLS、序列化与中间件开销；
- 服务基准受操作系统、杀软扫描、CPU 频率波动影响，数值仅供参考；
- 生产级压测请使用 wrk / hey / k6 等真实负载工具，并以多机部署数据为准。

## 复现

```powershell
cd benchmarks
go test -bench . -benchmem -benchtime=2s -count=3 -run '^$' .
```
