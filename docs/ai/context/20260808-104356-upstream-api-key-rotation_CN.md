# 上游 API Key 轮换执行记录

## 范围

- 来源：用户已登录的 `https://api.ai-genesis.app/keys` 页面。
- 本地目标：10 个 active 的 OpenAI API Key 账号。
- 未处理：上游 `Qwen` Key；当前本地没有同名或对应的账号/分组，未擅自新建渠道。

| 上游名称 | 本地账号 ID |
| --- | ---: |
| Gpt0.35 | 1129 |
| Gpt0.15 | 1128 |
| Gpt0.1 | 1130 |
| Deepseek | 6 |
| GLM | 4 |
| ClaudeMax | 2 |
| Grok | 1 |
| ClaudeKiro | 3 |
| kimi | 5 |
| GPT生图 | 1131 |

## 执行

- 用户授权后，通过每张上游 Key 卡片的复制按钮逐条读取新 Key，未在终端、文档或对话中输出 Key 原文。
- 因容器未注入可用的管理员初始密码，无法安全地使用管理员 HTTP 接口；在应用停机期间直接更新 PostgreSQL `accounts.credentials.api_key`，保留 `base_url`、`model_mapping` 及其余凭证字段。
- 每条更新均写入 `scheduler_outbox.account_changed`，并同步 Redis `sched:acc:<account_id>` 快照中的凭证，避免服务恢复后使用旧 Key。
- 首次浏览器自动点击未切换系统剪贴板，导致所有目标一度被同一值覆盖；已在服务仍停机时改用真实页面点击逐条覆盖并完成最终核验，旧错误值未被用于公网请求。

## 最终核验

- PostgreSQL：10 个目标账号均存在，Key 长度均为 67，10 个 Key 指纹互不相同。
- Redis：10 个 `sched:acc:*` 快照均与数据库对应账号的 Key 指纹一致，无不匹配项。
- 本地应用曾只监听 `127.0.0.1:18082` 以处理调度 outbox，健康检查为 200；未恢复公网入口。
- 最终状态：`sub2api-official-18082`、`sub2api-public-nginx-local` 均已停止，Cloudflare Tunnel 进程不存在，端口 `8080`/`18082` 无监听。
