import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import MonitorMetricPair from './MonitorMetricPair.vue'

describe('MonitorMetricPair', () => {
  it('未提供第二指标时只渲染模型目录指标', () => {
    const wrapper = mount(MonitorMetricPair, {
      props: {
        primaryIcon: 'database',
        primaryLabel: '模型目录延迟',
        primaryValue: '134',
        primaryUnit: 'ms',
      },
    })

    expect(wrapper.text()).toContain('模型目录延迟')
    expect(wrapper.text()).not.toContain('端点 PING')
    expect(wrapper.findAll('.rounded-xl')).toHaveLength(1)
    expect(wrapper.classes()).toContain('grid-cols-1')
  })

  it('提供第二指标时保持双指标布局', () => {
    const wrapper = mount(MonitorMetricPair, {
      props: {
        primaryIcon: 'database',
        primaryLabel: '模型目录延迟',
        primaryValue: '134',
        primaryUnit: 'ms',
        secondaryIcon: 'globe',
        secondaryLabel: '目录备用指标',
        secondaryValue: '200',
        secondaryUnit: 'ms',
      },
    })

    expect(wrapper.findAll('.rounded-xl')).toHaveLength(2)
    expect(wrapper.classes()).toContain('grid-cols-2')
  })
})
