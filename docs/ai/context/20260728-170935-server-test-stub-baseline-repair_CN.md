# Server 测试桩基线修复记录

时间：2026-07-28 17:09:35 +09:00

## 根因

`UserSubscriptionRepository.HardDelete` 已在 `9241cc1ec` 加入接口，`APIKeyRepository.GetActiveBySHA256Hash` 也已成为现行契约，但 `internal/server/api_contract_test.go` 中的共享测试桩未同步，因此 `internal/server` 在执行测试前无法编译。

## 最小修复

- 为 `stubUserSubscriptionRepo` 补充不支持调用的 `HardDelete` 实现。
- 为 `stubApiKeyRepo` 补充仅返回 active API Key 的 SHA-256 查找，与生产仓储的查询语义一致。

仅修改隔离 worktree 的测试文件；不读取或写入 Docker、PostgreSQL、Redis、Nginx、`18080`、`18086` 或公网链路。

## 验证

```powershell
Set-Location D:\CodeWorkSpace\sub2api-openai-billing\backend
gofmt -w internal/server/api_contract_test.go
go test -tags=unit ./internal/server -run '^$' -count=1
```

结果：通过，`internal/server` 已恢复测试编译。

完整 `go test -tags=unit ./internal/server -count=1` 仍存在既有 API contract 快照失败：管理员设置接口期望的若干 `auth_source_default_*_subscriptions` 空数组与实际 `null` 不一致；该失败与本次接口测试桩修复无关，后续 OpenAI 计费任务不依赖该接口。
