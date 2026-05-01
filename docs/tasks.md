# 任务记录

## 当前状态

项目已经完成第一版端到端 CLI 骨架：

- 已初始化 Go 项目
- 已加入 SQLite + WAL 存储
- 已加入采集调度 CLI
- 已加入 `xiaohongshu-mcp` HTTP adapter
- 已加入原生源码级采集器
- 已加入登录、采集结果查看、笔记拆解、选题评分、草稿生成、复盘记录命令
- 已加入 storage、native mapping、analyzer、scorer、draftgen 的基础测试
- 已补充产品方向、采集调研、原生采集器、adapter 使用说明和任务记录文档

## 已完成

- [x] 将产品方向收敛为“小红书内容增长副驾”
- [x] 调研内容采集路径和风险
- [x] 创建 Go + SQLite WAL 项目骨架
- [x] 实现 `xhs-tool` CLI，用于采集目标、运行记录和持久化
- [x] 定义外部采集命令协议：`{"items":[...]}`
- [x] 实现 `xhs-mcp-adapter`，作为 HTTP API 备用方案
- [x] 实现 `xhs-native-collector`，作为源码级原生采集方案
- [x] 增加 storage 和 native mapping 测试
- [x] 实现登录二维码和 cookies 保存流程
- [x] 实现采集结果查看命令
- [x] 实现笔记拆解模块，支持规则版和 OpenAI-compatible LLM 版
- [x] 实现选题评分模块，支持规则版和 OpenAI-compatible LLM 版评分排序
- [x] 实现内容草稿生成模块，支持规则版草稿
- [x] 实现复盘学习数据骨架，支持发布记录、表现快照和规则版复盘评分
- [x] 实现轻量 Web 工作台，后端和前端都由 Go 单二进制提供
- [x] 将 Web 工作台改为按工作流拆分的多工作区交互
- [x] 采集目标支持编辑和软删除
- [x] 增加内容页，支持左侧列表和右侧完整详情
- [x] 采集页支持多选目标、设置每个目标采集条数，并查看单次运行采集到的标题
- [x] 增加详情状态、缺失字段和失败原因，主服务使用本机非 headless 浏览器从搜索页逐条采集详情
- [x] 接入密钥分发服务配置，页面填写服务地址、用户名、密码，并从可用密钥下拉菜单选择 LLM Key
- [x] 推送代码到 GitHub

## 进行中

- [x] 在真实机器上使用 cookies + Chromium 跑通原生采集

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

### 6. Windows 本机原生采集验证

目标：验证不启动 `xiaohongshu-mcp.exe`，直接使用本项目原生采集器完成采集。

结果：

- `login status` 返回 `logged_in: true`
- `xhs-native-collector --details=false` 可以返回搜索卡片
- `xhs-tool collect once` 可以写入 SQLite
- `xhs-tool item list` 可以查看入库结果
- `xhs-tool run list` 可以查看运行记录

状态：已完成。

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

状态：已完成第一版骨架，并已接入 OpenAI-compatible LLM analyzer。当前默认仍使用规则版 analyzer；通过 `--engine llm` 可启用大模型拆解。后续需要用真实模型和更多样本评估输出质量。

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

状态：已完成第一版规则评分骨架，并已接入 OpenAI-compatible LLM scorer。当前支持 `score batch --engine rule|llm` 和 `score list`，可生成 `topic_candidates` 并按总分排序。后续需要用真实运营反馈校准权重和提示词。

### 7. 实现内容草稿生成模块

目标：把候选选题变成可供运营人员快速审核和改写的草稿。

当前输出：

- 标题备选
- 开头
- 正文结构
- 封面文案
- 图片脚本
- 标签建议
- 风险提示

状态：已完成第一版规则生成骨架，并已接入 OpenAI-compatible LLM 生成器。当前支持 `draft batch --engine rule|llm` 和 `draft list`，写入 `generated_drafts`。后续需要加入人工评分字段，并用真实样本评估草稿可用率。

### 8. 实现复盘学习数据骨架

目标：把人工发布和发布后表现记录下来，形成后续校准评分权重和 prompt 的数据基础。

当前命令：

```bash
go run ./cmd/xhs-tool publish add --db data/xhs.db --draft-id 1 --url "https://www.xiaohongshu.com/explore/..." --operator "editor"
go run ./cmd/xhs-tool publish list --db data/xhs.db --limit 20
go run ./cmd/xhs-tool review add --db data/xhs.db --publish-id 1 --views 1000 --likes 80 --collects 20 --comments 5 --follows 3
go run ./cmd/xhs-tool review list --db data/xhs.db --limit 20
go run ./cmd/xhs-tool review score --db data/xhs.db --limit 20
go run ./cmd/xhs-tool review report --db data/xhs.db --limit 20
```

状态：已完成第一版数据骨架和规则版复盘评分。当前只记录人工发布后的数据，不自动发布、不自动抓取账号后台数据。后续需要做选题反推、权重校准和 LLM 复盘总结。

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
go build ./cmd/xhs-tool ./cmd/xhs-mcp-adapter ./cmd/xhs-native-collector ./cmd/xhs-web
```

启动 Web 工作台：

```bash
go run ./cmd/xhs-web --addr :8080 --db data/xhs.db
```

运行原生采集器：

```bash
go run ./cmd/xhs-native-collector --kind keyword --keyword "AI工具" --limit 3
```

通过 SQLite 调度执行采集：

```bash
go run ./cmd/xhs-tool collect once --db data/xhs.db --command "go run ./cmd/xhs-native-collector --limit 3"
```

从采集到复盘的本地流程：

```bash
go run ./cmd/xhs-tool analyze batch --db data/xhs.db --limit 20 --engine rule
go run ./cmd/xhs-tool score batch --db data/xhs.db --limit 20 --engine rule
go run ./cmd/xhs-tool draft batch --db data/xhs.db --limit 20
go run ./cmd/xhs-tool publish add --db data/xhs.db --draft-id 1 --url "https://www.xiaohongshu.com/explore/..."
go run ./cmd/xhs-tool review add --db data/xhs.db --publish-id 1 --views 1000 --likes 80 --collects 20 --comments 5 --follows 3
go run ./cmd/xhs-tool review score --db data/xhs.db --limit 20
```
