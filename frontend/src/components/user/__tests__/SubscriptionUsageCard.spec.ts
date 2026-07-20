import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import SubscriptionUsageCard from '../SubscriptionUsageCard.vue'

const baseProps = {
  title: 'OpenAI',
  platform: 'openai',
  status: 'active',
  statusLabel: '正常',
  expirationLabel: '到期时间',
  expirationValue: '永久',
  unlimitedTitle: '无限额度',
  unlimitedDescription: '当前没有用量限制',
}

describe('SubscriptionUsageCard', () => {
  it.each([
    ['0%', '0'],
    ['50%', '0.5'],
    ['100%', '1'],
    ['150%', '1'],
    ['-25%', '0'],
    ['', '0'],
    ['NaN%', '0'],
    ['not-a-percentage', '0'],
  ])('把进度百分比 %s 安全转换为 scaleX 值 %s', (progressWidth, expectedValue) => {
    const wrapper = mount(SubscriptionUsageCard, {
      props: {
        ...baseProps,
        usageRows: [
          {
            label: '日额度',
            value: '$0.00 / $1.00',
            progressWidth,
            progressClass: 'bg-primary-500',
            testId: 'usage-progress',
          },
        ],
      },
    })

    expect(wrapper.get('[data-testid="usage-progress"]').attributes('style')).toContain(
      `--meter-value: ${expectedValue}`
    )
  })
})
