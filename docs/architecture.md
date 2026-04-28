# 架构设计

## 产品方向

本项目是一个内部使用的小红书内容运营副驾。第一阶段不做 Web 应用，也不做全自动运营系统，而是先把关键技术模块逐个验证清楚：

1. 数据采集
2. 笔记拆解
3. 选题评分
4. 内容生成
5. 复盘学习

每个模块都应该能独立运行、独立测试、独立评估效果，确认有效后再集成成完整工作流。

## 当前架构

当前实现是本地优先的 Go CLI 系统：

```text
cmd/
  xhs-tool/              主命令：采集目标、调度、SQLite 写入
  xhs-mcp-adapter/       连接已运行的 xiaohongshu-mcp HTTP 服务
  xhs-native-collector/  直接复用 xiaohongshu-mcp 的 Go 包进行采集

internal/
  collector/             执行外部采集命令，并保存结果
  analyzer/              笔记拆解：规则版和 OpenAI-compatible LLM 版
  scorer/                选题评分：规则版和 OpenAI-compatible LLM 版
  draftgen/              内容草稿生成：规则版和 OpenAI-compatible LLM 版
  reviewer/              复盘学习：规则版表现评分器
  storage/               SQLite + WAL 表结构和持久化
  xhsmcp/                xiaohongshu-mcp HTTP API 客户端
  xhsnative/             原生源码级采集器封装和数据映射
```

当前优先路线是 `xhs-native-collector`，因为它不需要额外运行 exe、HTTP 服务、MCP 服务或 Docker 容器。

## 数据流

```text
collector_targets
      |
      v
xhs-tool collect once/daemon
      |
      v
外部采集命令
      |
      v
JSON: { "items": [...] }
      |
      v
SQLite collected_items
      |
      v
note_analyses
      |
      v
topic_candidates
      |
      v
generated_drafts
      |
      v
publish_records -> performance_snapshots -> performance_reports
```

采集命令被设计成外部命令，是为了让调度和存储保持稳定，同时允许以后替换不同的数据来源：

- `xhs-native-collector`：当前优先的原生采集器
- `xhs-mcp-adapter`：HTTP API 备用方案
- 未来采集器：第三方数据服务、手动导入、浏览器插件等

## 存储设计

项目使用 SQLite 作为主数据库，并在 `internal/storage/sqlite.go` 中启用 WAL。

当前表：

- `collector_targets`：采集目标，例如关键词、笔记 ID
- `collection_runs`：采集运行记录、状态、时间、错误信息
- `collected_items`：标准化后的笔记/卡片数据，按 `target_id + external_id` 去重
- `note_analyses`：笔记拆解结果
- `topic_candidates`：候选选题和评分
- `generated_drafts`：生成的内容草稿
- `publish_records`：人工发布记录
- `performance_snapshots`：发布后的表现数据快照
- `performance_reports`：基于表现快照生成的复盘评分和调整建议

设计原则：每个阶段写自己的独立表，不把拆解、评分、生成、复盘结果塞回 `collected_items`。这样每一步都能单独测试和评估，也方便以后替换规则版实现为 LLM 或更复杂的学习算法。

## 采集策略

目前支持两条采集路径。

### 原生采集器

`xhs-native-collector` 直接引入 `github.com/xpzouying/xiaohongshu-mcp` 的底层包：

- `browser`
- `xiaohongshu`

它不会启动上游的 HTTP 服务或 MCP 服务。

这是 2GB Linux 服务器上的优先方案。

运行约束：

- 单 browser
- 单 page
- 同一时间只跑一个采集任务
- 默认不加载全部评论
- 每个关键词默认只取 3-5 条

### HTTP Adapter

`xhs-mcp-adapter` 连接一个已经运行的 `xiaohongshu-mcp` HTTP API。它仍然可以作为备用方案或对照方案，但不再是主路线。

## 部署假设

目标服务器：

- Linux
- 2GB 内存
- 不需要 GUI
- Go 1.24+
- 安装 Chrome 或 Chromium，用于 headless 采集

推荐运行环境变量：

```bash
export ROD_BROWSER_BIN=/usr/bin/chromium
export COOKIES_PATH=data/cookies.json
```

2GB 内存可以跑，但采集必须保持串行和低频。

## 关键设计决策

### 使用 Go + SQLite WAL

状态：已接受。

原因：

- 部署体积小
- 不需要单独数据库服务
- 足够支撑早期实验
- 适合本地 CLI 和单机 Linux 部署

代价：

- 目前不面向多人并发 Web 工作负载

### 模块必须单独测试

状态：已接受。

原因：

- 采集、拆解、评分、生成、复盘的失败模式不同
- 每个模块都需要单独验证质量
- 避免还没证明数据和模型有效，就先做复杂 Web 应用

### 优先使用原生采集器

状态：已接受。

原因：

- 少一个进程
- 少一个 HTTP 服务
- 更适合 2GB Linux 服务器
- 调度和落库逻辑仍由本项目掌控

代价：

- 上游依赖会把浏览器自动化依赖带入本项目
- 因为上游要求 Go 1.24，本项目也需要 Go 1.24

### 暂不做 Web UI

状态：已接受。

原因：

- 当前最大风险是输出质量，不是界面
- CLI 更适合快速测试和跑夜间任务
- Web UI 应该等模块契约稳定后再做

## 当前风险

- 登录和 cookies 流程已经有基础版本，但还需要在 Linux 服务器复测。
- 原生采集器依赖上游包的行为和页面解析逻辑。
- Headless Chrome 在 2GB 服务器上仍可能偏重。
- 笔记拆解已有规则版和 LLM 版入口，但输出质量还需要真实样本评估。
- 选题评分已有规则版入口，但权重还需要真实运营反馈校准。
- 内容生成和复盘学习已有第一版数据流骨架，但当前仍是规则/人工记录版，需要用真实样本验证质量。
- 当前还没有正式的 schema migration 系统，只有幂等建表。

## 下一步架构任务

当前已经增加第一版笔记拆解模块：

```text
collected_items -> note_analyses
```

该模块从 SQLite 读取采集到的笔记，生成结构化拆解结果，再写回 SQLite。

当前支持两种 analyzer：

- `rule`：规则版，默认启用，用于验证数据流
- `llm`：OpenAI-compatible 大模型版，用于真实内容拆解

当前也已经增加第一版选题评分模块：

```text
note_analyses -> topic_candidates
```

当前支持两种 scorer：

- `rule`：规则版，默认启用，用于验证排序链路
- `llm`：OpenAI-compatible 大模型版，用于真实选题评分和理由生成

当前已经增加第一版内容生成模块：

```text
topic_candidates -> generated_drafts
```

当前支持两种 draft generator：

- `rule`：规则版，默认启用，用于验证数据流
- `llm`：OpenAI-compatible 大模型版，用于真实草稿生成

当前已经增加第一版复盘学习数据骨架：

```text
generated_drafts -> publish_records -> performance_snapshots -> performance_reports
```

下一步需要用真实样本评估 LLM 拆解质量、规则评分质量和草稿可用性，并根据发布表现优化 prompt、字段和评分权重。
