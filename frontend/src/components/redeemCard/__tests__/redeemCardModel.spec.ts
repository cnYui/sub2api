import { describe, expect, it } from 'vitest'

import {
  CARD_HEIGHT,
  CARD_RADIUS,
  CARD_THICKNESS,
  CARD_WIDTH,
  borderColorAt,
  cardEdgeSegments,
  codeFontSize,
  codeLines,
  codeStatusStamp,
  contentFromRedeemCode,
  formatSerial,
  heatmapColors
} from '../redeemCardModel'

describe('redeemCardModel', () => {
  it('厚度按 0.6mm、20px/mm 换算成 12 个设计像素', () => {
    expect(CARD_THICKNESS).toBe(12)
    expect(CARD_WIDTH / CARD_HEIGHT).toBeCloseTo(85.6 / 54, 3)
  })

  it('热力图 52 周 × 7 天，同一个种子画出同一张图，颜色只取当前主题的色阶', () => {
    const dark = heatmapColors(7, 'dark')
    expect(dark).toHaveLength(364)
    expect(heatmapColors(7, 'dark')).toEqual(dark)
    expect(heatmapColors(8, 'dark')).not.toEqual(dark)
    const palette = new Set(['#161b22', '#0e4429', '#006d32', '#26a641', '#39d353'])
    expect(dark.every((color) => palette.has(color))).toBe(true)
    // 黑白两版只换色阶，格子的深浅分布一致。
    const light = heatmapColors(7, 'light')
    const level = (colors: string[], palette: string[]) => colors.map((color) => palette.indexOf(color))
    expect(level(light, ['#ebedf0', '#9be9a8', '#40c463', '#30a14e', '#216e39'])).toEqual(
      level(dark, [...palette])
    )
  })

  it('兑换码 4 位一组、每行 4 组，不增删字符也不改大小写', () => {
    const code = 'a3F9c2e17b4d58e0c6a1f93b2d7e4c85'
    const lines = codeLines(code)
    expect(lines).toEqual([
      ['a3F9', 'c2e1', '7b4d', '58e0'],
      ['c6a1', 'f93b', '2d7e', '4c85']
    ])
    expect(lines.flat().join('')).toBe(code)
    expect(codeFontSize(lines.length)).toBe(92)
    expect(codeFontSize(1)).toBe(112)
    expect(codeFontSize(3)).toBe(72)
  })

  it('余额兑换码自动填写面值和到期日', () => {
    const content = contentFromRedeemCode(
      { id: 42, type: 'balance', value: 10, expires_at: '2026-10-01T00:00:00Z' },
      123
    )
    expect(content).toEqual({
      amount: '$10',
      plan: '余额充值',
      tokens: '',
      valid_until: '2026-10-01',
      serial: '0042',
      heatmap_seed: 123
    })
    expect(formatSerial(content.serial)).toBe('No.0042')
  })

  it('余额套餐兑换码按档位填写价格、有效天数和每期额度', () => {
    const content = contentFromRedeemCode({
      id: 7,
      type: 'balance_package',
      value: 0,
      expires_at: null,
      balance_package_plan: {
        name: '标准档',
        price_cny: 29.9,
        weekly_credit_usd: 50,
        validity_days: 28,
        refresh_count: 4
      }
    })
    expect(content.amount).toBe('¥29.90')
    expect(content.plan).toBe('标准档 · 28 天')
    expect(content.tokens).toBe('每期 $50 × 4 期')
    expect(content.valid_until).toBe('长期有效')
  })

  it('兑换码不能再用时才盖章', () => {
    expect(codeStatusStamp('used')).toBe('已兑换')
    expect(codeStatusStamp('expired')).toBe('已过期')
    expect(codeStatusStamp('disabled')).toBe('已停用')
    expect(codeStatusStamp('unused')).toBe('')
    expect(codeStatusStamp(undefined)).toBe('')
  })

  it('描边颜色与 135° 渐变的三个色标一致', () => {
    expect(borderColorAt(0, 0)).toBe('#2b8a99')
    expect(borderColorAt(CARD_WIDTH, CARD_HEIGHT)).toBe('#8e2f72')
    expect(borderColorAt(CARD_WIDTH / 2, CARD_HEIGHT / 2)).toBe('#3a3f6e')
    expect(borderColorAt(0, 0, 0.5)).toBe('#16454d')
  })

  it('侧边小条沿圆角矩形外轮廓首尾相接，总长等于周长', () => {
    const segments = cardEdgeSegments()
    const W = CARD_WIDTH
    const H = CARD_HEIGHT
    const R = CARD_RADIUS
    // 每条中心到圆角矩形轮廓的距离都应接近 0。
    const distanceToOutline = (x: number, y: number) => {
      const cx = Math.min(Math.max(x, R), W - R)
      const cy = Math.min(Math.max(y, R), H - R)
      const inCorner = (x < R || x > W - R) && (y < R || y > H - R)
      if (inCorner) return Math.abs(Math.hypot(x - cx, y - cy) - R)
      return Math.min(Math.abs(x), Math.abs(W - x), Math.abs(y), Math.abs(H - y))
    }
    for (const seg of segments) {
      expect(distanceToOutline(seg.x, seg.y)).toBeLessThan(0.5)
    }
    const perimeter = 2 * (W - 2 * R) + 2 * (H - 2 * R) + 2 * Math.PI * R
    const total = segments.reduce((sum, seg) => sum + seg.length - 1, 0)
    expect(Math.abs(total - perimeter) / perimeter).toBeLessThan(0.002)

    // 背面翻转过：左上角那条侧边，正面接的是描边起点色，背面接的是右上角的颜色。
    const topLeft = segments.reduce((best, seg) => (seg.x + seg.y < best.x + best.y ? seg : best))
    expect(topLeft.front).toBe(borderColorAt(topLeft.x, topLeft.y, 0.82))
    expect(topLeft.back).toBe(borderColorAt(W - topLeft.x, topLeft.y, 0.82))
  })
})
