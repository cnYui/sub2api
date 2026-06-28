# 注册重复邮箱预检拦截方案

## 背景

- 用户反馈：新建用户时，如果邮箱已注册，当前前端仍会进入邮箱验证码页，并在验证码页弹出 `email already exists`。
- 期望行为：重复邮箱应在注册页直接拦截，不跳转到验证码页，也不触发验证码发送流程。
- 当前证据：
  - `frontend/src/views/auth/RegisterView.vue` 在 `emailVerifyEnabled=true` 时只做本地表单校验，随后写入 `sessionStorage.register_data` 并跳转 `/email-verify`。
  - `frontend/src/views/auth/EmailVerifyView.vue` 挂载后读取 `register_data`，自动调用 `sendVerifyCode()`。
  - `backend/internal/service/auth_service.go` 的 `SendVerifyCodeAsync()` 已在入队发信前检查 `ExistsByEmail()` 并返回 `ErrEmailExists`，因此重复邮箱一般不会真正发出邮件，但用户已经进入了验证码页。

## 可选方案

### 方案 A：注册页直接调用现有发送验证码接口

- 做法：`RegisterView` 在跳转前调用 `/auth/send-verify-code`；成功后跳到验证码页，失败则留在注册页。
- 优点：改动最小，不新增后端接口。
- 缺点：重复邮箱仍然会请求“发送验证码”接口，只是后端提前返回；语义上不满足“不进入发送验证码环节”。

### 方案 B：新增注册预检接口（推荐）

- 做法：新增 `/auth/precheck-register`，复用后端注册开放、保留邮箱、邮箱后缀、邮箱是否存在等校验，但不生成、不缓存、不发送验证码。
- 前端：`RegisterView` 在邮箱验证开启时，先调用预检接口；预检失败留在注册页并展示错误；预检成功才写入 `register_data` 并跳转 `/email-verify`，由验证码页继续发送验证码。
- 优点：重复邮箱不会触发发送验证码接口，符合用户期望；后端仍在发送接口保留兜底校验，避免竞态。
- 缺点：需要新增一个很小的后端 API 和前端 API 封装。

### 方案 C：调整注册接口校验顺序并复用注册接口做预检

- 做法：让 `/auth/register` 在验证码校验前先检查邮箱是否存在，然后前端用缺失验证码的注册请求做预检。
- 优点：少一个公开接口。
- 缺点：接口语义混乱，预检会返回 `EMAIL_VERIFY_REQUIRED` 作为成功分支，不利于维护。

## 推荐设计

采用方案 B。

- 后端新增 `AuthService.PrecheckRegisterEmail(ctx, email)`：
  - 检查注册是否开启。
  - 拦截保留邮箱。
  - 复用 `validateRegistrationEmailPolicy()`。
  - 查询 `userRepo.ExistsByEmail()`；存在则返回 `ErrEmailExists`。
  - 不接触 `emailService`、`emailQueueService`、验证码缓存和邮件队列。
- 后端新增 handler `PrecheckRegister`：
  - 请求体只需要 `email`。
  - 成功返回 `{ ok: true }`。
  - 错误复用现有 `response.ErrorFrom()`，保持 `EMAIL_EXISTS`、`EMAIL_SUFFIX_NOT_ALLOWED` 等错误结构。
- 路由新增 `POST /api/v1/auth/precheck-register`：
  - 复用注册类接口的限流策略，避免被滥用枚举。
- 前端新增 `precheckRegister()` API。
- `RegisterView.handleRegister()`：
  - 当 `emailVerifyEnabled=true` 时，先调用 `precheckRegister({ email })`。
  - 如果预检报错，保留在注册页、清理 Turnstile token、显示现有错误消息，不写 `register_data`，不跳转。
  - 预检成功后保持现有跳转逻辑。
- `EmailVerifyView` 不改自动发送逻辑；它仍是进入验证码页后的验证码发送入口。

## 测试计划

- 后端 service 单测：
  - 重复邮箱返回 `ErrEmailExists`。
  - 注册关闭返回 `ErrRegDisabled`。
  - 邮箱后缀不允许返回 `ErrEmailSuffixNotAllowed`。
  - 正常邮箱通过。
- 前端 `RegisterView` 单测：
  - 邮箱验证开启且预检返回 `EMAIL_EXISTS` 时，不调用 `router.push('/email-verify')`，不写 `sessionStorage.register_data`，显示错误。
  - 邮箱验证开启且预检成功时，写入 `register_data` 并跳转 `/email-verify`。
- 验证命令：
  - `go test -count=1 -tags=unit ./internal/service`
  - `cd frontend && npm test -- RegisterView.spec.ts`

## 边界说明

- 预检只能减少已知重复邮箱进入验证码发送流程，不能替代后端发送接口和最终注册接口的重复邮箱兜底校验。
- 如果预检通过后邮箱被并发注册，`/auth/send-verify-code` 或 `/auth/register` 仍会返回 `EMAIL_EXISTS`。
