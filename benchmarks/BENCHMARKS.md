# webx 性能对比基准

## 环境

- 操作系统：Windows 10（10.0.19045）
- CPU：AMD Ryzen 7000 系列（Zen 4，6 核 12 线程）
- Go：1.26.5 windows/amd64
- 日期：2026-08-08
- webx 版本：P0 零分配改造后的 main 分支（v1.1.0 候选）
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
| net/http ServeMux（标准库参照） | 143.1 | 32 | 1 |
| webx | 100.6 | 12 | **0** |
| gin | 89.2 | 59 | 1 |
| echo | 89.1 | 18 | 1 |
| fasthttprouter | 202.7 | 56 | 3 |

### 多核（GOMAXPROCS=12）

| 框架 | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| net/http ServeMux（标准库参照） | 142.9 | 32 | 1 |
| webx | 100.4 | 11 | **0** |
| gin | 83.0 | 58 | 1 |
| echo | 88.7 | 18 | 1 |
| fasthttprouter | 171.7 | 56 | 3 |

说明：P0 零分配改造后（路由参数槽化、Context 池复用），webx 路由分发
达到 **0 allocs/op**，耗时约 0.10µs，与 gin/echo 差距缩小到 15% 以内，
并快于标准库 ServeMux 与 fasthttprouter。

## 二、端到端服务基准（HTTPS + keep-alive）

四者均为本地回环 **HTTPS** 服务（TLS 1.2 + HTTP/1.1 keep-alive），
Handler 写入 `hello`；`-cpu=1` 时 RunParallel 单 worker（等效顺序压测），
`-cpu=12` 时 12 并发。

### 单核（GOMAXPROCS=1）

| 框架 | ns/op | 约合 req/s | B/op | allocs/op |
| --- | ---: | ---: | ---: | ---: |
| webx | 35 060 | ≈ 28.5k | 5 555 | 66 |
| gin | 36 634 | ≈ 27.3k | 5 522 | 65 |
| echo | 34 658 | ≈ 28.9k | 5 498 | 66 |
| fasthttp | 29 745 | ≈ 33.6k | 3 264 | 46 |

### 多核（GOMAXPROCS=12，12 并发）

| 框架 | ns/op | 约合 req/s | B/op | allocs/op |
| --- | ---: | ---: | ---: | ---: |
| webx | 9 253 | ≈ 108k | 5 802 | 67 |
| gin | 9 744 | ≈ 103k | 5 784 | 66 |
| echo | 9 110 | ≈ 110k | 5 720 | 67 |
| fasthttp | 6 851 | ≈ 146k | 3 268 | 46 |

说明：单核下 webx 已略快于 gin/echo；多核下与两者持平，
fasthttp 因零拷贝架构领先约 35-45%。

### webx HTTP/2 参考（默认协议路径）

| 配置 | ns/op | 约合 req/s |
| --- | ---: | ---: |
| 单核 | 50 416 | ≈ 19.8k |
| 多核（12 并发） | 14 987 | ≈ 66.7k |

说明：HTTP/2 在小请求微基准下因帧与 HPACK 开销反而低于 HTTP/1.1；
其价值在真实场景的多路复用与低延迟，不在裸吞吐。基准统一走
HTTP/1.1 是为了跨框架公平对比。

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
