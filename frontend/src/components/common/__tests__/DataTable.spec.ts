import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import DataTable from '../DataTable.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

const matchMediaListeners = new Set<(event: MediaQueryListEvent) => void>()

function installDesktopViewport() {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: query.includes('min-width: 768px'),
    media: query,
    onchange: null,
    addEventListener: (_type: string, listener: (event: MediaQueryListEvent) => void) => {
      matchMediaListeners.add(listener)
    },
    removeEventListener: (_type: string, listener: (event: MediaQueryListEvent) => void) => {
      matchMediaListeners.delete(listener)
    },
    addListener: (listener: (event: MediaQueryListEvent) => void) => {
      matchMediaListeners.add(listener)
    },
    removeListener: (listener: (event: MediaQueryListEvent) => void) => {
      matchMediaListeners.delete(listener)
    },
    dispatchEvent: () => true,
  }))

  class ResizeObserverStub {
    observe() { return undefined }
    unobserve() { return undefined }
    disconnect() { return undefined }
  }

  vi.stubGlobal('ResizeObserver', ResizeObserverStub)
}

describe('DataTable 排序箭头动效', () => {
  beforeEach(() => {
    installDesktopViewport()
  })

  afterEach(() => {
    matchMediaListeners.clear()
    vi.unstubAllGlobals()
  })

  it('切换排序时保持同一个 SVG 节点，只改变 transform', async () => {
    const wrapper = mount(DataTable, {
      props: {
        columns: [
          { key: 'name', label: 'Name', sortable: true },
        ],
        data: [
          { id: 1, name: 'Beta' },
          { id: 2, name: 'Alpha' },
        ],
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    await nextTick()

    const header = wrapper.get('th')
    const arrowBefore = wrapper.get('.data-table-sort-arrow').element

    expect(wrapper.findAll('.data-table-sort-arrow')).toHaveLength(1)
    expect(wrapper.get('.data-table-sort-arrow').classes()).toContain('opacity-50')

    await header.trigger('click')
    await nextTick()

    const arrowAfterFirstClick = wrapper.get('.data-table-sort-arrow').element
    expect(arrowAfterFirstClick).toBe(arrowBefore)
    expect(wrapper.get('.data-table-sort-arrow').classes()).toContain('rotate-180')
    expect(wrapper.get('.data-table-sort-arrow').classes()).not.toContain('opacity-50')

    await header.trigger('click')
    await nextTick()

    const arrowAfterSecondClick = wrapper.get('.data-table-sort-arrow').element
    expect(arrowAfterSecondClick).toBe(arrowBefore)
    expect(wrapper.get('.data-table-sort-arrow').classes()).not.toContain('rotate-180')
    expect(wrapper.get('.data-table-sort-arrow').classes()).not.toContain('opacity-50')
  })
})
