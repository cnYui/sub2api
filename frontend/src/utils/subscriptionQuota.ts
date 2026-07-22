import type { UserSubscription } from '@/types'

const ONE_DAY_MS = 24 * 60 * 60 * 1000

export interface RemainingDurationParts {
  days: number
  hours: number
  minutes: number
}

const PUBLIC_CODEX_SUBSCRIPTION_WEEKLY_LIMITS_USD = new Map<string, number>([
  ['codex-pool-19-usd', 58],
  ['codex-pool-29-usd', 78],
  ['codex-pool-49-usd', 118],
  ['codex-pool-69-usd', 158],
  ['codex-pool-89-usd', 198],
  ['codex-pool-135-usd', 299],
  ['codex-pool-179-usd', 400],
])

const PUBLIC_CODEX_SUBSCRIPTION_DISPLAY_NAMES = new Map<string, string>([
  ['codex-pool-19-usd', '29 元订阅池'],
  ['codex-pool-29-usd', '39 元订阅池'],
  ['codex-pool-49-usd', '59 元订阅池'],
  ['codex-pool-69-usd', '79 元订阅池'],
  ['codex-pool-89-usd', '99 元订阅池'],
  ['codex-pool-135-usd', '149 元订阅池'],
  ['codex-pool-179-usd', '199 元订阅池'],
])

export const PUBLIC_CODEX_SUBSCRIPTION_VALIDITY_DAYS = 28

type RollingWeeklySubscriptionLike = Pick<
  UserSubscription,
  'effective_weekly_limit_usd' | 'weekly_window_resets_at'
> & {
  group?: { name?: string | null } | null
}

// 周额度页面统一显示整数；实际限额始终由后端返回的精确值决定。
export function formatSubscriptionQuotaUSD(value: number | null | undefined): string {
  if (value == null || !Number.isFinite(value)) return '∞'
  return `$${Math.round(value)}`
}

export function publicCodexSubscriptionDisplayName(groupName: string | null | undefined): string | null {
  if (!groupName) return null
  return PUBLIC_CODEX_SUBSCRIPTION_DISPLAY_NAMES.get(groupName.trim()) ?? null
}

export function displayAdminGroupName(groupName: string | null | undefined): string {
  if (!groupName) return ''
  const normalized = groupName.trim()
  return publicCodexSubscriptionDisplayName(normalized) ?? normalized
}

export function isPublicCodexSubscriptionGroupName(groupName: string | null | undefined): boolean {
  return publicCodexSubscriptionWeeklyLimitUSD(groupName) != null
}

export function publicCodexSubscriptionWeeklyLimitUSD(groupName: string | null | undefined): number | null {
  if (!groupName) return null
  return PUBLIC_CODEX_SUBSCRIPTION_WEEKLY_LIMITS_USD.get(groupName) ?? null
}

export function isRollingWeeklySubscription(subscription: RollingWeeklySubscriptionLike): boolean {
  return Boolean(
    subscription.effective_weekly_limit_usd != null ||
      subscription.weekly_window_resets_at ||
      isPublicCodexSubscriptionGroupName(subscription.group?.name),
  )
}

export function isOneTimeDailyQuota(
  subscription: Pick<UserSubscription, 'starts_at' | 'expires_at'>
): boolean {
  if (!subscription.starts_at || !subscription.expires_at) return false

  const startsAt = new Date(subscription.starts_at).getTime()
  const expiresAt = new Date(subscription.expires_at).getTime()

  if (!Number.isFinite(startsAt) || !Number.isFinite(expiresAt)) return false

  return expiresAt <= startsAt + ONE_DAY_MS
}

export function getRemainingDurationParts(
  targetAt: Date | string,
  now: Date = new Date()
): RemainingDurationParts | null {
  const targetTime = targetAt instanceof Date ? targetAt.getTime() : new Date(targetAt).getTime()
  const nowTime = now.getTime()

  if (!Number.isFinite(targetTime) || !Number.isFinite(nowTime)) return null

  const diffMs = targetTime - nowTime
  if (diffMs <= 0) return null

  const totalMinutes = Math.floor(diffMs / (1000 * 60))
  const days = Math.floor(totalMinutes / (24 * 60))
  const hours = Math.floor((totalMinutes % (24 * 60)) / 60)
  const minutes = totalMinutes % 60

  return { days, hours, minutes }
}
