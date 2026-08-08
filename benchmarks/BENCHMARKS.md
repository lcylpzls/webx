# webx 性能对比基准

## 环境

- 操作系统：Windows 10（10.0.19045）
- CPU：AMD Ryzen 7000 系列（Zen 4，6 核 12 线程）
- Go：1.26.5 windows/amd64
- 日期：2026-08-08
- 命令：`go test -bench . -benchmem -benchtime=1s -count=3 -cpu=1,12 -run '^$' .`
- 数据：单核（`-cpu=1`）与多核（`-cpu=12`）各跑 3 轮，取每组最优值

## 公平性约定

- **全部框架服务端均使用 HTTPS（TLS 1.2，HTTP/1.1 keep-alive）**；
  webx 只支持 HTTPS，因此不引入任何明文 HTTP 对比；
- 客户端统一使用 `net/http` 连接池，并行预热建立连接后再计时；
- Handler 行为一致：路由基准写固定文本 `hello`，服务基准写固定文本
  `hello`；中间件均不启用（webx 除必选路径外无中间件）；
- 路由基准为内存内分发（无网络协议），不含 http/https 差异；
- 服务基准为本地回环，数值受操作系统、杀软扫描与 CPU 频率波动影响。

## 一、路由分发基准（内存内，不含网络/TLS）

同一套 500 条参数化路由（`/api/v1/resource/{n}/:id`），请求
`GET /api/v1/resource/25/42`。

### 单核（GOMAXPROCS=1）

| 框架 | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| net/http ServeMux（标准库参照） | 150.9 | 33 | 1 |
| webx | 343.0 | 706 | 5 |
| gin | 96.6 | 59 | 1 |
| echo | 92.5 | 19 | 1 |
| fasthttprouter | 288.7 | 56 | 3 |

### 多核（GOMAXPROCS=12）

| 框架 | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| net/http ServeMux（标准库参照） | 147.6 | 32 | 1 |
| webx | 350.6 | 707 | 5 |
| gin | 84.7 | 58 | 1 |
| echo | 90.7 | 18 | 1 |
| fasthttprouter | 177.1 | 56 | 3 |

说明：webx 每次分发包含上下文池获取/归还、路由参数 map 构建与中间件链
装配，分配数（5 allocs）高于 gin/echo；单次分发约 0.34µs，仍在同数量级。

## 二、端到端服务基准（HTTPS + keep-alive）

四者均为本地回环 **HTTPS** 服务（TLS 1.2 + HTTP/1.1 keep-alive），
Handler 写入 `hello`；`-cpu=1` 时 RunParallel 单 worker（等效顺序压测），
`-cpu=12` 时 12 并发。

### 单核（GOMAXPROCS=1）

| 框架 | ns/op | 约合 req/s | B/op | allocs/op |
| --- | ---: | ---: | ---: | ---: |
| webx | 37 019 | ≈ 27.0k | 5 554 | 66 |
| gin | 35 995 | ≈ 27.8k | 5 522 | 65 |
| echo | 36 039 | ≈ 27.7k | 5 498 | 66 |
| fasthttp | 30 390 | ≈ 32.9k | 3 264 | 46 |

### 多核（GOMAXPROCS=12，12 并发）

| 框架 | ns/op | 约合 req/s | B/op | allocs/op |
| --- | ---: | ---: | ---: | ---: |
| webx | 9 439 | ≈ 106k | 5 769 | 66 |
| gin | 8 954 | ≈ 112k | 5 740 | 66 |
| echo | 9 540 | ≈ 105k | 5 759 | 67 |
| fasthttp | 6 901 | ≈ 145k | 3 270 | 46 |

说明：单核下四者差距在 20% 以内；多核下 webx 与 gin/echo 基本持平，
fasthttp 因零拷贝架构领先约 35%。

## 注意事项

- webx 设计上**强制 TLS**（生产默认 HTTP/2 + HTTP/3），本基准统一走
  HTTPS + HTTP/1.1，未测 HTTP/2/HTTP/3 差异；
- 路由基准只衡量内存内分发，不反映网络、TLS、序列化与中间件开销；
- 生产级压测请使用 wrk / hey / k6 等真实负载工具，并以多机部署数据为准。

## 复现

```powershell
cd benchmarks
go test -bench . -benchmem -benchtime=1s -count=3 -cpu=1,12 -run '^$' .
```
