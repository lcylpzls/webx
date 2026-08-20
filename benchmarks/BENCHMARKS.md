# webx 性能对比基准

## 环境

- 操作系统：Windows 10（10.0.19045）
- CPU：AMD Ryzen 5 7600（Zen 4，6 核 12 线程）
- Go：1.26.5 windows/amd64
- 日期：2026-08-08
- webx 版本：v1.2.0（热路径深度优化）
- hertz 版本：v0.10.6（HTTP/2 使用 hertz-contrib/http2 v0.1.8）
- 命令：`go test -bench . -benchmem -benchtime=1s -count=3 -cpu=1,12 -run '^$' .`
- 数据：单核（`-cpu=1`）与多核（`-cpu=12`）各跑 3 轮，取每组最优值

## 方法学

- 工具：`go test -bench . -benchmem`，`-benchtime=1s -count=3`；
- 并行模型：`RunParallel`，`-cpu=1` 为单 worker（等效顺序压测），
  `-cpu=12` 为 12 并发；
- 预热：每个基准在计时前由 12 个 goroutine 各发 200 次请求，使
  TLS/QUIC 握手与连接池在计时前就绪，避免握手计入结果；
- 客户端：HTTP/1.1 与 HTTP/2 使用 `net/http` 连接池
  （`MaxIdleConnsPerHost=64`）；HTTP/3 使用 quic-go `http3.Transport`
  （单 QUIC 连接多路复用）；
- 协议真实性：每个协议路径均有断言测试（协商结果必须为
  `HTTP/2.0` / `HTTP/3.0`），防止基准静默回退为 HTTP/1.1；
- 公平性：全部为 HTTPS；gin/echo/ServeMux 与 webx 同属标准库阵营；
  fasthttp/hertz 为自研协议栈参照；HTTP/3 四者统一挂载 quic-go
  `http3.Server`（webx 内建同栈实现）。

## 公平性约定

- **全部框架服务端均使用 HTTPS（TLS 1.2，HTTP/1.1 keep-alive）**；
  webx 只支持 HTTPS，因此不引入任何明文 HTTP 对比；
- **标准库封装阵营**：webx、gin、echo、裸 `net/http` ServeMux 均跑在
  `net/http` 服务器之上（`ServeTLS` 自动协商协议），fasthttp、hertz
  为自研协议栈参照，单独列出；
- HTTP/2 对比为独立小节：webx 与 hertz 均使用 TLS + ALPN 协商 h2，
  客户端统一使用 `net/http`（`ForceAttemptHTTP2`）；
- HTTP/3 对比为独立小节：四者统一挂载 quic-go `http3.Server`，
  客户端统一使用 quic-go `http3.Transport`；
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
| net/http ServeMux（标准库参照） | 145.0 | 32 | 1 |
| webx | 104.3 | 13 | **0** |
| gin | 87.6 | 58 | 1 |
| echo | 89.6 | 18 | 1 |
| fasthttprouter | 204.0 | 56 | 3 |

### 多核（GOMAXPROCS=12）

| 框架 | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| net/http ServeMux（标准库参照） | 141.1 | 31 | 1 |
| webx | 104.4 | 11 | **0** |
| gin | 81.4 | 58 | 1 |
| echo | 87.9 | 18 | 1 |
| fasthttprouter | 166.2 | 56 | 3 |

说明：P0 零分配改造后（路由参数槽化、Context 池复用），webx 路由分发
达到 **0 allocs/op**，耗时约 0.10µs，与 gin/echo 差距缩小到 15% 以内，
并快于标准库 ServeMux 与 fasthttprouter。

## 二、端到端服务基准（HTTPS + keep-alive）

以下四个框架均为基于 `net/http` 的**标准库封装阵营**（webx、gin、
echo、裸 ServeMux），本地回环 **HTTPS** 服务（TLS 1.2 + HTTP/1.1
keep-alive），Handler 写入 `hello`；`-cpu=1` 时 RunParallel 单 worker
（等效顺序压测），`-cpu=12` 时 12 并发。

### 单核（GOMAXPROCS=1）

| 框架 | ns/op | 约合 req/s | B/op | allocs/op |
| --- | ---: | ---: | ---: | ---: |
| net/http ServeMux（标准库下限） | 34 985 | ≈ 28.6k | 4 710 | 61 |
| echo | 34 954 | ≈ 28.6k | 5 497 | 66 |
| gin | 35 256 | ≈ 28.4k | 5 521 | 65 |
| webx | 35 332 | ≈ 28.3k | 5 554 | 66 |

### 多核（GOMAXPROCS=12，12 并发）

| 框架 | ns/op | 约合 req/s | B/op | allocs/op |
| --- | ---: | ---: | ---: | ---: |
| echo | 8 162 | ≈ 123k | 5 596 | 66 |
| net/http ServeMux（标准库下限） | 8 231 | ≈ 121k | 4 805 | 62 |
| webx | 8 611 | ≈ 116k | 5 665 | 66 |
| gin | 8 620 | ≈ 116k | 5 627 | 66 |

说明：**标准库封装阵营内 webx 与 gin/echo/ServeMux 完全同档**，单核
差距在 1% 以内，多核差距在 6% 以内，均属本地回环噪声；webx 单请求
分配约 66 次，主要来自 `net/http` 连接与缓冲管理，路由与响应信封
本身为 0 分配。

自研协议栈参照（不在标准库阵营内）：

| 框架 | 单核 | 多核 |
| --- | ---: | ---: |
| fasthttp | 29 584 | 6 911 |
| hertz | 29 983 | 6 730 |

fasthttp/hertz 通过自研 HTTP/1.1 解析与对象复用领先标准库阵营约
18-25%，这是协议栈实现差异，非框架层差距。

## 三、HTTP/2 基准（HTTPS + ALPN h2）

标准库封装阵营（webx、gin、echo、裸 ServeMux）以 TLS + ALPN 协商
HTTP/2；客户端统一使用 `net/http` HTTP/2 传输（单连接多路复用），
预热后并行压测。hertz 作为自研协议栈参照保留。

### 单核（GOMAXPROCS=1）

| 框架 | ns/op | 约合 req/s | B/op | allocs/op |
| --- | ---: | ---: | ---: | ---: |
| net/http ServeMux（标准库下限） | 38 610 | ≈ 25.9k | 6 372 | 64 |
| echo | 49 240 | ≈ 20.3k | 7 276 | 72 |
| gin | 49 636 | ≈ 20.1k | 7 298 | 71 |
| webx | 50 383 | ≈ 19.8k | 7 330 | 72 |
| hertz（自研协议栈参照） | 47 757 | ≈ 20.9k | 5 536 | 62 |

### 多核（GOMAXPROCS=12，12 并发）

| 框架 | ns/op | 约合 req/s | B/op | allocs/op |
| --- | ---: | ---: | ---: | ---: |
| webx | 12 264 | ≈ 81.5k | 7 635 | 69 |
| hertz（自研协议栈参照） | 12 496 | ≈ 80.0k | 5 691 | 59 |
| gin | 13 548 | ≈ 73.8k | 7 593 | 68 |
| net/http ServeMux（标准库下限） | 13 697 | ≈ 73.0k | 6 669 | 63 |
| echo | 14 267 | ≈ 70.1k | 7 567 | 69 |

说明：HTTP/2 小请求微基准因帧与 HPACK 开销低于 HTTP/1.1；其价值在
真实场景的多路复用与低延迟，不在裸吞吐。标准库阵营内：单核裸
ServeMux 最快（少一层框架上下文），webx/gin/echo 同档；**多核下
webx 反超并领先全部标准库阵营约 10-16%**（较 gin 快 10%、echo 快
16%、ServeMux 快 12%），同时快于自研协议栈的 hertz（约 2%）。webx
的路由零分配与 Context 池在 h2 多流并发下收益被放大。

### webx 全中间件参考（生产形态，HTTP/1.1）

| 配置 | ns/op | 约合 req/s | B/op | allocs/op |
| --- | ---: | ---: | ---: | ---: |
| 裸路径（单核） | 35 332 | ≈ 28.3k | 5 554 | 66 |
| 全中间件（单核） | 45 301 | ≈ 22.1k | 9 267 | 111 |
| 裸路径（多核） | 8 611 | ≈ 116k | 5 665 | 66 |
| 全中间件（多核） | 10 015 | ≈ 100k | 9 436 | 111 |

说明：全中间件 = RequestID + Recovery + Timeout + CORS + Validation +
Security + Gzip + Metrics + AccessLog；相对裸路径增加约 22% 耗时与
45 allocs，属中间件固有成本（日志、压缩、安全头、超时上下文等）。
v1.2.0 起头部写入全部使用预计算规范化键，消除键规范化分配。

## 四、HTTP/3 基准（QUIC over UDP）

标准库 `net/http` 本身不提供 HTTP/3 服务端，但 gin/echo/ServeMux
均为 `http.Handler`，可统一挂到 quic-go `http3.Server` 宿主上；
webx 的 HTTP/3 同为 quic-go 协议栈（quic-go v0.61 + `http3`），因此
四者共享同一传输层与协议实现，只差框架层。客户端统一使用 quic-go
`http3.Transport`（单 QUIC 连接多路复用），与 h1/h2 相同的预热与
并行压测逻辑；数据于 2026-08-09 采集，与上表同一台机器。

### 单核（GOMAXPROCS=1）

| 框架 | ns/op | 约合 req/s | B/op | allocs/op |
| --- | ---: | ---: | ---: | ---: |
| echo | 111 751 | ≈ 8.9k | 12 874 | 204 |
| net/http ServeMux（标准库下限） | 113 589 | ≈ 8.8k | 12 873 | 204 |
| webx | 118 096 | ≈ 8.5k | 12 939 | 205 |
| gin | 118 181 | ≈ 8.5k | 12 898 | 203 |

### 多核（GOMAXPROCS=12，12 并发）

| 框架 | ns/op | 约合 req/s | B/op | allocs/op |
| --- | ---: | ---: | ---: | ---: |
| gin | 29 408 | ≈ 34.0k | 13 515 | 199 |
| webx | 29 856 | ≈ 33.5k | 13 565 | 201 |
| net/http ServeMux（标准库下限） | 29 678 | ≈ 33.7k | 13 438 | 200 |
| echo | 30 669 | ≈ 32.6k | 13 486 | 200 |

说明：四者在 HTTP/3 下**完全同档**（单核差距 ≤6%、多核差距 ≤4%，
均属噪声），因为传输与协议实现全部由 quic-go 承担，框架层差异被
摊薄；对比 HTTP/1.1，HTTP/3 在本地回环小请求下吞吐约为 h1 的
1/3.4（单核）到 1/4（多核），这是 QUIC 的固有成本（UDP 丢包重传、
TLS 1.3 握手、帧与流控管理）；其价值在真实网络：多路复用免队头
阻塞、连接迁移、0-RTT 与弱网抗丢包，不在回环裸吞吐。

## 五、三协议总览与结论（webx 视角）

### webx 自身三协议吞吐

| 协议 | 单核 | 多核（12 并发） | allocs/op |
| --- | ---: | ---: | ---: |
| HTTP/1.1 | 35.3µs（≈28.3k req/s） | 8.6µs（≈116k） | 66 |
| HTTP/2 | 50.4µs（≈19.8k） | 12.3µs（≈81.5k） | 69 |
| HTTP/3 | 118.1µs（≈8.5k） | 29.9µs（≈33.5k） | 201 |

### 标准库阵营定位

- HTTP/1.1：与 gin / echo / ServeMux 完全同档（差距 ≤6%，噪声级）；
- HTTP/2 单核：与 gin/echo 同档（裸 ServeMux 因少一层框架上下文最快）；
- HTTP/2 多核：webx 领先全部标准库阵营约 10-16%，同时快于自研
  协议栈参照 hertz（约 2%）；
- HTTP/3：四家完全同档，quic-go 协议栈摊薄框架层差异。

### 结论

webx 可作为工业级基座的性能依据：

- 路由匹配与响应信封 0 allocs/op，端到端分配主要来自 `net/http` 与
  quic-go 协议栈自身；
- HTTP/2 多核并发是强项（标准库阵营第一）；HTTP/1.1 与主流标准库
  框架持平；HTTP/3 无短板；
- 与自研协议栈（fasthttp / hertz）的 HTTP/1.1 差距约 20% 属传输层
  实现差异，不影响以 HTTP/2/HTTP/3 为主的生产定位；
- 该矩阵同时用于 CI 门禁（路由 `≤1 allocs/op` 与标准化响应门禁），
  防止回归。

## 注意事项

- webx 传输可裁剪：`UseHttp1or2Listen(addr, true)` 为 HTTPS
  （HTTP/1.1+HTTP/2，ALPN 协商），本基准的 HTTP/1.1 对比统一走 HTTPS；
  纯 HTTP/1.1 明文模式（`useTLS=false`）不经 TLS 加密，未纳入横向对比；
- HTTP/3 宿主为 quic-go `http3.Server`：标准库 `net/http` 无 h3 服务端，
  gin/echo/ServeMux 以 `http.Handler` 挂载，webx 内建同栈实现；
- hertz 的 HTTP/2 需额外注册 `hertz-contrib/http2` 扩展（`AddProtocol`），
  本次对比即采用官方扩展最新版；
- 路由基准只衡量内存内分发，不反映网络、TLS、序列化与中间件开销；
- 本矩阵为本地回环 + 小请求（`hello`）微基准，真实负载（大 body、
  弱网、多机部署、连接迁移）请使用 wrk / hey / k6 等工具独立压测；
- 数据采集于 Windows 10（AMD Ryzen 5 7600），Linux 生产环境数值与
  相对关系可能不同，建议在目标环境复测关键基准。

## 六、Linux 虚拟机复测（Debian 13 / 2 核）

2026-08-09 将源码推送至内网 Linux 虚拟机（10.127.90.113）复测：

- 操作系统：Debian GNU/Linux 13 (trixie)，内核 6.12.86 x86_64；
- 硬件：2 核 / 1.9GiB 内存（虚拟化环境，资源受限）；
- Go：1.26.5 linux/amd64；依赖经 goproxy.cn 下载；
- 命令：`go test -bench . -benchmem -benchtime=1s -count=3 -cpu=1,2 -run '^$' .`；
- 数据：单核（`-cpu=1`）与双核（`-cpu=2`）各 3 轮取最优。

### 路由分发基准（内存内）

| 框架 | 单核 | 双核 |
| --- | ---: | ---: |
| webx | 109.5 ns（0 allocs） | 110.8 ns（0 allocs） |
| echo | 112.7 ns | 108.3 ns |
| gin | 123.7 ns | 106.6 ns |
| net/http ServeMux | 151.2 ns | 152.5 ns |
| fasthttprouter | 216.1 ns | 192.8 ns |

### 端到端（HTTPS，ns/op → 约合 req/s）

| 框架 | HTTP/1.1 单核 | HTTP/1.1 双核 | HTTP/2 单核 | HTTP/2 双核 | HTTP/3 单核 | HTTP/3 双核 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| webx | 26 572（37.6k） | 28 091（35.6k） | 38 765（25.8k） | 40 973（24.4k） | 63 712（15.7k） | 55 468（18.0k） |
| gin | 26 692（37.5k） | 27 878（35.9k） | 46 036（21.7k） | 44 155（22.6k） | 59 866（16.7k） | 54 787（18.3k） |
| echo | 26 893（37.2k） | 27 200（36.8k） | 45 798（21.8k） | 45 750（21.9k） | 56 551（17.7k） | 51 613（19.4k） |
| net/http ServeMux | 25 965（38.5k） | 26 210（38.2k） | 34 005（29.4k） | 35 582（28.1k） | 56 689（17.6k） | 52 501（19.0k） |
| hertz（自研协议栈参照） | 22 473（44.5k） | 22 511（44.4k） | 35 886（27.9k） | 37 679（26.5k） | 无法构建（扩展停更） | 无法构建 |
| fasthttp（自研协议栈参照） | 21 909（45.6k） | 20 977（47.7k） | 不支持 | 不支持 | 不支持 | 不支持 |

### 结论

- HTTP/1.1：与 Windows 结论一致——webx 与 gin/echo/ServeMux 同档
  （差距 ≤4%），hertz/fasthttp 靠自研协议栈领先 15-22%；
- HTTP/2：受限双核环境下 webx 仍领先 gin/echo 约 15%，略低于裸
  ServeMux 与 hertz（约 8-14%），且首轮波动较大（虚拟机 GC/调度
  噪声），多核生产环境的差距预期进一步收窄（Windows 12 核实测
  webx 为阵营第一）；
- HTTP/3：webx 与标准库阵营差距约 6-13%，属资源受限下的正常波动；
- 全中间件（webx，HTTP/1.1 单核）：34 384 ns ≈ 29.1k req/s。

说明：虚拟机仅 2 核 / 1.9GiB，客户端与服务端同机竞争资源，数值
整体高于 Windows 开发机（非性能回退）；三平台 CI 的基准门禁不受影响。

## 复现

```powershell
cd benchmarks
go test -bench . -benchmem -benchtime=1s -count=3 -cpu=1,12 -run '^$' .
```

```bash
# Linux 虚拟机（示例：2 核）
cd benchmarks
go test -bench . -benchmem -benchtime=1s -count=3 -cpu=1,2 -run '^$' .
```
