import { describe, expect, it } from 'vitest'
import { getProgressScale } from '../SubscriptionsView.vue'
import { meterScale as riskMeterScale } from '../RiskControlView.vue'
import { quotaScale as webSearchQuotaScale } from '../SettingsView.vue'

describe('管理页 meter 比例函数', () => {
  it.each([
    ['正常比例', 5, 10, 0.5],
    ['零用量', 0, 10, 0],
    ['负用量', -1, 10, 0],
    ['超过上限', 15, 10, 1],
    ['零上限', 5, 0, 0],
    ['负上限', 5, -10, 0],
    ['无效上限', 5, Number.NaN, 0],
    ['无限上限', 5, Number.POSITIVE_INFINITY, 0],
    ['无效用量', Number.NaN, 10, 0],
    ['无限用量', Number.POSITIVE_INFINITY, 10, 0],
  ])('%s返回稳定的 0..1 比例', (_label, value, limit, expected) => {
    expect(getProgressScale(value, limit)).toBe(expected)
  })

  it.each([
    ['负队列值', -1, 100, 0],
    ['超过队列上限', 120, 100, 1],
    ['非有限值', Number.NaN, 100, 0],
    ['无限值', Number.POSITIVE_INFINITY, 100, 0],
  ])('风控 meter 对%s保持安全比例', (_label, value, limit, expected) => {
    expect(riskMeterScale(value, limit)).toBe(expected)
  })

  it.each([
    ['正常配额', 25, 100, 0.25],
    ['负用量', -1, 100, 0],
    ['超限用量', 120, 100, 1],
    ['无效上限', 10, Number.NaN, 0],
    ['无限用量', Number.POSITIVE_INFINITY, 100, 0],
  ])('网页搜索配额对%s保持安全比例', (_label, used, limit, expected) => {
    expect(webSearchQuotaScale(used, limit)).toBe(expected)
  })
})
