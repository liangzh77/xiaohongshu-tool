# Web 工作台

## 启动

```bash
go run ./cmd/xhs-web --addr :8080 --db data/xhs.db
```

打开：

```text
http://localhost:8080
```

服务仍然使用 Go + SQLite + WAL，不需要 Node、Docker 或独立前端构建流程。前端静态资源直接内嵌在 `xhs-web` 二进制中。

## 大模型密钥

页面不会要求填写大模型 API Key。大模型 Key 通过密钥分发服务获取。

在页面顶部填写：

- 密钥分发服务
- 用户名
- 密码
- 可用密钥

点击“测试并保存”后，后端会：

1. 调用 `POST /api/auth/login` 获取 JWT token
2. 调用 `GET /api/keys` 拉取当前用户可访问的密钥列表
3. 在页面“可用密钥”下拉菜单中展示密钥名称
4. 按用户选择调用 `GET /api/keys/{keyName}` 获取真实 API Key
5. 只在服务端内存中使用该 Key 调用 LLM

默认 keyName 是 `OPENAI_API_KEY`，可以启动时修改：

```bash
go run ./cmd/xhs-web --key-name GEMINI_API_KEY
```

如果下拉菜单选择了其他密钥，后续拆解、评分、生成草稿的大模型调用会使用当前选择的密钥。真实 Key 不会返回给浏览器。

模型和 OpenAI-compatible 地址仍通过启动参数或环境变量设置：

```bash
set XHS_LLM_MODEL=your_model
set XHS_LLM_BASE_URL=https://api.openai.com/v1
go run ./cmd/xhs-web --addr :8080 --db data/xhs.db
```

## 当前页面能力

- 新增关键词采集目标
- 触发一次采集
- 批量笔记拆解
- 批量选题评分
- 批量生成草稿
- 查看采集内容、候选选题、草稿、运行记录和复盘报告
- 触发规则版复盘评分

## 当前限制

- 密钥分发服务配置只保存在当前服务进程内存中，重启后需要重新填写。
- Web 工作台不做自动发布。
- 复盘指标仍需要人工录入或后续接入只读数据来源。
- LLM 功能需要先设置模型名，否则页面会提示配置错误。
