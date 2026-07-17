# 代码冗余重构：用量统计与聚合查询结果

日期：2026-07-17

## 已完成

- 删除 `service.UsageStats` 的重复结构体，服务与用户用量 Handler 直接使用 `usagestats.UsageStats`。
- 用户、API Key、账号、模型四套聚合 SQL 收敛为一个内部白名单维度查询；维度只能映射到固定列，外部输入不会拼接为 SQL 列名。
- inbound endpoint、upstream endpoint 和 endpoint path 三种统计共用同一个查询、筛选和扫描实现；表达式也由内部白名单固定。
- 保持原 endpoint path SQL 的输出形状，避免影响既有行为与查询测试。

## 验证

- `go test -p 1 -parallel 1 -count=1 ./internal/repository ./internal/service ./internal/handler`
- `git diff --check`

## 边界

- 未修改数据库迁移和运行态环境。
- `backend/internal/repository/migrations_schema_integration_test.go` 是用户已有的未提交改动，本阶段未修改、未暂存、不会提交。
