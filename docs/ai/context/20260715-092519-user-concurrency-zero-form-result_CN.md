# 用户并发 0 值表单契约修复结果

时间：2026-07-15 09:33 JST

对应设计：`docs/ai/context/20260715-092335-user-concurrency-zero-form-design_CN.md`

对应计划：`docs/ai/context/20260715-092519-user-concurrency-zero-form-implementation-plan_CN.md`

## 结果

本地分支 `codex/fix-user-concurrency-zero-form` 已统一管理后台创建和编辑用户时的并发输入契约：

- `0` 表示不限并发，可以通过两个弹窗正常提交。
- 正整数表示具体并发上限。
- 负数、小数、`NaN` 和无限值被前端判为非法。
- 两个并发输入均声明 `min="0"`、`step="1"`。
- 中文提示为“0 表示不限并发”，英文提示为“0 = unlimited concurrency”。

本次只修复前端控制面，不改变后端 `concurrency <= 0` 直接放行的既有语义。

## 根因

`UserEditModal.vue` 原先使用 `form.concurrency < 1`，把后端合法的 0 错误拦截；`UserCreateModal.vue` 则没有并发校验和提示，两个入口使用不同规则。

## 实现

新增纯函数 `frontend/src/utils/userConcurrency.ts`，统一判断输入必须满足：

```ts
Number.isFinite(value) && Number.isInteger(value) && value >= 0
```

两个弹窗在调用既有 `adminAPI.users.create/update` 前复用该函数。创建弹窗保留通用 `useForm()` 的原有职责，只增加组件级并发校验入口；未扩大修改通用表单抽象。

中英文 locale 删除了已不准确的 `concurrencyMin`，改为 `concurrencyInvalid`，并增加 `form.concurrencyHint`。

## TDD 证据

共享校验测试首次运行因 `userConcurrency` 模块不存在而失败。

两个弹窗测试首次运行共 8 项：

- 1 项通过：创建弹窗原本已能把 0 发送给 API。
- 7 项失败：编辑弹窗拒绝 0；创建弹窗不阻止负数和小数；两个弹窗都缺少 `min/step` 和不限并发提示；编辑弹窗仍使用旧错误文案。

实现后目标测试：

```text
Test Files  3 passed (3)
Tests       17 passed (17)
```

覆盖共享校验边界、创建和编辑提交 0、非法值阻断、输入属性和提示。

## 回归验证

- `UsersView.spec.ts`：2/2 通过。
- `pnpm typecheck`：通过。
- `pnpm build`：通过，870 个模块完成转换。
- `git diff --check -- frontend`：通过。

构建仅出现项目既有的 Browserslist 数据过期、动态/静态 import 混用和大 chunk 警告，没有新增构建错误。

## 修改文件

- `frontend/src/utils/userConcurrency.ts`
- `frontend/src/utils/__tests__/userConcurrency.spec.ts`
- `frontend/src/components/admin/user/UserCreateModal.vue`
- `frontend/src/components/admin/user/UserEditModal.vue`
- `frontend/src/components/admin/user/__tests__/UserCreateModal.spec.ts`
- `frontend/src/components/admin/user/__tests__/UserEditModal.spec.ts`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`

## 边界

- 未修改后端、数据库 schema、Redis 或运行态用户数据。
- 未修改上游账号并发或 CLIProxyAPI 配置。
- 未部署公网 18084。
- 运行态 `users.id=13.concurrency=0` 保持不变。
