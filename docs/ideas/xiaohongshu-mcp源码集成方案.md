# xiaohongshu-mcp 源码集成方案

## 技术判断

从技术角度，最适合本项目的集成方式是：**不启动 xiaohongshu-mcp HTTP 服务，不接 MCP 协议，而是直接复用它的底层 Go 包**。

也就是直接引入：

- `github.com/xpzouying/xiaohongshu-mcp/browser`
- `github.com/xpzouying/xiaohongshu-mcp/xiaohongshu`
- 必要时引入 `cookies` 和 `configs`

不要引入它的 `main.go`、Gin HTTP server、MCP server、发布/评论/点赞/收藏 handlers。

这样可以保持本项目的架构：

- 单一 Go CLI/daemon
- SQLite + WAL
- 无 Docker
- 无额外 HTTP 服务
- 不跑 MCP server
- 后续采集、拆解、评分、生成、复盘都在一个进程内编排

## 关键现实

内存最小化的瓶颈不在 Go，而在 Chrome。

`xiaohongshu-mcp` 的搜索、详情、用户主页能力本质上是 Rod 驱动 Chrome 页面，从 `window.__INITIAL_STATE__` 和页面 DOM 中取数据。即使不跑 Docker、不跑 HTTP server，只要要拿小红书网页数据，仍然需要一个 Chrome/Chromium 进程。

所以可优化的是：

- 不跑额外 HTTP server
- 不跑 MCP server
- 不每个请求都新建浏览器
- 复用一个 headless Chrome
- 同一时间只跑一个采集任务
- 每个任务只开一个 page，用完关闭 page
- 采集详情时限制条数，不加载全部评论

## 推荐架构

新增一个原生采集包：

```text
internal/xhsnative/
  browser_pool.go
  collector.go
  mapper.go
```

核心职责：

- `browser_pool.go`: 懒加载并复用一个 headless Chrome 实例。
- `collector.go`: 实现关键词搜索、笔记详情、用户主页三类只读采集。
- `mapper.go`: 把上游 `xiaohongshu.Feed` / `FeedDetailResponse` 映射成本项目 `storage.Item`。

再新增命令：

```text
cmd/xhs-native-collector/main.go
```

输出仍然保持：

```json
{
  "items": []
}
```

这样它可以直接替换现在的 `xhs-mcp-adapter`，继续被 `xhs-tool collect` 调用。

## 第一阶段实现范围

只做只读：

- 关键词搜索
- 可选拉详情补正文
- 单篇笔记详情

暂不做：

- 发布
- 评论
- 点赞
- 收藏
- 删除 cookies
- 代理池
- Cookie 池
- 验证码处理
- 多账号并发

## 依赖方式

在 `go.mod` 里加：

```go
require github.com/xpzouying/xiaohongshu-mcp <version>
```

开发期如果需要固定到本地 clone，可临时加：

```go
replace github.com/xpzouying/xiaohongshu-mcp => C:/Users/liang/AppData/Local/Temp/xiaohongshu-mcp-inspect
```

长期不要依赖临时目录。后续可以 fork 到自己的 GitHub，然后 replace/require 自己的 fork。

## 登录和 cookies

上游 `browser.NewBrowser()` 会从 `COOKIES_PATH` 读取 cookies。建议本项目统一设置：

```bash
COOKIES_PATH=data/cookies.json
```

登录流程有两种选择：

1. 第一阶段保留外部登录：先用上游工具生成 `cookies.json`，本项目只负责读取。
2. 第二阶段集成登录命令：在本项目里新增 `xhs-tool login qrcode`，调用 `xiaohongshu.NewLogin(page)` 获取二维码并保存 cookies。

推荐先做方案 1，采集跑通后再做方案 2。

## Linux 部署方式

Linux 上不需要 Docker，但机器必须安装 Chrome/Chromium。

示例：

```bash
export ROD_BROWSER_BIN=/usr/bin/google-chrome
export COOKIES_PATH=data/cookies.json
go run ./cmd/xhs-native-collector --kind keyword --keyword "AI工具" --limit 5
```

如果服务器没有桌面环境，使用 headless 模式即可。

## 最小内存策略

- 全局只保留 1 个 browser 实例。
- 设置采集并发为 1。
- 搜索结果默认只取 3-5 条。
- 默认 `load_all_comments=false`。
- 详情页拉取失败时降级保留搜索卡片。
- 每轮采集结束关闭 page，不关闭 browser。
- 长时间 daemon 可以每 N 次任务重启 browser，防止页面泄漏。

## 和当前 adapter 的关系

当前 `xhs-mcp-adapter` 是 HTTP sidecar 方案，适合快速验证。

下一步应新增 `xhs-native-collector`，跑通后：

- 保留 `xhs-mcp-adapter` 作为备选；
- 默认使用 `xhs-native-collector`；
- 后续稳定后可以删除 HTTP adapter。
