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

## 登录和 cookies

检查当前 cookies 是否可用：

```bash
go run ./cmd/xhs-tool login status --cookies data/cookies.json
```

生成登录二维码并等待扫码：

```bash
go run ./cmd/xhs-tool login qrcode --cookies data/cookies.json --out data/login-qrcode.html --wait 4m
```

命令会生成 `data/login-qrcode.html`。打开该文件，用小红书 App 扫码。扫码成功后，cookies 会保存到 `data/cookies.json`。

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

## 笔记拆解

规则版拆解，适合验证数据流：

```bash
go run ./cmd/xhs-tool analyze batch --db data/xhs.db --limit 3 --engine rule
```

大模型拆解，使用 OpenAI-compatible API：

```bash
export XHS_LLM_BASE_URL=https://api.openai.com/v1
export XHS_LLM_API_KEY=your_api_key
export XHS_LLM_MODEL=your_model

go run ./cmd/xhs-tool analyze batch --db data/xhs.db --limit 3 --engine llm
```

也可以直接传参数：

```bash
go run ./cmd/xhs-tool analyze item --db data/xhs.db --id 1 --engine llm --llm-model your_model --llm-api-key your_api_key
```

## 选题评分

基于拆解结果生成候选选题和评分。规则版适合验证链路：

```bash
go run ./cmd/xhs-tool score batch --db data/xhs.db --limit 20 --engine rule
```

大模型版使用 OpenAI-compatible API：

```bash
export XHS_LLM_BASE_URL=https://api.openai.com/v1
export XHS_LLM_API_KEY=your_api_key
export XHS_LLM_MODEL=your_model

go run ./cmd/xhs-tool score batch --db data/xhs.db --limit 20 --engine llm
```

查看评分结果：

```bash
go run ./cmd/xhs-tool score list --db data/xhs.db --limit 20
```

真实运营阶段需要根据人工反馈和发布表现校准评分提示词、规则权重和阈值。

## 内容草稿生成

基于候选选题生成可供运营人员审核的草稿。规则版适合验证链路：

```bash
go run ./cmd/xhs-tool draft batch --db data/xhs.db --limit 20 --engine rule
```

大模型版使用 OpenAI-compatible API：

```bash
export XHS_LLM_BASE_URL=https://api.openai.com/v1
export XHS_LLM_API_KEY=your_api_key
export XHS_LLM_MODEL=your_model

go run ./cmd/xhs-tool draft batch --db data/xhs.db --limit 20 --engine llm
```

查看草稿：

```bash
go run ./cmd/xhs-tool draft list --db data/xhs.db --limit 20
```

当前输出标题备选、开头、正文、封面文案、图片脚本、标签和风险提示。真实使用阶段需要由运营人员做事实核查和表达调整。

## 复盘记录

本项目不自动发布。运营人员人工发布后，可以记录发布链接：

```bash
go run ./cmd/xhs-tool publish add --db data/xhs.db --draft-id 1 --url "https://www.xiaohongshu.com/explore/..." --operator "editor"
```

查看发布记录：

```bash
go run ./cmd/xhs-tool publish list --db data/xhs.db --limit 20
```

记录发布后的表现快照：

```bash
go run ./cmd/xhs-tool review add --db data/xhs.db --publish-id 1 --views 1000 --likes 80 --collects 20 --comments 5 --follows 3
```

查看表现快照：

```bash
go run ./cmd/xhs-tool review list --db data/xhs.db --limit 20
```

生成规则版复盘评分：

```bash
go run ./cmd/xhs-tool review score --db data/xhs.db --limit 20
```

查看复盘报告：

```bash
go run ./cmd/xhs-tool review report --db data/xhs.db --limit 20
```

这些数据后续用于校准选题评分、总结可复用模式，并筛掉实际表现差的选题类型。当前评分是规则版，重点是先打通数据闭环，不代表最终运营判断。

## 当前开发注意

`go.mod` 当前固定到上游 commit：

```go
github.com/xpzouying/xiaohongshu-mcp v1.2.3-0.20260427025311-edfcc6acd4c0
```

后续如果要长期维护，建议 fork 到自己的 GitHub，并在 fork 中只保留或重点维护只读采集相关能力。

考虑到上游 `go.mod` 是 Go 1.24，本项目目前也使用 Go 1.24。Linux 部署机需要安装 Go 1.24，或直接使用 Go 1.24 构建好的二进制。
