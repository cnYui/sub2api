import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import RegisterView from '@/views/auth/RegisterView.vue'

const {
  pushMock,
  showErrorMock,
  registerMock,
  getPublicSettingsMock,
  precheckRegisterMock,
  validatePromoCodeMock,
  validateInvitationCodeMock,
  routeState,
} = vi.hoisted(() => ({
  pushMock: vi.fn(),
  showErrorMock: vi.fn(),
  registerMock: vi.fn(),
  getPublicSettingsMock: vi.fn(),
  precheckRegisterMock: vi.fn(),
  validatePromoCodeMock: vi.fn(),
  validateInvitationCodeMock: vi.fn(),
  routeState: {
    query: {} as Record<string, string>,
  },
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({
    push: pushMock,
  }),
  useRoute: () => routeState,
}))

vi.mock('vue-i18n', () => ({
  createI18n: () => ({
    global: {
      t: (key: string) => key,
    },
  }),
  useI18n: () => ({
    t: (key: string, params?: Record<string, string | number>) => {
      if (key === 'auth.signUpToStart') {
        return `注册以开始使用 ${params?.siteName ?? 'Sub2API'}`
      }
      return key
    },
    locale: { value: 'zh' },
  }),
}))

vi.mock('@/stores', () => ({
  useAuthStore: () => ({
    register: (...args: any[]) => registerMock(...args),
  }),
  useAppStore: () => ({
    showError: (...args: any[]) => showErrorMock(...args),
  }),
}))

vi.mock('@/api/auth', () => ({
  getPublicSettings: (...args: any[]) => getPublicSettingsMock(...args),
  isWeChatWebOAuthEnabled: () => false,
  precheckRegister: (...args: any[]) => precheckRegisterMock(...args),
  validatePromoCode: (...args: any[]) => validatePromoCodeMock(...args),
  validateInvitationCode: (...args: any[]) => validateInvitationCodeMock(...args),
}))

const publicSettings = {
  registration_enabled: true,
  email_verify_enabled: false,
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
}

const registerViewStubs = {
  AuthLayout: { template: '<div><slot /><slot name="footer" /></div>' },
  Icon: true,
  TurnstileWidget: true,
  LoginAgreementPrompt: true,
  EmailOAuthButtons: true,
  LinuxDoOAuthSection: true,
  WechatOAuthSection: true,
  OidcOAuthSection: true,
  RouterLink: true,
  transition: false,
}

describe('RegisterView', () => {
  beforeEach(() => {
    pushMock.mockReset()
    showErrorMock.mockReset()
    registerMock.mockReset()
    getPublicSettingsMock.mockReset()
    precheckRegisterMock.mockReset()
    validatePromoCodeMock.mockReset()
    validateInvitationCodeMock.mockReset()
    routeState.query = {}
    sessionStorage.clear()
    localStorage.clear()

    getPublicSettingsMock.mockResolvedValue(publicSettings)
    precheckRegisterMock.mockResolvedValue({ ok: true })
  })

  it('即使公开设置开启优惠码，也不渲染注册优惠码输入框', async () => {
    const wrapper = mount(RegisterView, {
      global: {
        stubs: registerViewStubs,
      },
    })

    await flushPromises()

    expect(wrapper.find('#email').exists()).toBe(true)
    expect(wrapper.find('#password').exists()).toBe(true)
    expect(wrapper.find('#promo_code').exists()).toBe(false)
    expect(validatePromoCodeMock).not.toHaveBeenCalled()
  })

  it('邮箱验证开启且邮箱已存在时留在注册页并不写入验证码注册数据', async () => {
    getPublicSettingsMock.mockResolvedValue({
      ...publicSettings,
      email_verify_enabled: true,
    })
    precheckRegisterMock.mockRejectedValue({
      reason: 'EMAIL_EXISTS',
      message: 'email already exists',
    })

    const wrapper = mount(RegisterView, {
      global: {
        stubs: registerViewStubs,
      },
    })

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
      ...publicSettings,
      email_verify_enabled: true,
    })

    const wrapper = mount(RegisterView, {
      global: {
        stubs: registerViewStubs,
      },
    })

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
})
