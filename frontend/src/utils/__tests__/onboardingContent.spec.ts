import { describe, expect, it } from 'vitest'
import type { DriveStep } from 'driver.js'
import { normalizeOnboardingSteps } from '@/utils/onboardingContent'

describe('normalizeOnboardingSteps', () => {
  it('移除引导文案中的 emoji，同时保留 HTML 内容和原步骤', () => {
    const steps: DriveStep[] = [{
      popover: {
        title: '👋 欢迎使用',
        description: '<p>🎯 核心 <b>🔑 功能</b></p>',
        nextBtnText: '开始 🚀',
        prevBtnText: '← 返回',
      },
    }]

    const normalized = normalizeOnboardingSteps(steps)
    const popover = normalized[0].popover!

    expect(popover.title).toBe('欢迎使用')
    expect(popover.description).toBe('<p>核心 <b>功能</b></p>')
    expect(popover.nextBtnText).toBe('开始')
    expect(popover.prevBtnText).toBe('← 返回')
    expect(steps[0].popover!.title).toBe('👋 欢迎使用')
  })
})
