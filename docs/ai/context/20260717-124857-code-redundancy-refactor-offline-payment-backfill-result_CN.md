# 代码冗余重构：离线付款补录工具隔离结果

日期：2026-07-17

## 已完成

- 将固定五笔离线付款补录实现从 `internal/service` 移至 `internal/tool/offlinepaymentbackfill`。
- 命令改为调用工具专属包，运行时服务不再承载一次性运维逻辑。
- 工具单测迁移到专属包；命令测试保留执行人、确认 token、dry-run/execute 互斥校验，并覆盖默认 dry-run。
- 带 `integration` 标签的仓储测试改为依赖工具专属包。
- 补录数据、dry-run、execute、确认 token 和所有前置校验规则保持不变。

## 验证

- `go test -p 1 -parallel 1 -count=1 ./internal/tool/offlinepaymentbackfill`
- `go test -p 1 -parallel 1 -count=1 ./cmd/offline-payment-backfill`
- `go test -tags=integration -p 1 -parallel 1 -c -o /tmp/sub2api-offline-payment-backfill-integration.test ./internal/repository`

集成测试只做编译验证，未实际执行测试，未连接或修改数据库。

## 边界

- 未运行补录命令，未执行 dry-run 或 execute。
- 未修改运行态数据库、Redis、容器、Nginx 或公网链路。
- `backend/internal/repository/migrations_schema_integration_test.go` 是用户已有的未提交改动，本阶段未修改、未暂存、不会提交。
