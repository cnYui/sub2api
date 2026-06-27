# aaccx.pw 重置密码链接 404 修复计划

## 现象

- Gmail 密码重置邮件已成功发出。
- 邮件中的链接为 `https://aaccx.pw/reset-password?...`。
- 浏览器打开后显示 yui.web 的 404 页面。

## 复现

- `http://127.0.0.1:18080/reset-password?email=test%40example.com&token=test` 返回 200。
- `https://aaccx.pw/reset-password?email=test%40example.com&token=test` 返回 404。
- `https://aaccx.pw/forgot-password` 返回 404。
- `https://aaccx.pw/email-verify` 返回 404。

## 根因

Sub2API 前端本身已有 `/reset-password` 路由，问题不在前端页面。

公网 `aaccx.pw` nginx 只把以下路径转发给 Sub2API：

`dashboard|login|register|keys|subscriptions|purchase|orders|settings|admin`

`reset-password`、`forgot-password`、`email-verify` 等认证页面不在白名单内，因此落入 yui.web 静态站点兜底，最终显示 yui.web 404。

## 修复策略

- 在 `/opt/homebrew/etc/nginx/servers/aaccx-root.conf` 的 Sub2API SPA 路由白名单中补充明确路由。
- 不把所有未知路径代理给 Sub2API，避免破坏 yui.web 的页面解析。
- 补充的路由覆盖当前 Sub2API router 中应该由 Sub2API 接管的公共、用户、支付、管理入口：
  - `email-verify`
  - `auth`
  - `forgot-password`
  - `reset-password`
  - `key-usage`
  - `usage`
  - `redeem`
  - `affiliate`
  - `available-channels`
  - `profile`
  - `monitor`
  - `payment`
  - `custom`

## 验证计划

1. `nginx -t -c /opt/homebrew/etc/nginx/nginx.conf` 语法检查。
2. reload nginx。
3. 验证 `https://aaccx.pw/reset-password?...` 返回 200。
4. 验证 `https://aaccx.pw/forgot-password` 和 `https://aaccx.pw/email-verify` 返回 200。
5. 验证 `https://aaccx.pw/shop` 仍由 yui.web 处理。
