# 用户并发 0 值表单契约修复实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让管理后台创建和编辑用户时统一支持 `concurrency=0` 表示不限并发，并在前端拒绝负数、小数和非有限值。

**Architecture:** 用一个无状态纯函数集中表达用户并发输入契约，两个弹窗在进入现有 API 提交流程前复用该函数。输入控件和中英文提示同步表达 `0=不限并发`，不修改通用表单抽象或后端并发语义。

**Tech Stack:** Vue 3、TypeScript、Pinia、Vue I18n、Vitest、Vue Test Utils、pnpm。

---

对应设计：`docs/ai/context/20260715-092335-user-concurrency-zero-form-design_CN.md`

## 文件范围

- 新建：`frontend/src/utils/userConcurrency.ts`，提供用户并发纯校验。
- 新建：`frontend/src/utils/__tests__/userConcurrency.spec.ts`，覆盖数值边界。
- 新建：`frontend/src/components/admin/user/__tests__/UserEditModal.spec.ts`，覆盖编辑弹窗。
- 新建：`frontend/src/components/admin/user/__tests__/UserCreateModal.spec.ts`，覆盖创建弹窗。
- 修改：`frontend/src/components/admin/user/UserEditModal.vue`，接受 0、拒绝非法值、显示提示。
- 修改：`frontend/src/components/admin/user/UserCreateModal.vue`，提交前校验并显示提示。
- 修改：`frontend/src/i18n/locales/zh.ts`，增加中文并发校验与提示。
- 修改：`frontend/src/i18n/locales/en.ts`，增加英文并发校验与提示。
- 新建：`docs/ai/context/20260715-092519-user-concurrency-zero-form-result_CN.md`，记录结果。
- 修改：`AGENTS.md`，追加长期上下文索引。

### Task 1：共享并发校验

**Files:**

- Create: `frontend/src/utils/__tests__/userConcurrency.spec.ts`
- Create: `frontend/src/utils/userConcurrency.ts`

- [ ] **Step 1：写纯函数失败测试**

```ts
import { describe, expect, it } from 'vitest'
import { isValidUserConcurrency } from '../userConcurrency'

describe('isValidUserConcurrency', () => {
  it.each([0, 1, 5, 100])('接受非负整数 %s', (value) => {
    expect(isValidUserConcurrency(value)).toBe(true)
  })

  it.each([-1, 0.5, Number.NaN, Number.POSITIVE_INFINITY, Number.NEGATIVE_INFINITY])(
    '拒绝非法并发值 %s',
    (value) => {
      expect(isValidUserConcurrency(value)).toBe(false)
    }
  )
})
```

- [ ] **Step 2：运行测试确认 RED**

```bash
cd frontend
pnpm vitest run src/utils/__tests__/userConcurrency.spec.ts
```

预期：因 `../userConcurrency` 不存在而失败。

- [ ] **Step 3：实现最小纯函数**

```ts
export const isValidUserConcurrency = (value: number): boolean =>
  Number.isFinite(value) && Number.isInteger(value) && value >= 0
```

- [ ] **Step 4：运行测试确认 GREEN**

```bash
cd frontend
pnpm vitest run src/utils/__tests__/userConcurrency.spec.ts
```

预期：全部通过。

### Task 2：创建和编辑弹窗回归测试

**Files:**

- Create: `frontend/src/components/admin/user/__tests__/UserEditModal.spec.ts`
- Create: `frontend/src/components/admin/user/__tests__/UserCreateModal.spec.ts`

- [ ] **Step 1：建立组件测试公共 mock 结构**

两个测试文件分别 mock：

```ts
const mocks = vi.hoisted(() => ({
  create: vi.fn(),
  update: vi.fn(),
  updateUserAttributeValues: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn()
}))
```

`@/api/admin` 只暴露对应测试需要的 `adminAPI.users.create/update` 和 `adminAPI.userAttributes.updateUserAttributeValues`；`useAppStore()` 返回 `showError/showSuccess`；`useI18n().t()` 原样返回 key；`BaseDialog` 使用可渲染默认插槽和 footer 插槽的轻量 stub；`Icon` 与 `UserAttributeForm` 使用空 stub。

- [ ] **Step 2：写编辑弹窗失败测试**

覆盖以下断言：

```ts
expect(concurrencyInput.attributes('min')).toBe('0')
expect(concurrencyInput.attributes('step')).toBe('1')
expect(wrapper.text()).toContain('admin.users.form.concurrencyHint')

await concurrencyInput.setValue(0)
await wrapper.get('form').trigger('submit')
await flushPromises()
expect(mocks.update).toHaveBeenCalledWith(13, expect.objectContaining({ concurrency: 0 }))
```

并分别把输入设置为 `-1` 和 `0.5`，断言 `showError('admin.users.concurrencyInvalid')` 被调用，`update` 未调用。

- [ ] **Step 3：写创建弹窗失败测试**

覆盖以下断言：

```ts
expect(concurrencyInput.attributes('min')).toBe('0')
expect(concurrencyInput.attributes('step')).toBe('1')
expect(wrapper.text()).toContain('admin.users.form.concurrencyHint')

await concurrencyInput.setValue(0)
await wrapper.get('form').trigger('submit')
await flushPromises()
expect(mocks.create).toHaveBeenCalledWith(expect.objectContaining({ concurrency: 0 }))
```

并分别把输入设置为 `-1` 和 `0.5`，断言 `showError('admin.users.concurrencyInvalid')` 被调用，`create` 未调用。

- [ ] **Step 4：运行组件测试确认 RED**

```bash
cd frontend
pnpm vitest run \
  src/components/admin/user/__tests__/UserEditModal.spec.ts \
  src/components/admin/user/__tests__/UserCreateModal.spec.ts
```

预期失败原因：编辑弹窗仍拒绝 0；创建弹窗没有显式校验；两个输入缺少 `min/step` 和提示。

### Task 3：统一两个弹窗契约

**Files:**

- Modify: `frontend/src/components/admin/user/UserEditModal.vue`
- Modify: `frontend/src/components/admin/user/UserCreateModal.vue`
- Modify: `frontend/src/i18n/locales/zh.ts`
- Modify: `frontend/src/i18n/locales/en.ts`

- [ ] **Step 1：修改编辑弹窗**

并发输入调整为：

```vue
<input
  v-model.number="form.concurrency"
  type="number"
  min="0"
  step="1"
  class="input"
/>
<p class="input-hint">{{ t('admin.users.form.concurrencyHint') }}</p>
```

导入共享校验并替换旧判断：

```ts
import { isValidUserConcurrency } from '@/utils/userConcurrency'

if (!isValidUserConcurrency(form.concurrency)) {
  appStore.showError(t('admin.users.concurrencyInvalid'))
  return
}
```

- [ ] **Step 2：修改创建弹窗**

输入增加相同的 `min="0"`、`step="1"` 和提示。

导入 `useAppStore` 与共享校验，保留 `useForm()` 的提交和错误处理，并增加外层入口：

```ts
const appStore = useAppStore()

const handleSubmit = async () => {
  if (!isValidUserConcurrency(form.concurrency)) {
    appStore.showError(t('admin.users.concurrencyInvalid'))
    return
  }
  await submit()
}
```

模板表单改为：

```vue
<form id="create-user-form" @submit.prevent="handleSubmit" class="space-y-5">
```

- [ ] **Step 3：更新中英文文案**

中文：

```ts
concurrencyHint: '0 表示不限并发',
concurrencyInvalid: '并发数必须是非负整数',
```

英文：

```ts
concurrencyHint: '0 = unlimited concurrency',
concurrencyInvalid: 'Concurrency must be a non-negative integer',
```

删除已无引用的 `concurrencyMin` 文案。

- [ ] **Step 4：运行目标测试确认 GREEN**

```bash
cd frontend
pnpm vitest run \
  src/utils/__tests__/userConcurrency.spec.ts \
  src/components/admin/user/__tests__/UserEditModal.spec.ts \
  src/components/admin/user/__tests__/UserCreateModal.spec.ts
```

预期：全部通过。

### Task 4：回归验证

**Files:**

- Test: `frontend/src/views/admin/__tests__/UsersView.spec.ts`

- [ ] **Step 1：运行相邻页面测试**

```bash
cd frontend
pnpm vitest run src/views/admin/__tests__/UsersView.spec.ts
```

预期：全部通过。

- [ ] **Step 2：运行前端类型检查**

```bash
cd frontend
pnpm typecheck
```

预期：退出码 0。

- [ ] **Step 3：运行生产构建**

```bash
cd frontend
pnpm build
```

预期：退出码 0。

- [ ] **Step 4：检查差异格式**

```bash
git diff --check
```

预期：无输出，退出码 0。

### Task 5：结果记录

**Files:**

- Create: `docs/ai/context/20260715-092519-user-concurrency-zero-form-result_CN.md`
- Modify: `AGENTS.md`

- [ ] **Step 1：写结果文档**

记录根因、修改文件、TDD RED/GREEN 证据、测试/typecheck/build 结果、未部署和未修改运行态的边界。

- [ ] **Step 2：更新长期上下文**

在 `AGENTS.md` 的“最高优先级定论”顶部追加本次代码修复索引，不覆盖已有条目。

- [ ] **Step 3：最终校验**

```bash
git diff --check -- AGENTS.md frontend docs/ai/context
git status --short
```

只提交本任务代码、测试和新文档；不得带入当前工作区已有的其他未跟踪文档或无关 `AGENTS.md` 改动。

## 完成标准

- 创建和编辑用户均可提交 `concurrency=0`。
- 两个弹窗均拒绝负数、小数和非有限值，不发送 API 请求。
- 两个输入均声明 `min=0`、`step=1` 并显示不限并发提示。
- 共享校验、两个弹窗和 `UsersView` 测试通过。
- typecheck、build 和 `git diff --check` 通过。
- 未修改后端、数据库、Redis 或公网运行态。
