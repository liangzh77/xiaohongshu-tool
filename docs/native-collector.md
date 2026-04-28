# 原生采集器

## 目标

`xhs-native-collector` 直接复用 `xiaohongshu-mcp` 的底层 Go 包，不启动 `xiaohongshu-mcp` exe、不跑 HTTP 服务、不跑 MCP server。

它仍然输出主采集框架需要的统一 JSON：

```json
{
  "items": []
}
```

## 适合 2GB Linux 服务器的运行方式

2GB 内存可以跑，但必须保持低并发：

- 一个 Chrome/Chromium
- 一个 page
- 单并发
- 默认不加载评论
- 每次只取 3-5 条详情

Linux 服务器不需要 GUI，但需要安装 Chrome/Chromium。

Ubuntu/Debian 示例：

```bash
sudo apt update
sudo apt install -y chromium
```

如果包名不同，可以用：

```bash
which chromium
which chromium-browser
which google-chrome
```

运行前设置：

```bash
export ROD_BROWSER_BIN=/usr/bin/chromium
export COOKIES_PATH=data/cookies.json
```

## 单独测试

关键词搜索，默认会继续拉详情补全文案：

```bash
go run ./cmd/xhs-native-collector --kind keyword --keyword "AI工具" --limit 3
```

只拿搜索卡片，不拉详情，更省内存和时间：

```bash
go run ./cmd/xhs-native-collector --kind keyword --keyword "AI工具" --limit 5 --details=false
```

单篇详情：

```bash
go run ./cmd/xhs-native-collector --kind feed --feed-id "NOTE_ID" --xsec-token "XSEC_TOKEN"
```

## 接入采集框架

```bash
go run ./cmd/xhs-tool db init --db data/xhs.db
go run ./cmd/xhs-tool target add --db data/xhs.db --kind keyword --name "AI工具" --keyword "AI工具" --interval 5m
```

执行一次：

```bash
go run ./cmd/xhs-tool collect once --db data/xhs.db --command "go run ./cmd/xhs-native-collector --limit 3"
```

定时低频采集：

```bash
go run ./cmd/xhs-tool collect daemon --db data/xhs.db --command "go run ./cmd/xhs-native-collector --limit 3" --every 5m --limit 1
```

## 当前开发注意

`go.mod` 当前固定到上游 commit：

```go
github.com/xpzouying/xiaohongshu-mcp v1.2.3-0.20260427025311-edfcc6acd4c0
```

后续如果要长期维护，建议 fork 到自己的 GitHub，并在 fork 中只保留或重点维护只读采集相关能力。

考虑到上游 `go.mod` 是 Go 1.24，本项目目前也使用 Go 1.24。Linux 部署机需要安装 Go 1.24，或直接使用 Go 1.24 构建好的二进制。
