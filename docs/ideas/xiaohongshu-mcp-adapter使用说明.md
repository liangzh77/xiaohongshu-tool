# xiaohongshu-mcp Adapter 使用说明

## 目标

`xhs-mcp-adapter` 是一个只读采集适配器。它连接本地运行的 `xiaohongshu-mcp` HTTP API，把搜索结果或笔记详情转换成主采集框架需要的统一 JSON：

```json
{
  "items": []
}
```

主采集框架 `xhs-tool collect` 会读取这个 JSON，并写入 SQLite。

## 安全边界

当前适配器只实现：

- 关键词搜索：`GET /api/v1/feeds/search`
- 笔记详情：`POST /api/v1/feeds/detail`

当前适配器不实现：

- 发布图文/视频
- 评论/回复
- 点赞/取消点赞
- 收藏/取消收藏
- 删除 cookies
- 验证码处理
- 代理池、Cookie 池、设备指纹伪装

## 前置条件

先启动并登录 `xiaohongshu-mcp`，默认地址：

```bash
http://localhost:18060
```

可以用健康检查确认服务可用：

```bash
curl http://localhost:18060/health
```

可以用登录状态接口确认已登录：

```bash
curl http://localhost:18060/api/v1/login/status
```

后续如果选择源码级集成，可以去掉 HTTP sidecar，直接在本项目中调用上游 `xiaohongshu` 和 `browser` 包。

## 单独测试适配器

关键词搜索，默认会对搜索结果继续拉详情，以补全文案：

```bash
go run ./cmd/xhs-mcp-adapter --kind keyword --keyword "AI工具" --limit 5
```

只拿搜索卡片，不拉详情：

```bash
go run ./cmd/xhs-mcp-adapter --kind keyword --keyword "AI工具" --limit 5 --details=false
```

拉单篇笔记详情：

```bash
go run ./cmd/xhs-mcp-adapter --kind feed --feed-id "NOTE_ID" --xsec-token "XSEC_TOKEN"
```

## 接入主采集框架

初始化数据库：

```bash
go run ./cmd/xhs-tool db init --db data/xhs.db
```

添加一个关键词采集目标：

```bash
go run ./cmd/xhs-tool target add --db data/xhs.db --kind keyword --name "AI工具" --keyword "AI工具" --interval 5m
```

执行一次采集：

```bash
go run ./cmd/xhs-tool collect once --db data/xhs.db --command "go run ./cmd/xhs-mcp-adapter --base-url http://localhost:18060 --limit 5"
```

定时低频采集：

```bash
go run ./cmd/xhs-tool collect daemon --db data/xhs.db --command "go run ./cmd/xhs-mcp-adapter --base-url http://localhost:18060 --limit 5" --every 5m --limit 1
```

## 输出字段

每条 item 会尽量填充：

- `external_id`: 小红书笔记 ID
- `url`: 笔记 URL
- `author_name`: 作者昵称
- `title`: 标题
- `body`: 正文
- `tags`: 标签
- `like_count`: 点赞数
- `collect_count`: 收藏数
- `comment_count`: 评论数
- `published_at`: 发布时间
- `raw`: 原始响应片段

## 研发建议

先用 3-5 个关键词、每次 3-5 条、5-10 分钟一次的小样本跑 3 天，观察：

- 登录态是否稳定
- 是否频繁失败
- 是否出现验证码或异常提醒
- 采集字段是否足够支撑“爆文拆解”
- 拉详情的成本是否可以接受

如果详情接口不稳定，先降级为只保存搜索卡片，再做后续拆解实验。
