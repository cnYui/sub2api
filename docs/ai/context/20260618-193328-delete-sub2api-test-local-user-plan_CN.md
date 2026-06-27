# 删除 `sub2api-test-local@example.com` 测试用户执行计划

## 背景

用户截图中要求删除套餐列表里的测试用户，界面显示邮箱为：

```text
sub2api-test-local@example.com
```

## 删除边界

- 目标用户：`users.email = 'sub2api-test-local@example.com'`
- 当前用户 ID：`2`
- 当前角色：`user`
- 当前 API Key：1 个，掩码 `sk-398143e...e649c4`
- 当前订阅：1 条，分组 `codex-pool-19-usd`
- 明确不处理本机管理员账号 `15951875192@phone.com`

## 执行方案

优先复用 `AdminService.DeleteUser()` 的业务语义：

1. 删除前确认目标用户存在且不是 `admin`。
2. 备份当前 PostgreSQL 数据库。
3. 在单个数据库事务内：
   - 将目标用户未删除 API Key 写入 `deleted_api_key_audits`。
   - 将目标用户未删除 API Key 软删除，并用 tombstone 覆盖 `api_keys.key`。
   - 软删除目标用户。
4. 删除后清理认证缓存。
5. 验证管理列表和 API Key 均不可继续使用。

## 风险控制

- 不打印完整 API Key。
- 不删除或修改其它用户。
- 如果角色不是 `user` 或目标邮箱匹配不到唯一用户，停止执行。
- 保留数据库备份文件以便回滚。
