# 代码冗余治理阶段 1 结果

## 完成内容

- 创建分支 `codex/code-redundancy-refactor`。
- 新建整体重构计划文档，并在根 `AGENTS.md` 增加短索引。
- 将通用测试指针 helper 从 `unit` build tag 中释放，普通测试和 unit 测试共用同一实现。
- 将订阅权益周期仓储 stub 拆到无 build tag 的测试 helper 文件，避免普通测试引用不可见符号。

## 验证

- `go test ./...`：通过。
- `go test -tags=unit ./...`：通过。
- 用户未提交的 `backend/internal/repository/migrations_schema_integration_test.go` 保持未暂存，内容未纳入本阶段。
