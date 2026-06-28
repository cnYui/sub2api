# 注册重复邮箱预检拦截结果

## 目标

新建用户时，如果邮箱已注册，直接在注册页拦截，不跳转到邮箱验证码页，也不触发验证码发送接口，减少无效验证码发送尝试。

## 改动

- 后端新增只读预检能力：
  - `AuthService.PrecheckRegisterEmail(ctx, email)` 复用注册开放、保留邮箱、邮箱后缀和邮箱唯一性校验。
  - 新增 `POST /api/v1/auth/precheck-register`。
  - 该接口只校验，不生成验证码、不写验证码缓存、不入队邮件任务。
  - `/auth/send-verify-code` 与 `/auth/register` 仍保留原有重复邮箱兜底校验，用于处理并发竞态和绕过前端的直接请求。
- 前端新增 `precheckRegister()` API。
- `RegisterView.handleRegister()` 在 `email_verify_enabled=true` 时先调用预检接口：
  - 预检失败：停留在注册页，展示后端错误，不写 `sessionStorage.register_data`，不跳转 `/email-verify`。
  - 预检成功：保持原流程，写入 `register_data` 后跳转 `/email-verify`，由验证码页发送验证码。
- 新增前端回归测试覆盖：
  - 重复邮箱不会跳转验证码页。
  - 重复邮箱不会写入验证码注册数据。
  - 预检通过后才进入邮箱验证页。

## 验证

- 红灯：
  - `cd backend && go test -count=1 -tags=unit ./internal/service -run TestAuthService_PrecheckRegisterEmail`
    - 失败原因：`PrecheckRegisterEmail` 未定义。
  - `cd frontend && npx vitest run src/views/auth/__tests__/RegisterView.spec.ts`
    - 失败原因：`precheckRegister` 未被调用。
- 绿灯与回归：
  - `cd backend && go test -count=1 -tags=unit ./internal/service -run TestAuthService_PrecheckRegisterEmail`
    - 通过。
  - `cd frontend && npx vitest run src/views/auth/__tests__/RegisterView.spec.ts`
    - 3 tests passed。
  - `cd backend && go test -count=1 -tags=unit ./internal/service ./internal/handler ./internal/server/routes`
    - `internal/service`、`internal/handler`、`internal/server/routes` 均通过。
  - `cd frontend && npm run typecheck`
    - 通过。

## 边界

- 本次没有修改 SMTP 配置、邮件队列 worker、验证码缓存格式、注册成功后的履约逻辑或运行态数据库。
- 新预检接口会返回 `EMAIL_EXISTS`，已加注册类限流；如果后续要进一步降低邮箱枚举风险，可以把预检升级为带短期注册会话的流程。
