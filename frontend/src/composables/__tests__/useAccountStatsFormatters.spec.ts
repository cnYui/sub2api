import { describe, expect, it } from 'vitest'
import { useAccountStatsFormatters } from '../useAccountStatsFormatters'

describe('useAccountStatsFormatters', () => {
  it('格式化成本、数量、token 和耗时', () => {
    const { formatCost, formatNumber, formatTokens, formatDuration } = useAccountStatsFormatters()

    expect(formatCost(1234)).toBe('1.23K')
    expect(formatCost(12.3)).toBe('12.30')
    expect(formatCost(0.1234)).toBe('0.123')
    expect(formatCost(0.0012)).toBe('0.0012')

    expect(formatNumber(1234567)).toBe('1.23M')
    expect(formatNumber(12345)).toBe('12.35K')
    expect(formatNumber(123)).toBe('123')

    expect(formatTokens(1234567890)).toBe('1.23B')
    expect(formatTokens(1234567)).toBe('1.23M')
    expect(formatTokens(12345)).toBe('12.35K')
    expect(formatTokens(123)).toBe('123')

    expect(formatDuration(1234)).toBe('1.23s')
    expect(formatDuration(12.3)).toBe('12ms')
  })
})
