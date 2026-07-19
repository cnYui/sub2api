# 运行态备份与 GitHub 推送结果

## 备份结果

- 备份目录：`/Users/wujianxiang/CodeSpace/sub2api-runtime-backups/20260719-101835/`
- PostgreSQL：
  - 文件：`sub2api-20260719-101835.pgcustom.dump`
  - 大小约 `96M`
  - 使用容器内 `pg_restore -l` 校验 TOC 可读，通过。
- Redis：
  - 文件：`redis-20260719-101835.dump.rdb`
  - 压缩文件：`redis-20260719-101835.dump.rdb.gz`
  - 使用 `redis-check-rdb` 校验通过，读取 `941` 个 key，checksum OK。
- 校验清单：`SHA256SUMS`
- 恢复说明：`RESTORE_CN.md`

## 代码检查

- `git diff --check` 通过。
- staged diff 敏感关键词扫描未发现真实 secret；仅命中环境变量名 `YUI_USAGE_EVENT_HMAC_SECRET`。
- 后端全量测试通过：在 `backend/` 执行 `go test ./...`。

## 推送边界

- 数据库和 Redis 备份位于仓库外，没有提交到 Git。
- 代码按项目记忆推送到个人 fork `personal`，不推送上游 `origin`。
