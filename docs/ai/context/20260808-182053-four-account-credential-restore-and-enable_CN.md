# 四个 OpenAI API Key 账号凭证恢复与启用

## 变更范围

- 账号 `3`：Claude Kiro
- 账号 `1129`：GPT 0.35
- 账号 `1130`：GPT 0.1
- 账号 `1131`：GPT-Image-2

用户提供了四个新的上游 API Key。本记录不保存 Key 原文、哈希原文或可逆凭证。

## 执行

- 先对四个 Key 请求上游 `GET /v1/models`，均返回 HTTP `200`，并分别命中 Claude、GPT 和 GPT-Image-2 的目标模型。
- 通过项目现有 `CredentialCodec` 写入账号凭证，保留原有 `base_url`、`model_mapping` 和分组配置；数据库中的 `api_key` 以 `enc:v1:` 加密保存，并更新 HMAC 指纹。
- 清除账号错误、限流、模型限流和临时不可调度状态。
- 将四个账号设置为 `status=active`、`schedulable=true`，写入 `scheduler_outbox.account_changed` 并同步 Redis 调度快照。

## 核验

- PostgreSQL：四个账号均为 `active`，`schedulable=true`，`error_message` 为空；凭证字段为加密格式。
- Redis：四个 `sched:acc:<id>` 快照均为 `active`/可调度，快照不含 `api_key` 字段。
- 应用健康检查：`http://127.0.0.1:18082/health` 返回 HTTP `200`。
- 本次只执行不计费的模型目录鉴权探测，未发起聊天或生图请求。
