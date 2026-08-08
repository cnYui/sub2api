# Ollama Cloud 用量查询改用 API Key 指纹

## 背景

账号凭证启用 `CredentialCodec` 后，`credentials.api_key` 存储为随机密文；用 PostgreSQL 对该字段做明文比较或按密文做等值 CAS 都无法稳定命中。API Key 的可查询材料是 codec 写入的 `_api_key_fingerprint` HMAC，整段凭证并发校验使用 `_credential_fingerprint`。

## 实现

- `accountRepository.apiKeyLookupArg` 在 codec 启用时将输入 API Key 转为 HMAC，未启用时原样返回。
- `accountRepository.apiKeyLookupExpression` 在 codec 启用时读取 `_api_key_fingerprint`，未启用时读取 `api_key`。
- Ollama Cloud 分组查询、组锁定/更新、到期扫描统一使用上述参数和表达式；codec 启用时资格条件只检查指纹存在，不访问明文 API Key。
- 组锁定的凭证 CAS 改为 `credentialCASArg` 与 `credentialCASCondition`，避免比较随机 AES 密文。
- codec 为 nil 的 SQL 与参数保持原有明文测试语义。

## 验证

先新增 `TestOllamaCloudUsageQueriesUseAPIKeyFingerprintWithCredentialCodec`，运行定向测试得到预期 RED：三条 SQL 路径仍生成 `credentials ->> 'api_key'`，锁定参数仍为明文 API Key。

实现后执行：

```text
go test -tags unit ./internal/repository -run TestOllamaCloudUsageQueriesUseAPIKeyFingerprintWithCredentialCodec -count=1
go test -tags unit ./internal/repository -run 'Test.*OllamaCloudUsage' -count=1
go test -tags unit ./internal/repository -count=1
```

三条命令均通过；未接触真实密钥、外部服务或生产数据库。
