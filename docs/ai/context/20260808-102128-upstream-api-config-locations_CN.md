# 上游 API 配置位置审计

## 结论

- 当前生产实例使用 `deploy/docker-compose.dev.yml` 与 `deploy/docker-compose.18082.yml`，项目名为 `deploy`。
- 上游账号凭证不在源码或 `docker-compose.18082.yml` 中，主要存储在 PostgreSQL 的 `accounts.credentials` JSONB 字段。
- 当前数据库有 10 个未删除且 active 的 `openai/apikey` 账号；每个账号均包含 `api_key`、`base_url` 和 `model_mapping`。当前唯一上游地址为 `https://api.ai-genesis.app`。密钥内容未写入本记录。
- 调度器 Redis 会保存账号快照，键前缀为 `sched:acc:` 与 `sched:*`；快照包含 `Credentials`。本次只读核验发现 10 个账号快照键，不能只改数据库后直接启动而忽略缓存一致性。

## 配置位置

1. 账号名称、平台、认证类型、API Key、上游 Base URL、模型映射：PostgreSQL `accounts` 表的 `credentials` JSONB；账号与分组关系在 `account_groups`，分组倍率和展示名称在 `groups`。
2. 账号代理：`accounts.proxy_id` 关联 `proxies` 表；当前实例未配置代理记录。
3. 运行时配置和数据库连接：`deploy/data-18082/config.yaml`；容器挂载到 `/app/data`。该文件包含数据库/JWT 等运行凭证，不是当前这批上游 API Key 的主存储。
4. 容器环境注入：`deploy/docker-compose.dev.yml`、`deploy/docker-compose.18082.yml` 及 `deploy/.env`。当前运行环境没有发现 `CLAUDE_API_KEY`、`GEMINI_API_KEY` 或类似上游 API Key 环境变量；HTTP/HTTPS/ALL_PROXY 只控制出站代理。
5. 管理界面：前端 `frontend/src/views/admin/AccountsView.vue` 与 `frontend/src/components/account/EditAccountModal.vue`；后端接口为 `PUT /api/v1/admin/accounts/:id`。更新凭证时服务端会合并脱敏字段、写入数据库，并刷新调度器账号快照。

## 轮换注意事项

- 先在上游服务撤销旧 Key 并生成新 Key；仅修改本地数据库不能阻止已泄露的旧 Key 被直接调用。
- 当前公网服务已停止。建议在保持公网入口关闭的前提下，通过本地管理员接口/界面逐个或批量更新 `accounts.credentials.api_key` 与必要的 `base_url`，再由服务同步 Redis 调度快照。
- 不要把数据库 `credentials` 原文、Redis `sched:acc:*` 值、`deploy/.env` 或 `data-18082/config.yaml` 内容复制到聊天、工单或 Git。
