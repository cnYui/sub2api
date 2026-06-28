import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import RegisterView from '@/views/auth/RegisterView.vue'

const {
  pushMock,
  showErrorMock,
  registerMock,
  getPublicSettingsMock,
  validatePromoCodeMock,
  validateInvitationCodeMock,
  routeState,
} = vi.hoisted(() => ({
  pushMock: vi.fn(),
  showErrorMock: vi.fn(),
  registerMock: vi.fn(),
  getPublicSettingsMock: vi.fn(),
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
  validatePromoCode: (...args: any[]) => validatePromoCodeMock(...args),
  validateInvitationCode: (...args: any[]) => validateInvitationCodeMock(...args),
}))

describe('RegisterView', () => {
  beforeEach(() => {
    pushMock.mockReset()
    showErrorMock.mockReset()
    registerMock.mockReset()
    getPublicSettingsMock.mockReset()
    validatePromoCodeMock.mockReset()
    validateInvitationCodeMock.mockReset()
    routeState.query = {}
    sessionStorage.clear()
    localStorage.clear()

    getPublicSettingsMock.mockResolvedValue({
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
    })
  })

  it('即使公开设置开启优惠码，也不渲染注册优惠码输入框', async () => {
    const wrapper = mount(RegisterView, {
      global: {
        stubs: {
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
        },
      },
    })

    await flushPromises()

    expect(wrapper.find('#email').exists()).toBe(true)
    expect(wrapper.find('#password').exists()).toBe(true)
    expect(wrapper.find('#promo_code').exists()).toBe(false)
    expect(validatePromoCodeMock).not.toHaveBeenCalled()
  })
})
