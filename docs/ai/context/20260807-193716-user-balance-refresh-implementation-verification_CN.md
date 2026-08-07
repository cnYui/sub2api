# 用户余额前端及时刷新实现与验证

## 背景

用户页面的余额来自认证 Store 缓存。原逻辑仅每 60 秒刷新一次；浏览器或 WebView 在后台会冻结定时器，恢复页面后顶部可能继续显示旧的 `auth_user.balance`。

## 实现

- 将用户资料的前台兜底同步间隔缩短为 30 秒；页面不可见时不请求接口。
- 在 `visibilitychange`、`focus` 与 `pageshow` 时触发后台刷新，覆盖切回标签页、窗口重新聚焦和 bfcache 恢复。
- 使用同一个 `refreshUserPromise` 合并并发刷新，避免多个恢复事件重复请求 `/auth/me`。
- 仅在 `/auth/me` 成功返回后更新 Store 与 `localStorage`；网络失败时保留最近一次有效余额，401 仍沿用原有登出流程。
- 自动同步会同时清理定时器和页面事件监听；模块级清理保证单个页面不会因多个 Store 实例而叠加监听。

## 测试

- Store 测试新增页面恢复立即刷新、页面隐藏跳过轮询、30 秒前台轮询、`focus`/`pageshow` 并发去重和刷新失败保留余额覆盖。
- `frontend/node_modules/.bin/vitest.cmd run src/stores/__tests__/auth.spec.ts --reporter=dot`：25/25 通过。
- `frontend/node_modules/.bin/vue-tsc.cmd --noEmit`：通过。
- `frontend/node_modules/.bin/eslint.cmd . --ext .vue,.js,.jsx,.cjs,.mjs,.ts,.tsx,.cts,.mts`：通过。

Store 测试中保留了既有“损坏的本地用户缓存应清除”用例产生的预期错误日志；该日志不代表测试失败。

## 范围

未部署生产环境，未修改支付、余额服务端逻辑或工作区中已有的后端改动。余额金额仍以服务端 `/auth/me` 的响应为准。
