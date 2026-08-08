# 上游账号凭证安全改造实现记录

## 改造范围

- Redis `sched:acc:*` 与 `sched:meta:*` 只保留调度元数据，不再写入 API Key、OAuth Token、Cookie 或私钥；服务选中账号后按 ID 回源仓储，仓储负责解密。
- PostgreSQL `accounts.credentials` 保持 JSONB 结构。敏感字段使用 AES-256-GCM，密文格式为 `enc:v1:<base64>`；凭证文档和 API Key 另存 HMAC 指纹，用于 CAS 与分组查询，不保存明文查询条件。
- 账号创建、更新、OAuth 刷新、批量凭证合并、账号读取和影子账号读取统一经过仓储编解码边界。
- Ollama 云账号按 `_api_key_fingerprint` 查询、分组和聚合，随机密文不会破坏账号身份判断。

## 密钥与迁移

1. 在宿主机生成 32 字节随机值并保存为 64 位十六进制 Secret 文件，权限设为仅服务账号可读；不要把原文写入仓库、Compose 或日志。
2. 设置 `ACCOUNT_CREDENTIALS_ENCRYPTION_KEY_HOST_FILE` 指向该文件。Compose 将其只读挂载到 `/run/secrets/account_credentials_encryption_key`，服务读取 `ACCOUNT_CREDENTIALS_ENCRYPTION_KEY_FILE`。
3. 确认应用端口已停止后先执行 dry-run：

   ```powershell
   .\scripts\migrate-account-credentials.ps1
   ```

4. 核对待迁移数量后显式执行写入：

   ```powershell
   .\scripts\migrate-account-credentials.ps1 -Apply
   ```

   命令按 ID 升序分批、每批事务写入，重复执行时已加密且指纹完整的记录会跳过；失败批次回滚。输出只包含数量和错误分类，不输出凭证值。
5. 使用新 Compose 启动应用。服务初次重建调度快照时会清理 `sched:acc:*`、`sched:meta:*` 的历史账号 JSON，只保留 `sched:acc:last_used:*` 状态键。

## 回滚与注意事项

- 回滚前必须保留加密密钥文件；没有该密钥无法读取已加密凭证。
- 若迁移尚未执行，可回到旧镜像读取旧明文；迁移完成后不要在未注入同一密钥的旧镜像上启动写入路径。
- 本轮没有启动公网服务、重建生产容器、访问生产数据库或读取任何真实 API Key。
- `credentialCodec` 为空仅用于兼容现有单元测试和离线旧路径；服务 Wire 在启动时要求有效的 32 字节加密密钥。

## 验证记录

- `go -C backend test -tags unit ./internal/repository -count=1`：通过。
- `go -C backend test ./internal/config ./cmd/migrate-account-credentials -count=1`：通过。
- `go -C backend build ./cmd/server ./cmd/migrate-account-credentials`：通过。
- Ollama 指纹 SQL 定向测试与全仓储单元测试：通过。
- 全量 `go -C backend test ./... -count=1` 需在最终验证阶段运行；若超过执行窗口，仅记录超时，不将其表述为通过。
