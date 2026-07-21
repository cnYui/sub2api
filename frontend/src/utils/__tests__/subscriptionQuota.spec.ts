import { describe, expect, it } from 'vitest'

import {
  PUBLIC_CODEX_SUBSCRIPTION_VALIDITY_DAYS,
  formatSubscriptionQuotaUSD,
  isPublicCodexSubscriptionGroupName,
  isRollingWeeklySubscription,
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
    expect(publicCodexSubscriptionWeeklyLimitUSD('codex-pool-19-usd')).toBe(72)
    expect(publicCodexSubscriptionWeeklyLimitUSD('codex-pool-179-usd')).toBe(500)
    expect(publicCodexSubscriptionWeeklyLimitUSD('OpenAI')).toBeNull()
    expect(isPublicCodexSubscriptionGroupName('codex-pool-19-usd')).toBe(true)
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
})
