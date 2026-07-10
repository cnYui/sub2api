# 完整测试 lint 清理记录

## 背景

- 用户要求检查本地未提交修改，提交到本地 `main` 后跑完整测试。
- 已提交 API Key 并发替换后执行根目录 `make test`。
- 第一轮失败原因不是 Go 测试失败，而是本机缺少项目要求的 `golangci-lint`。
- 按 `DEV_GUIDE.md` 安装 `github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.7` 后复跑，`go test ./...` 通过，`golangci-lint run ./...` 暴露既有静态检查问题。

## 修复范围

- `traffic_pack_repo.go`、`usage_billing_repo.go`：显式忽略 `Rows.Close` 与 `Tx.Rollback` 的 defer 错误，匹配仓库内既有写法。
- `user_subscription_repo.go`：补齐完整 lint 扫描继续暴露的同类 `Rows.Close` defer 错误忽略。
- `billing_service.go`：删除 `tierMultiplier` 的无效初始赋值。
- `api_key_auth.go`：删除未使用的 `abortIfAPIKeyGroupUnavailable` 与 `abortIfAPIKeyGroupNotAllowed`。
- `payment_amounts.go`、`payment_order.go`：删除未使用的旧金额 helper。
- `payment_refund.go`：删除未使用的旧余额退款 `validateRefundRequest`。
- `payment_refund_test.go`：删除只覆盖上述死代码的陈旧测试，保留仍覆盖 provider instance 防猜测的 `PrepareRefund` 测试。

## 状态

- 本记录只处理完整测试入口暴露的 lint 问题。
- 未改 DB、nginx、Redis、Docker 容器或公网 18084。
