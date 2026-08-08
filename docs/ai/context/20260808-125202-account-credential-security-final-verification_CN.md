# 上游账号凭证安全改造最终验证

## 范围与边界

- 分支：`codex/secure-account-credentials`；工作在隔离工作树中，未触碰主工作区已有改动。
- 本轮实现不启动、重建或停止任何服务；不访问生产数据库、Redis 或上游 API，也不读取、复制或输出真实 API Key。
- 本次不执行明文迁移。上线前必须由运维人员准备 Docker Secret，应用停止后先运行迁移脚本 dry-run，再显式使用 `-Apply`。

## 已验证行为

- Redis 调度快照不存储敏感凭证；账号选中后按 ID 回源仓储，以得到解密后的凭证。
- `accounts.credentials` 的敏感字段采用 AES-256-GCM 的 `enc:v1:<base64>` 格式存储；HMAC 指纹用于 CAS 和 Ollama 同 Key 查询，避免随机密文参与相等比较。
- 迁移命令默认 dry-run；迁移批次在事务中锁定、检查、重写，损坏密文会报错且回滚当前批次。
- Compose 配置通过 Docker Secret 提供密钥文件，不在 YAML、环境变量值或日志中保存密钥原文。

## 命令与结果

| 命令 | 结果 |
| --- | --- |
| `go -C backend test -tags unit ./internal/repository -count=1` | 通过（3.747s） |
| `go -C backend test ./cmd/migrate-account-credentials ./internal/config ./internal/handler/dto -count=1` | 通过（各包 1.155s–1.290s） |
| `go -C backend test -tags unit ./internal/service -run 'Test(OpenAI\|Gateway)SelectAccountWithLoadAwareness.*SchedulerSnapshot' -count=1` | 通过（1.484s） |
| `go -C backend build ./cmd/server ./cmd/migrate-account-credentials` | 通过 |
| `docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.18082.yml config --quiet` | 通过；仅使用占位的 `POSTGRES_PASSWORD` 和 Secret 文件路径，未启动容器 |
| `gofmt -d`（本轮 Go 文件） | 无输出，通过 |
| Redis/SQL 静态扫描 | 仅命中回归测试中故意保留的旧明文断言；生产代码没有调度 API Key 写入或 `credentials ->> 'api_key'` 查询 |
| `go -C backend test ./... -count=1` | 64.029s 超时（退出码 124）；未将其表述为通过 |

## 已知非本轮失败

`go -C backend test -tags unit ./internal/service -count=1` 在 40.208s 后失败。失败测试是
`TestCheckBillingEligibility_SubscriptionMode_BypassesPlatformQuota`，原因是其 `fakeZeroQuotaCache` 嵌入的 `BillingCache` 为 nil，而被测路径调用了该嵌入接口的 `GetUserBalance`，触发空指针。

该测试文件和相关计费代码均不在本分支改动范围内；与 `main...HEAD` 的服务层差异只包含本轮调度快照回源实现和其测试。因此未修改该无关测试或计费逻辑，不能将服务层全量测试表述为通过。

## 后续上线顺序

1. 在宿主机生成 32 字节随机密钥并以 64 位十六进制保存到只读 Secret 文件。
2. 设置 `ACCOUNT_CREDENTIALS_ENCRYPTION_KEY_HOST_FILE`，确认 18082 应用容器已停止。
3. 执行 `scripts/migrate-account-credentials.ps1`，核对 dry-run 数量和错误分类。
4. 审批后执行 `scripts/migrate-account-credentials.ps1 -Apply`。
5. 使用同一 Secret 启动新镜像；检查日志中没有凭证内容，并验证账号调度、OAuth 刷新和 Ollama 查询。

## 遗留事项

- `credentialCodec` 具备旧密钥解密能力，但本轮未接入旧密钥 Secret 与轮换流程；密钥轮换需单独设计离线重加密和回滚方案。
- 调度快照的 `Credentials` 字段仍承载 `model_mapping`、`plan_type` 等非敏感调度元数据，不包含 API Key、Token、Cookie 或私钥。
