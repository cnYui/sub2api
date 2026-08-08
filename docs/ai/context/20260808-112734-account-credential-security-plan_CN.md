# 上游账号凭证安全改造实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**目标：** 移除 Redis 调度快照中的敏感凭证，并让 PostgreSQL 账号凭证在服务端透明加密、可迁移、可验证。

**架构：** 调度缓存只保存非敏感账号元数据；选中账号后由 `SchedulerSnapshotService` 通过 `AccountRepository` 回源读取解密凭证。账号仓储增加版本化字段加密、密钥配置和兼容明文迁移，使用带密钥的凭证指纹维持 CAS 语义，避免 SQL 再比较随机密文。

**技术栈：** Go、AES-256-GCM、PostgreSQL JSONB/迁移 SQL、Redis、现有 Ent/SQL 仓储和 testify/sqlmock 测试。

---

### 任务 1：凭证加密配置与纯函数编解码器

**文件：**

- 新建：`backend/internal/repository/account_credentials_crypto.go`
- 新建：`backend/internal/repository/account_credentials_crypto_test.go`
- 修改：`backend/internal/config/config.go`
- 修改：`backend/internal/repository/aes_encryptor.go`

- [ ] **步骤 1：先写失败测试**

在 `account_credentials_crypto_test.go` 添加以下行为：敏感键被编码为 `enc:v1:`，非敏感键保持原 JSON 类型；解码恢复原值；错误密钥、错误前缀和篡改密文均返回错误；同一明文两次加密产生不同密文。

```go
func TestCredentialCodecEncryptsOnlySensitiveValues(t *testing.T) {
	codec := newCredentialCodecForTest(t)
	input := map[string]any{
		"api_key":       "sk-test",
		"refresh_token": "refresh-test",
		"base_url":      "https://example.test",
		"model_mapping": map[string]any{"m": "m"},
	}
	stored, err := codec.EncryptMap(input)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(stored["api_key"].(string), "enc:v1:"))
	require.Equal(t, input["base_url"], stored["base_url"])
	require.NotEqual(t, "sk-test", stored["api_key"])
	got, err := codec.DecryptMap(stored)
	require.NoError(t, err)
	require.Equal(t, input, got)
}
```

- [ ] **步骤 2：运行测试确认 RED**

运行 `go -C backend test ./internal/repository -run TestCredentialCodecEncryptsOnlySensitiveValues -count=1`，预期因 `credentialCodec` 尚不存在而失败。

- [ ] **步骤 3：实现最小编解码器**

在新文件中定义 `credentialCodec`、`NewCredentialCodec(activeKey, legacyKeys)`、`EncryptMap`、`DecryptMap`。敏感键集合至少包含 `api_key`、`access_token`、`refresh_token`、`id_token`、`client_secret`、`private_key`、`cookie`、`cookies`、`setup_token`、`session_token`、`token`；嵌套对象按 JSON 序列化后整体加密。密文格式固定为 `enc:v1:` 加 base64(AES-GCM nonce+ciphertext+tag)。解码仅接受版本前缀，旧明文在迁移兼容模式中原样返回并标记需要回写。

在 `Config` 增加 `AccountCredentials` 配置结构，支持 `ACCOUNT_CREDENTIALS_ENCRYPTION_KEY_FILE`；读取文件时去除首尾空白，必须是 64 位十六进制。配置加载不自动生成该密钥，生产缺失直接报错。将 AES-GCM 的通用构造从 `NewAESEncryptor` 抽出到共享内部函数，保持 TOTP 行为不变。

- [ ] **步骤 4：运行测试确认 GREEN**

运行 `go -C backend test ./internal/repository -run 'TestCredentialCodec|TestNewAESEncryptor' -count=1`，预期全部通过。

- [ ] **步骤 5：提交**

```powershell
git add backend/internal/repository/account_credentials_crypto.go backend/internal/repository/account_credentials_crypto_test.go backend/internal/config/config.go backend/internal/repository/aes_encryptor.go
git commit -m "feat: 增加账号凭证加密编解码器"
```

### 任务 2：账号仓储统一加密读写与指纹 CAS

**文件：**

- 新建：`backend/migrations/206_account_credentials_security.sql`
- 修改：`backend/internal/repository/account_repo.go`
- 修改：`backend/internal/repository/account_repo_ollama_cloud_usage.go`
- 新建：`backend/internal/repository/account_repo_credentials_test.go`
- 修改：`backend/internal/repository/account_repo_temp_unsched_test.go`
- 修改：`backend/internal/repository/account_repo_upstream_billing_probe_cas_test.go`

- [ ] **步骤 1：先写失败仓储测试**

测试创建/读取会透明加解密，更新和批量更新不会写入敏感明文；读取旧明文账号会返回原值并可被迁移；`UpdateGrokOAuthCredentialsIfUnchanged` 使用解密后的规范 JSON 指纹完成 compare-and-set。

```go
func TestAccountRepositoryPersistsEncryptedCredentialsAndHydratesPlaintext(t *testing.T) {
	repo := newAccountRepositoryWithSQLAndCredentialCodec(t, testCredentialCodec(t))
	account := &service.Account{Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "sk-test", "base_url": "https://example.test"}}
	require.NoError(t, repo.Create(context.Background(), account))
	stored := readRawAccountCredentials(t, account.ID)
	require.NotContains(t, stored, "sk-test")
	got, err := repo.GetByID(context.Background(), account.ID)
	require.NoError(t, err)
	require.Equal(t, "sk-test", got.GetCredential("api_key"))
}
```

- [ ] **步骤 2：运行测试确认 RED**

运行 `go -C backend test ./internal/repository -run 'TestAccountRepositoryPersistsEncryptedCredentialsAndHydratesPlaintext' -count=1`，预期因仓储尚未注入 codec 而失败。

- [ ] **步骤 3：实现仓储边界**

给 `accountRepository` 增加 `credentialCodec *credentialCodec`，构造函数接收 codec；`createAccountRecord`、`updateAccount`、`UpdateCredentials`、`BulkUpdate`、OAuth 刷新和 Ollama 更新路径统一调用 `EncryptMap`，`accountsToService` 统一调用 `DecryptMap`。保留 `account.Credentials` 的服务层明文语义，禁止把密文泄露到 handler。

迁移 SQL 为 `accounts.credentials_fingerprint TEXT NOT NULL DEFAULT ''` 创建索引，并为历史行填充空指纹。指纹由规范化解密 JSON 经 HMAC-SHA256 生成；写入时同步更新。所有原来 `a.credentials = $n::jsonb` 的 CAS 改为 `a.credentials_fingerprint = $n`，并把 `refresh_token` 是否为空等判断移到事务内的 Go 解密检查，避免对密文 JSON 做明文 SQL 过滤。Ollama 按账号 ID 批量加载并在 Go 中按解密后的 API Key 分组，禁止用 `credentials ->> 'api_key'` 查询。

对已有明文记录采用兼容读取：读取成功后在同一事务回写密文和指纹；若密文解密失败则返回可识别错误并停止请求，不回退到猜测值。

- [ ] **步骤 4：运行仓储定向测试**

运行 `go -C backend test ./internal/repository -run 'TestAccountRepository|Test.*GrokOAuth.*Unchanged|Test.*Ollama' -count=1`，预期通过。

- [ ] **步骤 5：提交**

```powershell
git add backend/migrations/206_account_credentials_security.sql backend/internal/repository/account_repo.go backend/internal/repository/account_repo_ollama_cloud_usage.go backend/internal/repository/account_repo_credentials_test.go backend/internal/repository/account_repo_temp_unsched_test.go backend/internal/repository/account_repo_upstream_billing_probe_cas_test.go
git commit -m "feat: 统一加密账号凭证仓储读写"
```

### 任务 3：移除 Redis 凭证并恢复调度回源

**文件：**

- 修改：`backend/internal/repository/scheduler_cache.go`
- 修改：`backend/internal/service/scheduler_snapshot_service.go`
- 修改：`backend/internal/service/scheduler_snapshot_hydration_test.go`
- 修改：`backend/internal/repository/scheduler_cache_unit_test.go`

- [ ] **步骤 1：先写失败测试**

断言 `marshalSchedulerCacheAccount` 的完整和元数据 JSON 都不包含敏感键或测试 Key；断言 `SchedulerSnapshotService.GetAccount` 命中缓存后仍调用 `AccountRepository.GetByID`，并把回源对象作为返回值。

```go
func TestMarshalSchedulerCacheAccountNeverStoresCredentials(t *testing.T) {
	full, meta, err := marshalSchedulerCacheAccount(service.Account{
		ID: 7, Credentials: map[string]any{"api_key": "sk-secret", "model_mapping": map[string]any{"m": "m"}},
	})
	require.NoError(t, err)
	require.NotContains(t, string(full), "sk-secret")
	require.NotContains(t, string(meta), "api_key")
}
```

- [ ] **步骤 2：运行测试确认 RED**

运行 `go -C backend test ./internal/repository -run TestMarshalSchedulerCacheAccountNeverStoresCredentials -count=1`，预期当前实现因完整快照仍序列化凭证而失败。

- [ ] **步骤 3：实现缓存与回源**

删除 `filterSchedulerCredentials` 及其调用，`buildSchedulerMetadataAccount` 的 `Credentials` 固定为 `nil`；`writeAccountIDs` 只写非敏感 payload。`schedulerCache.GetAccount` 只解析元数据并返回不带凭证的对象。`SchedulerSnapshotService.GetAccount` 在缓存命中后始终调用 `accountRepo.GetByID`，再把缓存中的限流/时间状态合并到回源账号，确保请求路径得到解密凭证；回源失败按现有 fallback 超时和错误语义处理。

- [ ] **步骤 4：清理旧 Redis 快照函数与测试**

新增 `DeleteSensitiveSchedulerSnapshots(ctx)`，按 `sched:acc:*`、`sched:meta:*`、`sched:*:v*` 前缀删除旧完整快照；服务启动时只执行一次受控清理，不扫描其它业务键。测试使用 miniredis/现有 Redis fixture 验证清理后不存在旧 Key。

- [ ] **步骤 5：运行测试并提交**

运行 `go -C backend test ./internal/repository ./internal/service -run 'Test.*Scheduler|Test.*Snapshot|TestMarshalScheduler' -count=1`，再提交：

```powershell
git add backend/internal/repository/scheduler_cache.go backend/internal/repository/scheduler_cache_unit_test.go backend/internal/service/scheduler_snapshot_service.go backend/internal/service/scheduler_snapshot_hydration_test.go
git commit -m "fix: 从 Redis 调度快照移除账号凭证"
```

### 任务 4：一次性明文迁移命令与启动接线

**文件：**

- 新建：`backend/cmd/migrate-account-credentials/main.go`
- 修改：`backend/cmd/server/wire_gen.go`
- 修改：`backend/internal/repository/wire.go`
- 修改：`deploy/docker-compose.18082.yml`
- 新建：`scripts/migrate-account-credentials.ps1`

- [ ] **步骤 1：先写迁移行为测试**

为迁移函数添加内存/SQL mock 测试：明文行加密一次、密文行跳过、失败事务回滚、重复运行数量为零变化；日志只包含处理数量与错误分类。

- [ ] **步骤 2：运行测试确认 RED**

运行 `go -C backend test ./cmd/migrate-account-credentials ./internal/repository -run 'Test.*CredentialMigration' -count=1`，预期因迁移命令和 codec 注入尚不存在而失败。

- [ ] **步骤 3：实现迁移与依赖注入**

迁移命令读取同一配置和数据库连接，按 `id` 升序分页，每批在事务中读取、解密兼容明文、写入密文和 HMAC 指纹；任何一条解密或写入错误回滚当前批次并以非零退出。服务 Wire 注入专用 credential codec 到账号仓储和 scheduler snapshot service。Compose 增加只读 Secret 文件挂载和 `ACCOUNT_CREDENTIALS_ENCRYPTION_KEY_FILE`，不把 Key 原文写入 YAML。

- [ ] **步骤 4：实现运维脚本与迁移前检查**

PowerShell 脚本先检查服务停止、数据库可连、Secret 文件存在，再调用迁移命令；输出账号数量和失败 ID，不输出 JSON 凭证内容。脚本默认 dry-run，显式 `-Apply` 才写入。

- [ ] **步骤 5：运行测试并提交**

运行 `go -C backend test ./cmd/migrate-account-credentials ./internal/repository -run 'Test.*CredentialMigration|TestCredentialCodec' -count=1`，并执行 `go -C backend build ./cmd/server ./cmd/migrate-account-credentials` 后提交：

```powershell
git add backend/cmd/migrate-account-credentials/main.go backend/cmd/server/wire_gen.go backend/internal/repository/wire.go deploy/docker-compose.18082.yml scripts/migrate-account-credentials.ps1
git commit -m "feat: 增加账号凭证加密迁移与部署配置"
```

### 任务 5：安全回归、文档和最终验证

**文件：**

- 新建：`docs/ai/context/20260808-130000-account-credential-security-implementation_CN.md`
- 修改：`AGENTS.md`

- [ ] **步骤 1：运行静态凭证扫描**

运行 `rg -n --glob '*.go' --glob '*.sql' --glob '*.yml' 'scheduler.*api_key|filterSchedulerCredentials|credentials ->> ''api_key''' backend deploy`，预期只剩迁移说明/测试断言，不得有生产 Redis 写入或 PostgreSQL 明文 API Key 查询。

- [ ] **步骤 2：运行完整验证**

依次运行 `go -C backend test ./internal/repository ./internal/service ./cmd/migrate-account-credentials -count=1`、`go -C backend build ./cmd/server ./cmd/migrate-account-credentials` 和 `go -C backend test ./... -count=1`。记录每条命令的退出码和测试数量；超时只记录超时，不宣称通过。

- [ ] **步骤 3：记录部署前置条件**

上下文文档写明新 Secret 文件、迁移命令、回滚方式、Redis 旧快照清理范围、不得输出凭证原文，并在 `AGENTS.md` 增加本次改造的日期和文档链接。

- [ ] **步骤 4：提交验证记录**

```powershell
git add docs/ai/context/*account-credential-security-implementation_CN.md AGENTS.md
git commit -m "docs: 记录账号凭证安全改造验证"
```
