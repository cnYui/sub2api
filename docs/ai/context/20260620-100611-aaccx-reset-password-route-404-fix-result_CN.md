# aaccx.pw 重置密码链接 404 修复结果

## 修复内容

- 已修改 `/opt/homebrew/etc/nginx/servers/aaccx-root.conf`。
- 将 `aaccx.pw` 的 Sub2API SPA 路由白名单从：
  - `dashboard|login|register|keys|subscriptions|purchase|orders|settings|admin`
- 扩展为明确覆盖 Sub2API 前端认证、用户、支付、管理入口：
  - `dashboard`
  - `login`
  - `register`
  - `email-verify`
  - `auth`
  - `forgot-password`
  - `reset-password`
  - `keys`
  - `key-usage`
  - `usage`
  - `redeem`
  - `affiliate`
  - `available-channels`
  - `profile`
  - `subscriptions`
  - `purchase`
  - `orders`
  - `payment`
  - `settings`
  - `monitor`
  - `custom`
  - `admin`
- 没有把所有未知路径代理给 Sub2API，避免破坏 yui.web 的页面兜底。

## 验证

- 修复前：
  - 本地 `127.0.0.1:18080/reset-password?...` 返回 200。
  - 公网 `https://aaccx.pw/reset-password?...` 返回 404。
  - 公网 `https://aaccx.pw/forgot-password` 返回 404。
  - 公网 `https://aaccx.pw/email-verify` 返回 404。
- 修复后：
  - `nginx -t -c /opt/homebrew/etc/nginx/nginx.conf` 通过。
  - `nginx -s reload -c /opt/homebrew/etc/nginx/nginx.conf` 已执行。
  - 公网 `https://aaccx.pw/reset-password?...` 返回 200，HTML 为 Sub2API。
  - 公网 `https://aaccx.pw/forgot-password` 返回 200。
  - 公网 `https://aaccx.pw/email-verify` 返回 200。
  - 公网 `https://aaccx.pw/shop` 仍返回 200，继续由 yui.web 处理。

## 后续处理

- 重启了 `sub2api` 容器，刷新 HTML 注入的公开设置缓存。
- 刷新后 `/reset-password` HTML 注入已显示：
  - `email_verify_enabled=true`
  - `password_reset_enabled=true`
- 截图中暴露的旧 `xiaobianfuai@gmail.com` 密码重置 token 已从 Redis 删除。
- 已重新触发 `xiaobianfuai@gmail.com` 的忘记密码邮件；日志确认 EmailQueue worker 已发送。
