# xpzouying/xiaohongshu-mcp 评估

## 结论

可以作为调研和原型验证对象，但不建议第一版直接深度依赖或复制代码。

更稳的用法是：把它放在独立进程或容器里，只调用只读能力，把返回结果标准化写入我们自己的 SQLite 数据库。我们的系统仍然保留采集目标、频率控制、任务记录、结果去重、错误记录和后续拆解流程。

## 可用价值

README 显示该项目已经支持：

- 登录和检查登录状态
- 搜索内容
- 获取推荐列表
- 获取帖子详情，包括互动数据和评论
- 获取用户主页
- 发布图文/视频
- 评论、回复、点赞、收藏
- MCP HTTP 接入

对我们的采集模块来说，真正需要的只有：

- 搜索内容
- 推荐列表
- 帖子详情
- 用户主页公开笔记列表

其余写操作能力应默认禁用。

## 主要风险

- 仓库根目录没有看到明确的 `LICENSE` 文件，不能默认认为可以直接复制代码进商业或内部长期项目。
- 功能包含发布、评论、点赞、收藏，这些能力如果交给自动任务，会把采集工具变成账号操作工具，风险边界扩大。
- README 明确依赖登录态，并提到 cookies 保存、网页登录互踢等行为，说明账号状态是关键风险点。
- 项目不是官方小红书 API。即使作者声称稳定运行，也不能等价为平台允许或不会触发风控。
- MCP 面向 AI 工具调用，如果不做工具白名单，模型可能误调用写操作。

## 推荐接入方式

不要把它作为库 import 到我们的 Go 项目里。建议先用外部适配器方式：

1. 独立运行 `xiaohongshu-mcp`。
2. 我们的 `xhs-tool collect daemon` 按频率选择 due target。
3. 外部 adapter 调用 MCP 的只读工具。
4. adapter 把结果转换成统一 JSON 输出到 stdout。
5. 我们的采集框架读取 stdout，写入 SQLite。

统一 JSON 格式：

```json
{
  "items": [
    {
      "external_id": "note_id",
      "url": "https://www.xiaohongshu.com/...",
      "author_name": "作者",
      "title": "标题",
      "body": "正文",
      "tags": ["标签"],
      "like_count": 10,
      "collect_count": 3,
      "comment_count": 2,
      "published_at": "2026-04-27T10:00:00+08:00",
      "raw": {}
    }
  ]
}
```

## 安全边界

第一阶段只允许调用：

- `check_login_status`
- 搜索内容
- 获取推荐列表
- 获取帖子详情
- 获取用户主页

第一阶段禁止调用：

- 发布图文/视频
- 评论
- 回复评论
- 点赞/取消点赞
- 收藏/取消收藏
- 删除 cookies

如果要接入 MCP，必须做工具白名单，不能把完整 MCP 工具集直接暴露给自动 agent。

## 是否替代 RPA

它可以替代一部分 RPA，但不能消除风控风险。它仍然需要登录态，仍然可能受到平台限制。它的价值在于让我们更快验证“低频自动采集是否能拿到足够数据”，不是证明这条路长期稳定。

如果试用，建议小样本开始：

- 1 个测试账号
- 5 个关键词
- 每个关键词每 5-10 分钟采集一次
- 每次只保存前几条结果
- 只读，不互动，不发布
- 连续观察 3-7 天登录状态、验证码、异常提醒和数据质量

## Sources

- GitHub: https://github.com/xpzouying/xiaohongshu-mcp
- README raw: https://raw.githubusercontent.com/xpzouying/xiaohongshu-mcp/main/README.md
