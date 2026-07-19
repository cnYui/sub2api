# 运行态备份与 GitHub 推送计划

## 目标

- 备份当前 `sub2api-candidate` 运行态 PostgreSQL 数据库和 Redis 数据库，便于另一台电脑恢复完整服务。
- 将当前本地 `main` 分支代码提交并推送到个人 fork `personal`，不推送上游 `origin`。

## 边界

- 只读导出 PostgreSQL 和 Redis，不修改数据库业务数据、Redis 业务键、Nginx 或公网链路。
- 数据库备份包含用户、订单、用量等敏感业务数据，只保存在本机备份目录，不提交到 Git。
- 不在文档、提交信息或回复中写入完整 API Key、token、HMAC secret、SMTP 密码、支付密钥。

## 备份方案

- 备份目录：`/Users/wujianxiang/CodeSpace/sub2api-runtime-backups/20260719-101835/`
- PostgreSQL：
  - 使用 `pg_dump -Fc` 导出 `sub2api-candidate-postgres` 的 `sub2api` 数据库。
  - 使用 `pg_restore -l` 校验 dump 可读取。
- Redis：
  - 查询 Redis 持久化目录和文件名。
  - 执行 `redis-cli SAVE` 生成一致的 RDB 快照。
  - `docker cp` 导出 RDB，并用 `gzip` 压缩。
  - 使用 `redis-check-rdb` 校验 RDB 可读取。
- 生成 `SHA256SUMS` 与恢复说明。

## Git 推送方案

- 先检查工作区差异和未跟踪文件。
- 扫描即将提交内容中的敏感关键词，发现明显 secret 则停止。
- 备份目录位于仓库外，不提交数据库或 Redis 备份文件。
- 提交当前本地代码和上下文文档。
- 推送当前 `main` 到 `personal/main`。

## 回滚边界

- 备份步骤不修改运行态数据；若备份失败，删除未完成备份目录后重跑。
- Git 提交后如需撤销，只在用户明确要求时再做 revert 或新提交修正；不执行 destructive reset。
