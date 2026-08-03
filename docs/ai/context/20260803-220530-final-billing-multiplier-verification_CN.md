# 最终计费倍率验证记录

## 已验证

- `go build ./internal/service ./internal/config` 通过。
- `go test -tags unit ./internal/config -run 'Test(LoadBillingFinalMultiplierFromEnvironment|ConfigKeysAreEnvReachable)'` 通过。
- `git diff --check` 通过（仅有工作区既有换行提示）。
- `docker compose config` 确认 18082 服务注入 `BILLING_FINAL_MULTIPLIER=10`。
- 18082 容器已重建，状态为 `healthy`；`GET http://127.0.0.1:18082/health` 返回 200 和 `{"status":"ok"}`。

## 测试阻断

计费包完整单测暂时无法编译，因为工作区既有的 `backend/internal/service/payment_order_provider_snapshot_test.go` 仍按旧签名调用 `createOrderInTx`，而当前支付实现已增加 `BalancePackagePlan` 参数。该错误与最终计费倍率改动无关，未修改用户已有支付改动。

## 运行配置

当前本地 18082 容器使用最终倍率 `10`。其他环境默认 `1.0`，可通过 `BILLING_FINAL_MULTIPLIER` 覆盖。
