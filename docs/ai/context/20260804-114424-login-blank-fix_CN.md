# 公网 `/login` 白屏修复记录

## 问题

公网 `https://aaccx.pw/login` 返回入口 HTML，但 Cloudflare 对 `vendor-*` 静态资源返回 403。入口 JS 因依赖加载失败，Vue 没有挂载到 `#app`，页面表现为空白。18082 本地直连同一资源返回 200，说明问题在公网静态资源路径规则，不在应用路由或登录组件。

## 修复

- 修改 `frontend/vite.config.ts` 和 `frontend/vite.config.js` 的 `manualChunks`：将 `vendor-vue`、`vendor-i18n`、`vendor-misc` 等公共依赖分包名统一改为 `lib-*`。
- 更新对应的懒加载契约测试，确认 Stripe 仍保持独立分包且不会进入共享依赖。
- 使用源码重建 `deploy-sub2api:latest`，仅执行 `sub2api-official-18082` 应用容器的强制替换；未操作现有 PostgreSQL、Redis 容器及持久化卷。

## 验证

- Vitest：`stripeLazyLoading.spec.ts` 4/4 通过。
- 生产构建：`pnpm run build` 成功，Vite 完成 971 个模块转换。
- 本地入口 `/login` 返回 200，引用 `lib-vue`、`lib-i18n`、`lib-misc`。
- 公网 `https://aaccx.pw/login` 的入口 JS、Vue、i18n、misc JS/CSS 和页面 CSS 全部返回 200。
- 浏览器实际打开公网登录页，DOM 包含邮箱、密码输入框和登录按钮，截图确认页面正常显示，不再白屏。
- 容器 `sub2api-official-18082` 状态为 `running (healthy)`。

## 访问地址

- 登录页：`https://aaccx.pw/login`
