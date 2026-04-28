# 任务记录

## 当前状态

项目已经完成第一版数据采集原型：

- 已初始化 Go 项目
- 已加入 SQLite + WAL 存储
- 已加入采集调度 CLI
- 已加入 `xiaohongshu-mcp` HTTP adapter
- 已加入原生源码级采集器
- 已加入 storage 和 mapping 的基础测试
- 已补充产品方向、采集调研、原生采集器、adapter 使用说明等文档

## 已完成

- [x] 将产品方向收敛为“小红书内容增长副驾”
- [x] 调研内容采集路径和风险
- [x] 创建 Go + SQLite WAL 项目骨架
- [x] 实现 `xhs-tool` CLI，用于采集目标、运行记录和持久化
- [x] 定义外部采集命令协议：`{"items":[...]}`
- [x] 实现 `xhs-mcp-adapter`，作为 HTTP API 备用方案
- [x] 实现 `xhs-native-collector`，作为源码级原生采集方案
- [x] 增加 storage 和 native mapping 测试
- [x] 推送代码到 GitHub

## 进行中

- [ ] 在真实机器上使用 cookies + Chromium 跑通原生采集

## 下一批任务

### 1. 跑通原生采集器端到端

目标：证明不运行 `xiaohongshu-mcp.exe` 也能完成采集。

步骤：

- 在目标机器安装 Chrome/Chromium
- 设置 `ROD_BROWSER_BIN`
- 准备 `COOKIES_PATH=data/cookies.json`
- 先运行 `xhs-native-collector --details=false`
- 再运行 `--details=true --limit 3`
- 通过 `xhs-tool collect once` 确认数据能写入 SQLite

验收标准：

- 关键词搜索能返回 items
- `collected_items` 中能看到入库数据
- 不需要运行额外 HTTP/MCP 服务
- 2GB 服务器内存占用可接受

### 2. 接管登录和 cookies 流程

目标：不再依赖手动运行上游工具生成 cookies。

可选命令：

- `xhs-tool login status`
- `xhs-tool login qrcode`
- 将 cookies 保存到 `data/cookies.json`

验收标准：

- 本项目可以创建或更新 `data/cookies.json`
- 采集器可以复用该 cookie 文件
- 登录失效时能给出清晰错误

状态：已实现，并已在 Windows 本机完成扫码保存 cookies 验证。后续需要在 Linux 服务器复测。

### 3. 增加采集结果查看命令

目标：在做笔记拆解前，方便人工检查采集数据质量。

候选命令：

```bash
xhs-tool item list --db data/xhs.db --limit 20
xhs-tool item show --db data/xhs.db --id 123
xhs-tool run list --db data/xhs.db --limit 20
```

验收标准：

- 可以看到采集结果是否有标题、正文、作者、互动数据
- 可以查看失败运行和错误信息

状态：已实现。

### 4. 实现笔记拆解模块

目标：把原始笔记转成结构化内容洞察。

输出字段建议：

- 选题
- 目标用户痛点
- 标题钩子
- 开头钩子
- 情绪触发点
- 内容结构
- 转化意图
- 可复用表达模式
- 风险提示

验收标准：

- 从 `collected_items` 读取数据
- 写入新表 `note_analyses`
- 输出符合 JSON schema
- 支持单条运行和批量运行

### 5. 实现选题评分模块

目标：把拆解后的笔记沉淀成可执行选题，并排序。

初始评分维度：

- 账号匹配度
- 趋势信号
- 内容生产可行性
- 涨粉潜力
- 差异化空间
- 合规/风险

验收标准：

- 生成排序后的候选选题
- 每个分数都有解释
- 可以过滤低质量或高风险选题

## 暂不做

- Web UI
- 自动发布
- 自动点赞/评论/收藏
- 多账号矩阵操作
- 全量评论抓取
- 代理池或 Cookie 池
- 大规模爬取

## 常用命令

运行测试：

```bash
go test ./...
```

构建 CLI：

```bash
go build ./cmd/xhs-tool ./cmd/xhs-mcp-adapter ./cmd/xhs-native-collector
```

运行原生采集器：

```bash
go run ./cmd/xhs-native-collector --kind keyword --keyword "AI工具" --limit 3
```

通过 SQLite 调度执行采集：

```bash
go run ./cmd/xhs-tool collect once --db data/xhs.db --command "go run ./cmd/xhs-native-collector --limit 3"
```
