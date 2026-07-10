import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import UserDashboardStats from '../UserDashboardStats.vue'
import type { UserDashboardStats as UserStatsType } from '@/api/usage'

const messages: Record<string, string> = {
  'dashboard.balance': '余额',
  'dashboard.apiKeys': 'API 密钥',
  'dashboard.todayRequests': '今日请求',
  'dashboard.todayCost': '今日消费',
  'dashboard.totalTokens': '总 Token',
  'dashboard.todayTokens': '今日 Token',
  'dashboard.performance': '性能',
  'dashboard.avgResponse': '平均响应',
  'dashboard.averageTime': '平均耗时',
  'dashboard.input': '输入',
  'dashboard.output': '输出',
  'dashboard.actual': '实际',
  'dashboard.standard': '标准',
  'dashboard.requests': '请求',
  'dashboard.tokens': 'Token',
  'dashboard.platformBreakdown': '按平台拆分',
  'dashboard.platformCount': '{count} 个平台',
  'dashboard.platformQuota.title': '平台限额',
  'dashboard.platformQuota.daily': '每日',
  'dashboard.availableModels': '可用模型',
  'dashboard.availableModelsCount': '{count} 个模型',
  'common.active': '启用',
  'common.available': '可用',
  'common.total': '总计',
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        const message = messages[key] ?? key
        return Object.entries(params ?? {}).reduce(
          (text, [name, value]) => text.replace(`{${name}}`, String(value)),
          message,
        )
      },
    }),
  }
})

vi.mock('@/components/icons/Icon.vue', () => ({
  default: {
    props: ['name'],
    template: '<span data-testid="icon" />',
  },
}))

const stats: UserStatsType = {
  total_api_keys: 3,
  active_api_keys: 2,
  total_requests: 120,
  total_input_tokens: 1000,
  total_output_tokens: 800,
  total_cache_creation_tokens: 0,
  total_cache_read_tokens: 0,
  total_tokens: 1800,
  total_cost: 0.02,
  total_actual_cost: 0.012,
  today_requests: 10,
  today_input_tokens: 200,
  today_output_tokens: 100,
  today_cache_creation_tokens: 0,
  today_cache_read_tokens: 0,
  today_tokens: 300,
  today_cost: 0.004,
  today_actual_cost: 0.003,
  average_duration_ms: 520,
  rpm: 2,
  tpm: 60,
  by_platform: [
    {
      platform: 'anthropic',
      total_requests: 8,
      total_tokens: 240,
      total_actual_cost: 0.002,
      today_requests: 2,
      today_tokens: 60,
      today_actual_cost: 0.001,
    },
    {
      platform: 'openai',
      total_requests: 20,
      total_tokens: 600,
      total_actual_cost: 0.01,
      today_requests: 8,
      today_tokens: 240,
      today_actual_cost: 0.002,
    },
  ],
}

describe('UserDashboardStats', () => {
  it('用硬编码可用模型列表替代按平台拆分和平台计费展示', () => {
    const wrapper = mount(UserDashboardStats, {
      props: {
        stats,
        balance: 100,
        isSimple: false,
      },
    })

    expect(wrapper.text()).toContain('可用模型')
    expect(wrapper.text()).toContain('6 个模型')

    for (const model of [
      'gpt-5.6-sol',
      'gpt-5.6-terra',
      'gpt-5.6-luna',
      'gpt-5.5',
      'gpt-5.4',
      'gpt-image-2',
    ]) {
      expect(wrapper.text()).toContain(model)
    }

    for (const model of [
      'gpt-image-1',
      'gpt-image-1.5',
      'gpt-5.4-mini',
      'gpt-5.3-codex',
      'gpt-5.3-codex-spark',
      'codex-auto-review',
      'gpt-5.2',
    ]) {
      expect(wrapper.text()).not.toContain(model)
    }

    expect(wrapper.text()).not.toContain('按平台拆分')
    expect(wrapper.text()).not.toContain('平台限额')
    expect(wrapper.text()).not.toContain('Claude')
    expect(wrapper.text()).not.toContain('OpenAI')
  })
})
