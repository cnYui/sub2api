# OpenAI 原子计费实施基线失败记录

时间：2026-07-28 17:05:17 +09:00

## 执行范围

已从 `main` 的 `a44db95e2` 创建隔离 worktree `D:\CodeWorkSpace\sub2api-openai-billing`，分支为 `codex/openai-billing-atomic-hold`。原工作区的未提交前端改动未带入。

## 基线命令

```powershell
Set-Location D:\CodeWorkSpace\sub2api-openai-billing\backend
go test -tags=unit ./internal/service ./internal/repository ./internal/server -count=1
```

为取得可复现的最小错误，进一步执行：

```powershell
go test -tags=unit ./internal/server -count=1
```

## 结果

基线在编译 `internal/server` 测试时失败，尚未修改本次计费代码。`api_contract_test.go` 中的既有测试桩未跟随接口演进：

- `stubApiKeyRepo` 缺少 `service.APIKeyRepository.GetActiveBySHA256Hash`。
- `stubUserSubscriptionRepo` 缺少 `service.UserSubscriptionRepository.HardDelete`。

失败位置为 `internal/server/api_contract_test.go:1310`、`:1315`、`:1324`、`:2692`、`:2695`。

## 处理边界

此失败早于 OpenAI 原子 hold 的任何代码修改，不能归因于本任务。按实施计划与执行规则暂停后续实现，等待确认是否允许把测试桩接口同步作为独立基线修复后继续。
