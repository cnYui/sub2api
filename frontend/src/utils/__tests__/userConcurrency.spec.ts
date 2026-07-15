import { describe, expect, it } from 'vitest'
import { isValidUserConcurrency } from '../userConcurrency'

describe('isValidUserConcurrency', () => {
  it.each([0, 1, 5, 100])('接受非负整数 %s', (value) => {
    expect(isValidUserConcurrency(value)).toBe(true)
  })

  it.each([-1, 0.5, Number.NaN, Number.POSITIVE_INFINITY, Number.NEGATIVE_INFINITY])(
    '拒绝非法并发值 %s',
    (value) => {
      expect(isValidUserConcurrency(value)).toBe(false)
    }
  )
})
