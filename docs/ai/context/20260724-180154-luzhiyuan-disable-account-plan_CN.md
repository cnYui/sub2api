# luzhiyuan2026@163.com 账号禁用运行态计划

## 目标

- 立即阻断 `luzhiyuan2026@163.com` 继续通过 Sub2API 使用模型接口。

## 范围

- 目标用户：`users.id=35 / luzhiyuan2026@163.com`。
- 目标 API Key：`api_keys.id=41 / codex_used`。
- 只修改 `users.status` 与该用户未删除 API Key 的 `api_keys.status`。

## 备份

- 变更前导出目标用户与 API Key 状态快照到 `backups/20260724-180154-luzhiyuan-disable-account-prechange.sql`。
- 备份只包含回滚所需字段，不导出完整 API Key 明文。

## 回滚边界

- 如需回滚，将 `users.id=35` 的 `status` 恢复为备份中的旧值。
- 将 `api_keys.user_id=35` 且 `deleted_at IS NULL` 的 Key 状态恢复为备份中的旧值。
- 不回滚历史 usage、traffic credit、reservation 或套餐记录。
