# 用户并发 0 值表单契约修复设计

时间：2026-07-15 09:23 JST

## 背景

Sub2API 的真实并发契约是：

- `concurrency > 0`：限制单把 API Key 的并发数。
- `concurrency <= 0`：`ConcurrencyService.AcquireAPIKeySlot()` 直接放行，不创建 API Key 并发槽。

运行态已使用 `users.concurrency=0` 表示不限并发，但管理后台存在两处契约不一致：

- `UserEditModal.vue` 使用 `form.concurrency < 1` 拒绝 0，导致管理员无法通过正式页面保存不限并发。
- `UserCreateModal.vue` 没有并发校验和语义提示，0 可以提交，负数和小数也可能进入 API 请求。

因此问题不在后端或数据库，而在前端用户管理表单没有统一表达“0 表示不限并发”的既有契约。

## 目标

- 创建用户和编辑用户统一接受非负整数并发值。
- `0` 明确表示不限并发并可正常提交。
- 正整数继续表示具体并发上限。
- 负数、小数和非有限数值不发送 API 请求，并显示明确错误。
- 中文和英文界面都显示 `0 = 不限并发` 的提示。

## 非目标

- 不修改后端 `<=0` 的并发语义。
- 不修改数据库 schema、运行态用户数据或 Redis。
- 不修改上游账号并发和 CLIProxyAPI 限制。
- 不修改账号管理页面中的上游账号并发控件；其并发语义和本次用户/API Key 入口并发不同。
- 不部署公网运行态。

## 方案比较

### 方案一：只修复编辑弹窗

把编辑弹窗校验改为 `<0`，即可解决当前管理员无法保存 0 的直接问题。但创建弹窗仍可提交负数或小数，两个入口继续使用不同契约。

不采用。

### 方案二：共享用户并发校验并统一两个弹窗

新增轻量纯函数，统一判断并发值必须是非负整数；创建和编辑弹窗都使用该函数，并统一输入属性和提示文案。

采用该方案。两个入口共享同一条业务规则，测试可以直接覆盖边界值，且不需要改动通用 `useForm()`。

### 方案三：前后端同时拒绝负数

后端目前明确把所有非正数视为不限流。改为只接受 0 会扩大到 API、服务、批量并发和历史数据兼容，超出本次页面契约修复范围。

不采用。

## 组件设计

### 共享校验

新增 `frontend/src/utils/userConcurrency.ts`：

```ts
export const isValidUserConcurrency = (value: number): boolean =>
  Number.isFinite(value) && Number.isInteger(value) && value >= 0
```

该函数只负责表达用户并发输入契约，不负责显示错误或修改表单状态。

### 编辑用户弹窗

`UserEditModal.vue`：

- 并发输入增加 `min="0"` 和 `step="1"`。
- 使用共享校验替换现有 `<1` 判断。
- 0 正常进入 `adminAPI.users.update()`。
- 校验失败时沿用 `appStore.showError()`，不发送请求。
- 输入下方显示“0 表示不限并发”。

### 创建用户弹窗

`UserCreateModal.vue`：

- 并发输入增加 `min="0"` 和 `step="1"`。
- 在调用现有 `useForm().submit()` 前增加组件级校验入口。
- 校验失败时通过 `appStore.showError()` 返回，不进入 `useForm()`，避免发送请求和触发成功状态。
- 0 正常进入 `adminAPI.users.create()`。
- 输入下方显示与编辑弹窗相同的提示。

不扩展通用 `useForm()` 的 API，因为当前校验只属于用户并发字段，修改通用表单抽象会扩大影响面。

### 国际化

在 `admin.users` 下统一调整或新增：

- 校验错误：并发数必须是非负整数。
- 输入提示：0 表示不限并发。

英文文案表达相同含义。

## 测试设计

新增纯函数测试：

- 接受 0 和正整数。
- 拒绝负数、小数、`NaN` 和无限值。

新增两个弹窗的组件测试：

- 编辑已有并发 0 的用户时，表单正确显示 0。
- 编辑提交 0 时调用 `adminAPI.users.update(id, payload)`，payload 中 `concurrency=0`。
- 创建提交 0 时调用 `adminAPI.users.create(payload)`，payload 中 `concurrency=0`。
- 负数和小数均显示错误且不调用 API。
- 两个弹窗的输入都包含 `min=0`、`step=1`，并显示不限并发提示。

先写测试并确认因当前 `<1` 校验、缺少共享校验或缺少提示而失败，再实施最小修复。

## 验证范围

- 目标 Vitest 测试。
- 相邻 `UsersView` 测试。
- 前端 typecheck。
- 前端 build。
- `git diff --check`。

本次不启动或部署公网服务，不修改运行态数据库。
