# 分组 9 上游凭证恢复验证

## 验证结果

- 分组 9 的账号 `1128` 上游地址为 `https://api.ai-genesis.app`。
- 管理员提供的文本包含同一段 API Key 连续拼接两次；取单段文本请求上游 `/v1/models` 返回 HTTP `200`，模型数据正常。
- 将原始重复文本直接请求上游返回 HTTP `401`，错误码为 `INVALID_API_KEY`。
- 因此此前账号 `1128` 的 `Authentication failed (401): API key is disabled` 是重复凭证导致的上游鉴权失败，不能据此判断单段 Key 已被上游禁用。

## 已构建操作

- 已构建一次性恢复工具 `backend/cmd/restore-account-1128`，使用项目现有凭证加密与账号仓储路径，计划写入单段 Key、恢复账号 `active + schedulable`、清理错误/冷却字段并同步调度缓存。
- 当前会话执行 `docker cp` 时被 Docker named pipe 权限拒绝，恢复工具尚未进入生产容器，数据库和 Redis 未发生本次操作的写入。

本记录不保存 API Key 原文、哈希或可逆凭证。
