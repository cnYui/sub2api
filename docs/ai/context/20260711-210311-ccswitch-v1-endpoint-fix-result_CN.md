# `/keys` CCSwitch OpenAI `/v1` 端点修复结果

## 结论

- 已在分支 `codex/fix-ccswitch-v1-endpoint` 修复 `/keys` 导入 CCSwitch 后 Codex Base URL 缺少 `/v1` 的问题。
- OpenAI/Codex 导入现在会把裸域名、尾斜杠、已有或连续末尾 `/v1` 统一为恰好一个 `/v1` endpoint；用量脚本仍请求正式的 `/v1/usage`。
- Anthropic、Gemini、Antigravity 和自动 Key `group=null` fallback 行为不变。
- 代码与审查已完成，尚未构建镜像或部署运行态。

## 背景与根因

Sub2API 正式收敛为只接受 `/v1/*` 模型 API 后，裸 `/responses`、`/models` 等路径会返回 `400 INVALID_BASE_URL`。但 `frontend/src/utils/ccswitchImport.ts` 的 OpenAI 分支仍把传入的 `baseUrl` 原样写入 CCSwitch `endpoint`。

公开 `api_base_url` 为空时，`/keys` 会使用 `window.location.origin` 作为 fallback，因此从公网页面导入得到的是裸域名。CCSwitch 把该值写入 Codex `base_url` 后，Codex 请求会落到已禁止的裸路径。旧单测还错误断言 `endpoint === baseUrl`，把过时行为固化成了测试契约。

## 改动

- `frontend/src/utils/ccswitchImport.ts`
  - 仅在 OpenAI/Codex 分支清理首尾空白、尾斜杠和一个或多个连续末尾 `/v1`，再生成恰好一个 `/v1` endpoint。
  - deeplink 新增可选 `usageBaseUrl`，值为不含版本段的根地址；现有 `{{baseUrl}}/v1/usage` 因而仍落到正确的 `/v1/usage`，不会形成 `/v1/v1/usage`。
  - 保留 `app=codex` 和 `model=gpt-5.5`。
- `frontend/src/utils/__tests__/ccswitchImport.spec.ts`
  - 修正旧 endpoint 契约，并覆盖空白、尾斜杠、已有 `/v1`、`/v1/`、连续 `/v1/v1` 和大小写输入。
  - 固化非 OpenAI 平台不生成 `usageBaseUrl`。
- `frontend/src/views/user/__tests__/KeysView.spec.ts`
  - 覆盖 `/keys` 实际点击链路，验证 `endpoint`、`usageBaseUrl` 和 `usageScript`。
  - 测试清理 fake timers、wrapper 和 spy，避免 interval 泄漏。

未修改 `KeysView.vue` 生产代码。

## TDD 证据

RED：

- 首轮工具层测试共 9 项，5 项失败，失败点均为 OpenAI endpoint 缺少 `/v1` 或 `usageBaseUrl` 缺失。
- 加入 `/keys` 点击链路后共 12 项，6 项失败，继续指向同一根因。
- 质量首审补充 `/v1/v1` 与 `/v1/v1/` 边界后，12 项中 2 项失败，实际结果均错误保留双 `/v1`。

GREEN：

```text
CCSwitch 工具与 KeysView 目标测试：15/15 PASS
加入 UseKeyModal 邻近回归：19/19 PASS
pnpm typecheck：exit 0
git diff --check：PASS
git show --check：PASS
```

## 审查

- 规范审查逐项核对 9 项要求，全部通过。
- 质量首审发现两个问题：连续末尾 `/v1` 会生成重复版本段，以及 KeysView 测试存在 interval 清理泄漏。
- 提交 `aebcb7997` 修复上述问题后，质量复审未发现 Critical、Important 或 Minor 问题，结论为 `Ready to merge: Yes`。

## Git 提交

- 分支：`codex/fix-ccswitch-v1-endpoint`
- 基线：本地 `main` 的 `3c5c62503`
- `d1bdaa978`：`docs: 记录 CCSwitch v1 端点修复设计`
- `7e0cf0235`：`fix: 修复 CCSwitch OpenAI v1 端点`
- `aebcb7997`：`fix: 完善 CCSwitch v1 规范化边界`

## 未触碰范围

- 未修改 Anthropic、Gemini、Antigravity endpoint 规则或自动 Key `group=null` fallback。
- 未修改 `KeysView.vue` 生产代码、后端代码、Nginx、Postgres、Redis 或 CLIProxyAPI。
- 未修改数据库、缓存或运行态配置，未构建 Docker 镜像，未部署公网 18084。

## 当前工作区并行改动说明

开始记录本结果前，工作区已存在 `AGENTS.md` 的未提交长期记忆修改，以及以下未跟踪文档：

- `docs/ai/context/20260711-120137-openai-gpt54-gpt55-gpt56-priority-pricing-audit_CN.md`
- `docs/ai/context/20260711-211142-sub2api-cliproxyapi-api-key-concurrency-design-audit_CN.md`
- `docs/ai/context/20260711-213023-29-subscription-api-key-401-diagnosis_CN.md`
- `docs/ai/context/20260711-213214-sub2api-upstream-account-concurrency-100-design_CN.md`

本次只在 `AGENTS.md` 顶部追加 CCSwitch 结果条目并新建本文，未覆盖、删除、重命名、暂存或提交上述并行改动。
