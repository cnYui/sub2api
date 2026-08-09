# 旧 Claude 与 Grok 渠道平台类型修正

## 变更

将截图中的三个旧渠道从 OpenAI 平台修正为对应平台，并同步修正其唯一承接账号与模型目录监控：

| 渠道 | 分组 ID | 账号 ID | 监控 ID | 平台 |
| --- | ---: | ---: | ---: | --- |
| Grok0.9倍率(优质) | 3 | 1 | 1 | `grok` |
| Claude1.5倍率(优质) | 4 | 2 | 2 | `anthropic` |
| Claude0.45倍率 | 5 | 3 | 3 | `anthropic` |

- 分组、账号和监控 provider 在同一事务口径下保持一致，避免平台过滤后账号不可调度。
- 监控名称同步为当前渠道名称；Kiro 监控移除上游模型目录中不存在的 `claude-opus-4-5-20251124`。历史探测记录保留审计，不删除。
- 写入 API Key 认证缓存失效事件和调度器全量重建事件；重启应用使内存渠道缓存与调度快照重新加载。

## 验证

- 三把对应用户 API Key 的无计费 `GET /v1/models` 均返回 HTTP 200。
- 新平台下监控首轮模型目录状态：Grok 8/8、Claude1.5 9/9、Claude0.45 10/10 均为 `operational`。
- Redis `sched:acc:1`、`sched:acc:2`、`sched:acc:3` 已更新为 `grok`、`anthropic`、`anthropic`，且调度快照不含明文凭证标记。
- 应用容器保持 `healthy`；本地入口、本地 Nginx 和三个公网 `/health` 均返回 HTTP 200。
