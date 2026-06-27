# 用户自助重置密码能力核查

## 背景

用户指出历史迁移中，原 yui.web 手机号登录用户被迁移为 Sub2API 假邮箱账号，且密码统一重设为 `123123`。需要确认当前项目是否支持用户自己重新设置密码。

## 代码事实

- 已登录用户可以在 `/profile` 页面修改密码。
  - 前端路由：`frontend/src/router/index.ts` 的 `/profile` 指向 `ProfileView.vue`。
  - 页面组件：`frontend/src/views/user/ProfileView.vue` 固定展示 `ProfilePasswordForm`。
  - 表单接口：`frontend/src/components/user/profile/ProfilePasswordForm.vue` 调用 `userAPI.changePassword(old_password, new_password)`。
  - API：`frontend/src/api/user.ts` 调用 `PUT /api/v1/user/password`。
  - 后端路由：`backend/internal/server/routes/user.go` 注册 `user.PUT("/password", h.User.ChangePassword)`，需要 JWT 登录。
  - 后端服务：`backend/internal/service/user_service.go` 会校验当前密码，写入新密码并递增 `TokenVersion` 使旧 JWT 失效。

- 未登录用户理论上可以通过“忘记密码”邮件链路重置密码。
  - 前端路由：`/forgot-password`、`/reset-password` 已存在。
  - API：`frontend/src/api/auth.ts` 调用 `POST /auth/forgot-password` 和 `POST /auth/reset-password`。
  - 后端路由：`backend/internal/server/routes/auth.go` 注册公开接口 `/auth/forgot-password`、`/auth/reset-password`。
  - 后端服务：`AuthService.RequestPasswordResetAsync()` 会发邮件，`AuthService.ResetPassword()` 会消费 Redis reset token、更新密码并撤销 refresh token。

## 当前运行态

- `http://127.0.0.1:18080/api/v1/settings/public` 返回：
  - `email_verify_enabled=false`
  - `password_reset_enabled=false`

- `https://aaccx.pw/api/v1/settings/public` 返回同样结果：
  - `email_verify_enabled=false`
  - `password_reset_enabled=false`

- 直接调用本地和公网 `POST /api/v1/auth/forgot-password` 当前返回：
  - `500 {"code":500,"message":"Password reset is not configured"}`

## 判断

当前项目“代码支持”用户自助改密码，但对这批迁移用户的实际可用性分两种：

1. 用户知道当前密码 `123123`：可以登录假邮箱账号后进入 `/profile` 自己修改密码。
2. 用户不知道或忘了 `123123`：当前不能通过“忘记密码”自助找回，因为公网运行态关闭了邮箱验证和密码重置，而且迁移账号是假邮箱，邮件链路即使开启也无法可靠送达用户本人。

## 风险

- 统一默认密码 `123123` 是高风险状态；只依赖用户登录后手动改密码，覆盖率不可控。
- 假邮箱账号无法作为真实找回凭据；启用邮件重置不能解决手机号用户身份确认问题。
- 登录页只有在 `password_reset_enabled=true` 且非 backend mode 时才显示“忘记密码”入口，当前用户看不到该入口。

## 建议方向

- 短期：强制这批迁移用户首次登录后修改密码，或在下次登录时检测默认密码迁移标记并进入改密流程。
- 中期：恢复真实手机号身份字段，增加短信验证码或一次性迁移码找回流程，不要把假邮箱当作账号恢复凭据。
- 运维层面：如果要启用邮件重置，需要先配置 SMTP、`frontend_url`，打开 `email_verify_enabled=true` 和 `password_reset_enabled=true`，但这只适合真实邮箱用户。
