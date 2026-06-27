# AGENTS 记忆压缩计划

## 目标

将 `AGENTS.md` 中累积的长运行记录沉淀到 `docs/ai/context/` 的新上下文文件，并把 `AGENTS.md` 精简为后续 AI 协作的入口索引和高优先级约束。

## 必须保留的信息

- 三项目串联方案 A：Sub2API 是唯一公网 API 入口、用户 Key、计费和用量事实源；CLIProxyAPI 只做内网账号池、OAuth、协议转换和上游轮询；yui.web/shop 只保留展示、说明和跳转。
- 当前公网链路：`Cloudflare Tunnel -> nginx 127.0.0.1:8080 -> Sub2API 127.0.0.1:18080 -> CLIProxyAPI 127.0.0.1:8317`。
- 敏感信息规则：不要在文档、提交或日志中记录完整 API Key、内部 token、HMAC secret、SMTP 密码。
- 关键运行态、易踩坑和 2026-06-19 到 2026-06-24 的操作历史需要可追溯，但不应继续塞满 `AGENTS.md`。

## 文件变更

- 新增：`docs/ai/context/20260624-195608-agents-memory-compressed_CN.md`
  - 承接当前 `AGENTS.md` 的压缩全文。
  - 按核心架构、运行态、关键配置、用户迁移、功能变更、发布运维和排障记录重组。
- 修改：`AGENTS.md`
  - 保留协作入口、最新压缩记忆链接、最高优先级约束和最新运行态提醒。
  - 删除长运行记录列表，改为引用新上下文文档。

## 验证

- 确认新增上下文文件存在。
- 确认 `AGENTS.md` 引用新压缩文档。
- 确认 `AGENTS.md` 行数明显减少。
- 确认未改动无关工作区变更。
