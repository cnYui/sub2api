# 禁用 luzhiyuan2026@163.com API Key 计划

## 目标

- 禁用 `api_keys.id=41`，阻断 `luzhiyuan2026@163.com` 继续通过该 Key 调用模型 API。
- 不改用户状态、不改流量卡余额、不改历史 usage、不中断其他用户。

## 当前事实

- 用户：`users.id=35 / luzhiyuan2026@163.com`
- API Key：`api_keys.id=41 / codex_used`
- 当前状态：`active`
- 当前运行库：`sub2api-postgres-dev / sub2api`
- 当前 Redis：`sub2api-redis-dev`

## 操作步骤

1. 备份 `api_keys.id=41` 当前完整行到 `backups/`。
2. 验证备份文件可读，且只包含目标行。
3. 执行 DB 更新：`status='disabled'`，刷新 `updated_at`。
4. 删除该 Key 对应的认证缓存 `apikey:auth:<sha256(key)>`。
5. 复核 DB 状态和 Redis 缓存删除结果。

## 回滚边界

- 若需要恢复，只把 `api_keys.id=41` 的 `status` 改回备份中的 `active`，并再次删除该 Key 的认证缓存。
- 不回滚或修改用户、套餐、流量卡、usage 事实。
