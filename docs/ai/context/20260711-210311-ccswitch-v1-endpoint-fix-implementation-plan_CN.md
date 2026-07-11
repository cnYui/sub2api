# `/keys` CCSwitch OpenAI `/v1` 端点修复实施计划

> 执行时必须按 TDD 完成 RED、GREEN、回归验证；步骤使用复选框跟踪。

**目标：** 让 `/keys` 导入到 CCSwitch 的 OpenAI/Codex provider 使用恰好一个 `/v1` 作为 Base URL，同时保持 `/v1/usage` 用量查询可用。

**架构：** URL 规范化只发生在 `ccswitchImport` 的 OpenAI 平台边界。deeplink 同时携带 Codex `endpoint` 和用量脚本专用的 `usageBaseUrl`，其他平台不改变。

**技术栈：** Vue 3、TypeScript、Vitest、Vue Test Utils、CCSwitch deeplink v1。

---

### 任务 1：固化设计与分支基线

**文件：**
- 新建：`docs/ai/context/20260711-210311-ccswitch-v1-endpoint-fix-design_CN.md`
- 新建：`docs/ai/context/20260711-210311-ccswitch-v1-endpoint-fix-implementation-plan_CN.md`

- [x] 从本地 `main` 创建 `codex/fix-ccswitch-v1-endpoint`。
- [x] 记录创建分支前已有的 `AGENTS.md` 和审计文档改动，后续不重置、不误提交。
- [ ] 只暂存并提交本任务两份设计文档：

```bash
git add docs/ai/context/20260711-210311-ccswitch-v1-endpoint-fix-design_CN.md \
  docs/ai/context/20260711-210311-ccswitch-v1-endpoint-fix-implementation-plan_CN.md
git commit -m "docs: 记录 CCSwitch v1 端点修复设计"
```

### 任务 2：RED - 工具层 URL 契约

**文件：**
- 修改：`frontend/src/utils/__tests__/ccswitchImport.spec.ts`

- [ ] 把现有 OpenAI endpoint 断言从裸 `baseUrl` 改为 `${baseUrl}/v1`，并断言 `usageBaseUrl` 为不含 `/v1` 的根地址。
- [ ] 新增表格测试，覆盖 `https://api.example.com`、尾斜杠、已有 `/v1`、已有 `/v1/` 四种输入，全部期望：

```ts
expect(params.get('endpoint')).toBe('https://api.example.com/v1')
expect(params.get('usageBaseUrl')).toBe('https://api.example.com')
```

- [ ] 在 Anthropic、Gemini、Antigravity 用例中断言不存在 `usageBaseUrl`，防止平台规则扩散。
- [ ] 运行目标测试并确认因旧实现缺少 `/v1` 和 `usageBaseUrl` 而失败：

```bash
cd frontend
pnpm vitest run src/utils/__tests__/ccswitchImport.spec.ts
```

### 任务 3：RED - `/keys` 实际点击链路

**文件：**
- 修改：`frontend/src/views/user/__tests__/KeysView.spec.ts`

- [ ] 构造 `group.platform='openai'` 的 API Key，公开设置返回 `api_base_url='https://api.aaccx.pw'`。
- [ ] 模拟点击“导入到 CCSwitch”，解析 `window.open()` 收到的 deeplink。
- [ ] 断言 `app=codex`、`endpoint=https://api.aaccx.pw/v1`、`usageBaseUrl=https://api.aaccx.pw`。
- [ ] 解码 `usageScript`，断言仍包含 `{{baseUrl}}/v1/usage`，与 `usageBaseUrl` 组合后只有一个 `/v1`。
- [ ] 运行两个目标测试并确认新点击链路用例失败：

```bash
cd frontend
pnpm vitest run src/utils/__tests__/ccswitchImport.spec.ts src/views/user/__tests__/KeysView.spec.ts
```

### 任务 4：GREEN - 最小实现

**文件：**
- 修改：`frontend/src/utils/ccswitchImport.ts`

- [ ] 为 `CcSwitchImportConfig` 增加可选字段：

```ts
usageBaseUrl?: string
```

- [ ] 增加仅供 OpenAI 分支使用的纯函数，去掉空白、尾斜杠和末尾已有 `/v1`：

```ts
function resolveOpenAICcSwitchUrls(baseUrl: string) {
  const normalized = baseUrl.trim().replace(/\/+$/, '')
  const usageBaseUrl = normalized.replace(/\/v1$/i, '')
  return {
    endpoint: `${usageBaseUrl}/v1`,
    usageBaseUrl
  }
}
```

- [ ] OpenAI 分支复用该结果，保留 `app=codex` 和 `model=gpt-5.5`。
- [ ] 用数组展开组合可选 `model`、`usageBaseUrl` 参数，避免继续用 `splice()` 修改 entries。
- [ ] 重跑两个目标测试，确认全部通过。

### 任务 5：回归验证

**文件：**
- 不新增生产文件。

- [ ] 运行 CCSwitch、KeysView 和 UseKeyModal 邻近测试：

```bash
cd frontend
pnpm vitest run \
  src/utils/__tests__/ccswitchImport.spec.ts \
  src/views/user/__tests__/KeysView.spec.ts \
  src/components/keys/__tests__/UseKeyModal.spec.ts
```

- [ ] 运行前端类型检查：

```bash
cd frontend
pnpm typecheck
```

- [ ] 回到仓库根目录检查补丁格式：

```bash
git diff --check
```

### 任务 6：长期上下文与结果记录

**文件：**
- 修改：`AGENTS.md`
- 新建：`docs/ai/context/20260711-210311-ccswitch-v1-endpoint-fix-result_CN.md`

- [ ] 在 `AGENTS.md` 的“最高优先级定论”顶部追加本次根因、修复范围、测试证据和未部署说明；保留已有未提交定价审计条目。
- [ ] 新建结果文档，记录 RED 失败、GREEN 通过、最终 deeplink 参数、验证命令和未触碰范围。
- [ ] 复核 `git status --short`，确认没有把用户原有审计文档误计入本次提交。
- [ ] 发起独立代码审查，修复 Critical/Important 问题后重新运行目标测试、类型检查和 `git diff --check`。
