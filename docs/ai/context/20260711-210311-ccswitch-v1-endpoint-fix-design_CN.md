# `/keys` CCSwitch OpenAI 端点 `/v1` 修复设计

## 背景

Sub2API 已于 2026-07-08 统一只保留 `/v1/*` 作为正式 OpenAI/Codex 模型 API。裸 `/responses`、`/models`、`/chat/completions` 等路径会返回 `400 INVALID_BASE_URL`，公开使用指南也已明确 Codex 的 `base_url` 必须停在 `https://api.aaccx.pw/v1`。

`/keys` 页面“导入到 CCSwitch”仍沿用旧契约。运行态公开设置 `api_base_url` 为空时，页面使用 `window.location.origin`，OpenAI 分支又把该裸域名原样写入 deeplink 的 `endpoint`。从 `https://api.aaccx.pw/keys` 导入后，CCSwitch 因此得到 `https://api.aaccx.pw`，Codex 会访问已被拒绝的裸路径。

## 根因

- `frontend/src/views/user/KeysView.vue` 以 `publicSettings.api_base_url || window.location.origin` 作为导入输入。
- `frontend/src/utils/ccswitchImport.ts` 的 OpenAI 分支设置 `endpoint: baseUrl`，没有执行 `/v1` 规范化。
- `frontend/src/utils/__tests__/ccswitchImport.spec.ts` 仍断言 OpenAI `endpoint === baseUrl`，把旧行为固化为测试契约。
- 正式 `/v1` 路由和使用指南上线时没有同步更新 CCSwitch 导入生成器。

## 目标

- OpenAI/Codex CCSwitch 导入端点必须以恰好一个 `/v1` 结尾。
- 输入为裸域名、尾斜杠、已有 `/v1` 或 `/v1/` 时，结果都保持幂等。
- 用量查询继续请求正式的 `/v1/usage`，不得形成 `/v1/v1/usage`。
- 保留现有 `app=codex`、`model=gpt-5.5`、provider 名称、API Key 和其他 deeplink 参数。

## 非目标

- 不修改 Anthropic、Gemini、Antigravity 的 endpoint 规则。
- 不修改自动 Key 在 `group=null` 时回退为 Anthropic 的现有平台判定。
- 不全局改写 `api_base_url` 的存储或公开设置语义。
- 不恢复裸 API 路径兼容，不改后端路由、Nginx、数据库、Redis 或运行态配置。

## 方案比较

### 方案 A：在 CCSwitch OpenAI 配置边界规范化，推荐

在 `resolveCcSwitchImportConfig()` 的 OpenAI 分支处理 URL。先去掉空白、尾斜杠和末尾已有的 `/v1`，再生成规范 endpoint；同时通过 CCSwitch 官方 `usageBaseUrl` 参数保留站点根地址，让现有 `{{baseUrl}}/v1/usage` 脚本继续得到正确 URL。

优点：修复位置与错误来源一致，平台边界清晰，其他消费者和平台不受影响。

### 方案 B：全局把 `api_base_url` 改为 `/v1`

该字段同时服务于多种客户端和页面。全局改变会把 OpenAI 的路径语义扩散到 Claude、Gemini、Antigravity，风险高且不解决配置为空时的 fallback。

### 方案 C：在 `KeysView.vue` 点击处理里临时拼接 `/v1`

能修复当前按钮，但 URL 规则会散落在页面层，容易重复追加，也无法让 `ccswitchImport` 工具本身保持正确契约。

## 最终设计

OpenAI 分支把输入拆成两个值：

| 输入 `baseUrl` | `endpoint` | `usageBaseUrl` |
| --- | --- | --- |
| `https://api.aaccx.pw` | `https://api.aaccx.pw/v1` | `https://api.aaccx.pw` |
| `https://api.aaccx.pw/` | `https://api.aaccx.pw/v1` | `https://api.aaccx.pw` |
| `https://api.aaccx.pw/v1` | `https://api.aaccx.pw/v1` | `https://api.aaccx.pw` |
| `https://api.aaccx.pw/v1/` | `https://api.aaccx.pw/v1` | `https://api.aaccx.pw` |

CCSwitch 会把 `endpoint` 写入 Codex `base_url`，把 `usageBaseUrl` 作为用量脚本专用地址。现有脚本 `{{baseUrl}}/v1/usage` 因此解析为 `https://api.aaccx.pw/v1/usage`，不会双写版本段。

非 OpenAI 分支不生成 `usageBaseUrl`，继续保持当前行为。

## 修改范围

- 修改 `frontend/src/utils/ccswitchImport.ts`：增加 OpenAI URL 规范化和可选 `usageBaseUrl` 参数。
- 修改 `frontend/src/utils/__tests__/ccswitchImport.spec.ts`：覆盖四类输入与非 OpenAI 不变性。
- 修改 `frontend/src/views/user/__tests__/KeysView.spec.ts`：覆盖 `/keys` 实际点击生成的 endpoint、usageBaseUrl 和 usageScript。
- 新建结果文档并在 `AGENTS.md` 顶部追加长期记忆。

## 测试策略

1. 先修改测试并运行，确认旧实现因 endpoint 缺少 `/v1`、`usageBaseUrl` 缺失而失败。
2. 实现最小修复后重跑目标测试。
3. 运行 CCSwitch 工具测试、KeysView 测试、前端类型检查和 `git diff --check`。
4. 复核 deeplink 解码后的 OpenAI endpoint 为 `https://api.aaccx.pw/v1`，用量地址最终为 `https://api.aaccx.pw/v1/usage`。

## 分支与工作区约束

- 分支：`codex/fix-ccswitch-v1-endpoint`，基于本地 `main` 的 `3c5c62503` 创建。
- 创建分支前工作区已有用户修改：`AGENTS.md` 一条定价审计记忆，以及未跟踪的对应审计文档。本次不得重置、覆盖或误提交这些改动。
- 本设计已由用户在 2026-07-11 明确确认。
