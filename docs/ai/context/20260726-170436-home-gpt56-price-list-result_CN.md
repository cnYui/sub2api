# 首页 GPT-5.6 与价格清单实施结果

## 已完成

- 默认首页移除 GPT 5.3、Codex 5.4、GPT 5.5 展示，改为 `gpt-5.6-luna`、`gpt-5.6-sol`、`gpt-5.6-terra`。
- 首页新增静态价格区，完整列出 10 档 28 天订阅套餐：29、39、49、59、79、99、149、199、249、299 元。
- 首页新增静态 GPT 流量卡：5 刀额度售 2 元、10 刀额度售 3 元、20 刀额度售 5 元，均为 365 天有效。
- 未登录用户点击价格卡前往 `/login`，已登录用户前往 `/purchase`。
- 新增首页回归测试，覆盖模型替换、13 张价格卡完整性与登录态跳转。

## 验证

- `pnpm vitest run src/views/__tests__/HomeView.spec.ts`：6/6 通过。
- `pnpm typecheck`：通过。
- `pnpm lint:check`：通过。
- `pnpm build`：通过；保留项目既有的 Browserslist、动态导入与大包体警告。

## 维护约束

本次按用户要求使用前端硬编码。后台修改在售商品、价格、额度或有效期时，必须同步更新 `frontend/src/views/HomeView.vue` 的 `homeProducts` 与 `HomeView.spec.ts`。
