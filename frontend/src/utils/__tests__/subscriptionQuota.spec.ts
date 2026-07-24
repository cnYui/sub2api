import { describe, expect, it } from 'vitest'

import {
  PUBLIC_CODEX_SUBSCRIPTION_VALIDITY_DAYS,
  displayAdminGroupName,
  formatSubscriptionQuotaUSD,
  isPublicCodexSubscriptionGroupName,
  isRollingWeeklySubscription,
  publicCodexSubscriptionDisplayName,
  publicCodexSubscriptionWeeklyLimitUSD,
} from '../subscriptionQuota'

describe('subscriptionQuota', () => {
  it('周额度展示只做整数显示，不改变后端精确判断', () => {
    expect(formatSubscriptionQuotaUSD(71.6)).toBe('$72')
    expect(formatSubscriptionQuotaUSD(71.4)).toBe('$71')
    expect(formatSubscriptionQuotaUSD(null)).toBe('∞')
  })

  it('识别公共 Codex 滚动周订阅，避免页面自行推导刷新时间', () => {
    expect(PUBLIC_CODEX_SUBSCRIPTION_VALIDITY_DAYS).toBe(28)
    expect(publicCodexSubscriptionWeeklyLimitUSD('codex-pool-19-usd')).toBe(76)
    expect(publicCodexSubscriptionWeeklyLimitUSD('codex-pool-128-usd')).toBe(128)
    expect(publicCodexSubscriptionWeeklyLimitUSD('codex-pool-179-usd')).toBe(520)
    expect(publicCodexSubscriptionWeeklyLimitUSD('codex-pool-651-usd')).toBe(651)
    expect(publicCodexSubscriptionWeeklyLimitUSD('codex-pool-781-usd')).toBe(781)
    expect(publicCodexSubscriptionWeeklyLimitUSD('OpenAI')).toBeNull()
    expect(isPublicCodexSubscriptionGroupName('codex-pool-19-usd')).toBe(true)
    expect(isPublicCodexSubscriptionGroupName('codex-pool-651-usd')).toBe(true)
    expect(isPublicCodexSubscriptionGroupName('OpenAI')).toBe(false)
    expect(isRollingWeeklySubscription({
      effective_weekly_limit_usd: null,
      weekly_window_resets_at: null,
      group: { name: 'codex-pool-179-usd' },
    })).toBe(true)
    expect(isRollingWeeklySubscription({
      effective_weekly_limit_usd: null,
      weekly_window_resets_at: null,
      group: { name: 'legacy-weekly' },
    })).toBe(false)
  })

  it('管理页用人民币套餐名展示公共 Codex 分组，内部组名保持不变', () => {
    expect(publicCodexSubscriptionDisplayName('codex-pool-19-usd')).toBe('29 元订阅池')
    expect(publicCodexSubscriptionDisplayName('codex-pool-128-usd')).toBe('49 元订阅池')
    expect(publicCodexSubscriptionDisplayName('codex-pool-179-usd')).toBe('199 元订阅池')
    expect(publicCodexSubscriptionDisplayName('codex-pool-651-usd')).toBe('249 元订阅池')
    expect(publicCodexSubscriptionDisplayName('codex-pool-781-usd')).toBe('299 元订阅池')
    expect(publicCodexSubscriptionDisplayName('traffic-pack-openai')).toBeNull()
    expect(displayAdminGroupName('codex-pool-29-usd')).toBe('39 元订阅池')
    expect(displayAdminGroupName('traffic-pack-openai')).toBe('traffic-pack-openai')
  })
})
