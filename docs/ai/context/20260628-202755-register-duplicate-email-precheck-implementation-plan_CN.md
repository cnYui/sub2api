# 注册重复邮箱预检拦截 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 邮箱验证码注册时，重复邮箱在注册页被拦截，不进入 `/email-verify`，也不触发验证码发送接口。

**Architecture:** 后端新增只校验不发信的注册预检能力，复用注册开放、保留邮箱、邮箱后缀和邮箱唯一性规则。前端 `RegisterView` 在邮箱验证开启时先调用预检接口，成功后才保存 `register_data` 并跳转验证码页。

**Tech Stack:** Go/Gin 后端、Vue 3/TypeScript 前端、Vitest、Go unit tests。

---

### Task 1: 后端注册预检服务与接口

**Files:**
- Modify: `backend/internal/service/auth_service.go`
- Modify: `backend/internal/handler/auth_handler.go`
- Modify: `backend/internal/server/routes/auth.go`
- Test: `backend/internal/service/auth_service_register_test.go`

- [ ] **Step 1: Write the failing tests**

在 `backend/internal/service/auth_service_register_test.go` 增加：

```go
func TestAuthService_PrecheckRegisterEmail_EmailExists(t *testing.T) {
	repo := &userRepoStub{exists: true}
	service := newAuthService(repo, map[string]string{
		SettingKeyRegistrationEnabled: "true",
	}, nil, nil)

	err := service.PrecheckRegisterEmail(context.Background(), "user@test.com")
	require.ErrorIs(t, err, ErrEmailExists)
}

func TestAuthService_PrecheckRegisterEmail_EmailSuffixNotAllowed(t *testing.T) {
	repo := &userRepoStub{}
	service := newAuthService(repo, map[string]string{
		SettingKeyRegistrationEnabled:              "true",
		SettingKeyRegistrationEmailSuffixWhitelist: `["@example.com"]`,
	}, nil, nil)

	err := service.PrecheckRegisterEmail(context.Background(), "user@other.com")
	require.ErrorIs(t, err, ErrEmailSuffixNotAllowed)
}

func TestAuthService_PrecheckRegisterEmail_Success(t *testing.T) {
	repo := &userRepoStub{}
	service := newAuthService(repo, map[string]string{
		SettingKeyRegistrationEnabled: "true",
	}, nil, nil)

	err := service.PrecheckRegisterEmail(context.Background(), "user@test.com")
	require.NoError(t, err)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test -count=1 -tags=unit ./internal/service -run TestAuthService_PrecheckRegisterEmail`

Expected: FAIL because `PrecheckRegisterEmail` is undefined.

- [ ] **Step 3: Write minimal implementation**

Add `PrecheckRegisterEmail(ctx, email)` in `AuthService` and add handler/route:

```go
func (s *AuthService) PrecheckRegisterEmail(ctx context.Context, email string) error {
	if s.settingService == nil || !s.settingService.IsRegistrationEnabled(ctx) {
		return ErrRegDisabled
	}
	if isReservedEmail(email) {
		return ErrEmailReserved
	}
	if err := s.validateRegistrationEmailPolicy(ctx, email); err != nil {
		return err
	}
	existsEmail, err := s.userRepo.ExistsByEmail(ctx, email)
	if err != nil {
		logger.LegacyPrintf("service.auth", "[Auth] Database error checking email exists: %v", err)
		return ErrServiceUnavailable
	}
	if existsEmail {
		return ErrEmailExists
	}
	return nil
}
```

- [ ] **Step 4: Run backend test to verify it passes**

Run: `cd backend && go test -count=1 -tags=unit ./internal/service -run TestAuthService_PrecheckRegisterEmail`

Expected: PASS.

### Task 2: 前端注册页预检

**Files:**
- Modify: `frontend/src/types/index.ts`
- Modify: `frontend/src/api/auth.ts`
- Modify: `frontend/src/views/auth/RegisterView.vue`
- Test: `frontend/src/views/auth/__tests__/RegisterView.spec.ts`

- [ ] **Step 1: Write the failing tests**

在 `RegisterView.spec.ts` mock `precheckRegister`，增加两个用例：

```ts
it('邮箱验证开启且邮箱已存在时留在注册页并不写入验证码注册数据', async () => {
  getPublicSettingsMock.mockResolvedValue({
    registration_enabled: true,
    email_verify_enabled: true,
    promo_code_enabled: true,
    invitation_code_enabled: false,
    turnstile_enabled: false,
    turnstile_site_key: '',
    site_name: '天才程序员小站',
    linuxdo_oauth_enabled: false,
    wechat_oauth_enabled: false,
    oidc_oauth_enabled: false,
    oidc_oauth_provider_name: 'OIDC',
    github_oauth_enabled: false,
    google_oauth_enabled: false,
    registration_email_suffix_whitelist: [],
  })
  precheckRegisterMock.mockRejectedValue({
    reason: 'EMAIL_EXISTS',
    message: 'email already exists',
  })

  const wrapper = mount(RegisterView, { global: { stubs: registerViewStubs } })
  await flushPromises()
  await wrapper.find('#email').setValue('exists@example.com')
  await wrapper.find('#password').setValue('secret-123')
  await wrapper.find('form').trigger('submit.prevent')
  await flushPromises()

  expect(precheckRegisterMock).toHaveBeenCalledWith({ email: 'exists@example.com' })
  expect(pushMock).not.toHaveBeenCalledWith('/email-verify')
  expect(sessionStorage.getItem('register_data')).toBeNull()
  expect(showErrorMock).toHaveBeenCalledWith('email already exists')
})

it('邮箱验证开启且预检通过时才进入邮箱验证页', async () => {
  getPublicSettingsMock.mockResolvedValue({
    registration_enabled: true,
    email_verify_enabled: true,
    promo_code_enabled: true,
    invitation_code_enabled: false,
    turnstile_enabled: false,
    turnstile_site_key: '',
    site_name: '天才程序员小站',
    linuxdo_oauth_enabled: false,
    wechat_oauth_enabled: false,
    oidc_oauth_enabled: false,
    oidc_oauth_provider_name: 'OIDC',
    github_oauth_enabled: false,
    google_oauth_enabled: false,
    registration_email_suffix_whitelist: [],
  })
  precheckRegisterMock.mockResolvedValue({ ok: true })

  const wrapper = mount(RegisterView, { global: { stubs: registerViewStubs } })
  await flushPromises()
  await wrapper.find('#email').setValue('fresh@example.com')
  await wrapper.find('#password').setValue('secret-123')
  await wrapper.find('form').trigger('submit.prevent')
  await flushPromises()

  expect(precheckRegisterMock).toHaveBeenCalledWith({ email: 'fresh@example.com' })
  expect(pushMock).toHaveBeenCalledWith('/email-verify')
  expect(JSON.parse(sessionStorage.getItem('register_data') || '{}')).toMatchObject({
    email: 'fresh@example.com',
    password: 'secret-123',
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd frontend && npm test -- RegisterView.spec.ts`

Expected: FAIL because `precheckRegister` is not implemented/called.

- [ ] **Step 3: Write minimal implementation**

Add `PrecheckRegisterRequest/Response`, add API wrapper, import and call `precheckRegister()` before writing `register_data`.

- [ ] **Step 4: Run frontend test to verify it passes**

Run: `cd frontend && npm test -- RegisterView.spec.ts`

Expected: PASS.

### Task 3: Final verification and context

**Files:**
- Create: `docs/ai/context/YYYYMMDD-HHMMSS-register-duplicate-email-precheck-result_CN.md`

- [ ] **Step 1: Run focused backend tests**

Run: `cd backend && go test -count=1 -tags=unit ./internal/service -run 'TestAuthService_(PrecheckRegisterEmail|SendVerifyCode|Register)'`

- [ ] **Step 2: Run focused frontend tests**

Run: `cd frontend && npm test -- RegisterView.spec.ts`

- [ ] **Step 3: Record result context**

Create a result context file describing changed files, test commands, and known boundary that backend send/register endpoints still keep duplicate-email defense.
