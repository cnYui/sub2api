# 首页登录入口、套餐与模型展示修正结果

## 结果

- 首页未登录态只保留 Hero 区一个登录入口。
- Hero 区 CTA 文案从「立即开始」改为「立即登录」，仍跳转 `/login`。
- 右上角未登录态「登录」按钮已移除。
- 已登录态右上角控制台入口保留。
- 中部三张卡片改为当前套餐：
  - 29 元套餐：额度 19 刀。
  - 39 元套餐：额度 29 刀。
  - 59 元套餐：额度 49 刀。
- 三个套餐均说明「每天 24 点自动刷新，可使用 30 天」。
- 29 元套餐说明中加入「使用 cc-switch 项目，一键接入 API 到 Codex」。
- 底部支持模型只展示 GPT 5.3、Codex 5.4、GPT 5.5。

## 修改文件

- `frontend/src/views/HomeView.vue`
- `frontend/src/i18n/locales/zh.ts`
- `frontend/src/i18n/locales/en.ts`
- `frontend/src/views/__tests__/HomeView.spec.ts`

## 验证

- `pnpm vitest run src/views/__tests__/HomeView.spec.ts`：3 个测试通过。
- `pnpm vitest run src/views/__tests__/HomeView.spec.ts src/__tests__/visualThemeSource.spec.ts`：2 个测试文件、4 个测试通过。
- `curl -I http://127.0.0.1:5174/home`：返回 200。
- `curl http://127.0.0.1:5174/src/views/HomeView.vue`：可见 GPT 5.3、Codex 5.4、GPT 5.5。
- `curl http://127.0.0.1:5174/src/i18n/locales/zh.ts`：可见立即登录、三档套餐、刷新规则和 cc-switch 文案。
- `pnpm build`：通过；仅保留既有 Vite 动态导入、大 chunk 和 Browserslist 警告。
- `git diff --check`：通过。
