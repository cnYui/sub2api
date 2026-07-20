import { describe, expect, it } from 'vitest'
import { meterScale } from '../meter'

describe('meterScale', () => {
  it.each([
    ['正常比例', 5, 10, 0.5],
    ['零用量', 0, 10, 0],
    ['空用量', null, 10, 0],
    ['未定义用量', undefined, 10, 0],
    ['负用量', -1, 10, 0],
    ['超限用量', 15, 10, 1],
    ['空上限', 5, null, 0],
    ['未定义上限', 5, undefined, 0],
    ['零上限', 5, 0, 0],
    ['负上限', 5, -10, 0],
    ['NaN 用量', Number.NaN, 10, 0],
    ['Infinity 用量', Number.POSITIVE_INFINITY, 10, 0],
    ['NaN 上限', 5, Number.NaN, 0],
    ['Infinity 上限', 5, Number.POSITIVE_INFINITY, 0],
  ])('%s返回稳定的 0..1 比例', (_label, value, limit, expected) => {
    expect(meterScale(value, limit)).toBe(expected)
  })
})
