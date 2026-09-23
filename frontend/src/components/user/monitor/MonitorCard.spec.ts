import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import type { UserMonitorView } from '@/api/channelMonitor'
import MonitorCard from './MonitorCard.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key }),
}))

function makeItem(overrides: Partial<UserMonitorView> = {}): UserMonitorView {
  return {
    id: 1,
    name: 'Kimi1倍率',
    provider: 'openai',
    group_name: 'Kimi1倍率',
    primary_model: 'kimi-k3',
    primary_status: 'operational',
    primary_latency_ms: 120,
    primary_ping_latency_ms: null,
    availability_7d: 99.5,
    extra_models: [
      { model: 'kimi-k2.6', status: 'failed', latency_ms: 120 },
    ],
    timeline: [],
    ...overrides,
  }
}

function mountCard(item: UserMonitorView) {
  return mount(MonitorCard, {
    props: { item, window: '7d', availabilityValue: 99.5, countdownSeconds: 30 },
  })
}

describe('MonitorCard', () => {
  it('列出主模型和全部附加模型，并按状态着色', () => {
    const wrapper = mountCard(makeItem())
    const chips = wrapper.find('[data-testid="monitor-card-models"]').findAll(':scope > span')

    expect(chips.map(c => c.text())).toEqual(['kimi-k3', 'kimi-k2.6'])
    expect(chips[0].find('.rounded-full').classes()).toContain('bg-emerald-500')
    expect(chips[1].find('.rounded-full').classes()).toContain('bg-red-500')
  })

  it('分组名与卡片名相同时不重复显示分组标签', () => {
    const same = mountCard(makeItem())
    expect(same.text().match(/Kimi1倍率/g)).toHaveLength(1)

    const different = mountCard(makeItem({ name: 'Kimi1倍率（线路2）' }))
    expect(different.text()).toContain('Kimi1倍率（线路2）')
    expect(different.text().match(/Kimi1倍率/g)).toHaveLength(2)
  })
})
