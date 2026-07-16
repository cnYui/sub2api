# 用户并发原生约束校验补充结果

时间：2026-07-15 09:47 JST

对应原结果：`docs/ai/context/20260715-092519-user-concurrency-zero-form-result_CN.md`

## 问题

第一版组件测试直接触发 `submit` 事件，绕过了浏览器原生约束校验。真实页面输入 `-1` 或 `0.5` 时，`min="0"` 或 `step="1"` 会先触发 `invalid` 并阻止 `submit`，导致提交处理器内的 `concurrencyInvalid` 自定义提示无法显示。

## 修复

- 创建和编辑用户弹窗的并发输入都增加 `@invalid.prevent`。
- 原生 `invalid` 事件与提交前共享校验复用同一个错误提示函数。
- 非法值测试改用 `HTMLFormElement.requestSubmit()`，覆盖真实浏览器约束顺序。
- 保留创建弹窗的 email/password 原生 `required` 校验，未给表单增加 `novalidate`。

## TDD 证据

修改生产代码前，两个弹窗的 `-1` 和 `0.5` 用例共 4 项稳定失败，均表现为 `showError` 调用次数为 0；生产代码增加 `invalid` 处理后，两个弹窗测试 8/8 通过。

## 验证

- 共享校验、创建弹窗、编辑弹窗和 `UsersView`：19/19 通过。
- `pnpm typecheck`：通过。
- `pnpm build`：通过。
- 未修改后端、数据库、Redis、运行态或公网服务，未部署。
