import { describe, expect, it } from 'vitest'
import { meterScale } from '@/utils/meter'

describe('管理页 meter 比例函数', () => {
  it.each([
    ['正常比例', 5, 10, 0.5],
    ['零用量', 0, 10, 0],
    ['空用量', null, 10, 0],
    ['未定义用量', undefined, 10, 0],
    ['负用量', -1, 10, 0],
    ['超过上限', 15, 10, 1],
    ['空上限', 5, null, 0],
    ['未定义上限', 5, undefined, 0],
    ['零上限', 5, 0, 0],
    ['负上限', 5, -10, 0],
    ['无效上限', 5, Number.NaN, 0],
    ['无限上限', 5, Number.POSITIVE_INFINITY, 0],
    ['无效用量', Number.NaN, 10, 0],
    ['无限用量', Number.POSITIVE_INFINITY, 10, 0],
  ])('%s返回稳定的 0..1 比例', (_label, value, limit, expected) => {
    expect(meterScale(value, limit)).toBe(expected)
  })

  it.each([
    ['正常比例', 25, 100, 0.25],
    ['空用量', null, 100, 0],
    ['未定义用量', undefined, 100, 0],
    ['负用量', -1, 100, 0],
    ['超过上限', 120, 100, 1],
    ['空上限', 10, null, 0],
    ['未定义上限', 10, undefined, 0],
    ['零上限', 10, 0, 0],
    ['负上限', 10, -100, 0],
    ['NaN 用量', Number.NaN, 100, 0],
    ['Infinity 用量', Number.POSITIVE_INFINITY, 100, 0],
    ['NaN 上限', 10, Number.NaN, 0],
    ['Infinity 上限', 10, Number.POSITIVE_INFINITY, 0],
  ])('风控 meter 对%s保持安全比例', (_label, value, limit, expected) => {
    expect(meterScale(value, limit)).toBe(expected)
  })

  it.each([
    ['正常配额', 25, 100, 0.25],
    ['空用量', null, 100, 0],
    ['未定义用量', undefined, 100, 0],
    ['负用量', -1, 100, 0],
    ['超限用量', 120, 100, 1],
    ['空上限', 10, null, 0],
    ['未定义上限', 10, undefined, 0],
    ['零上限', 10, 0, 0],
    ['负上限', 10, -100, 0],
    ['NaN 用量', Number.NaN, 100, 0],
    ['无限用量', Number.POSITIVE_INFINITY, 100, 0],
    ['NaN 上限', 10, Number.NaN, 0],
    ['Infinity 上限', 10, Number.POSITIVE_INFINITY, 0],
  ])('网页搜索配额对%s保持安全比例', (_label, used, limit, expected) => {
    expect(meterScale(used, limit)).toBe(expected)
  })
})
