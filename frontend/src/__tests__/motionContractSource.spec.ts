import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

const root = process.cwd()
const styleSource = readFileSync(join(root, 'src/style.css'), 'utf8')
const overlaySources = [
  'components/layout/AppHeader.vue',
  'components/common/DateRangePicker.vue',
  'components/common/Select.vue',
  'components/common/ProxySelector.vue',
  'components/common/LocaleSwitcher.vue',
  'components/common/VersionBadge.vue',
  'components/common/SubscriptionProgressMini.vue',
  'components/account/AccountGroupsCell.vue',
  'components/common/HelpTooltip.vue',
].map((path) => ({ path, source: readFileSync(join(root, 'src', path), 'utf8') }))

const sharedBlock = (selector: string) => {
  const match = styleSource.match(new RegExp(`\\.${selector}\\s*\\{([\\s\\S]*?)\\n\\s*\\}`))

  expect(match, `缺少 .${selector} 共享样式块`).not.toBeNull()
  return match?.[1] ?? ''
}

describe('共享动效契约源码守卫', () => {
  it('定义 Material Relay easing 与 duration token', () => {
    expect(styleSource).toContain('--ease-out: cubic-bezier(0.23, 1, 0.32, 1)')
    expect(styleSource).toContain('--ease-in-out: cubic-bezier(0.77, 0, 0.175, 1)')
    expect(styleSource).toContain('--ease-drawer: cubic-bezier(0.32, 0.72, 0, 1)')
    expect(styleSource).toContain('--duration-press: 160ms')
    expect(styleSource).toContain('--duration-popover: 180ms')
    expect(styleSource).toContain('--duration-overlay-enter: 220ms')
    expect(styleSource).toContain('--duration-overlay-exit: 160ms')
    expect(styleSource).toContain('--duration-drawer: 280ms')
  })

  it('共享表面只声明明确属性并完整引用 motion token', () => {
    for (const selector of ['btn', 'input', 'card', 'glass-card']) {
      const block = sharedBlock(selector)

      expect(block).not.toMatch(/transition\s*:\s*all\b/)
      expect(block).not.toMatch(/transition-property\s*:\s*all\b/)
      expect(block).not.toMatch(/@apply[^;\n]*\btransition-all\b/)
      expect(block).not.toMatch(/@apply[^;\n]*\stransition(?:\s|;)/)
      expect(block).toMatch(/\btransition\s*:/)
      expect(block).toMatch(/var\(--duration-[a-z-]+\)/)
      expect(block).toContain('var(--ease-out)')
    }
  })

  it('目标浮层组件禁止不可中断或错误方向的动效声明', () => {
    for (const { path, source } of overlaySources) {
      expect(source, `${path} 不得使用 transition: all`).not.toMatch(/transition\s*:\s*all\b/)
      expect(source, `${path} 不得使用 Tailwind transition-all`).not.toMatch(/\btransition-all\b/)
      expect(source, `${path} 不得在浮层中使用 ease-in`).not.toMatch(/\bease-in\b(?!-out)/)
      expect(source, `${path} 不得使用 animate-scale-in`).not.toContain('animate-scale-in')
    }
  })

  it('定义可复用的 origin-aware popover 与 overlay contract', () => {
    const popoverBlock = styleSource.match(/\.popover-motion-enter-active\s*\{([\s\S]*?)\n\s*\}/)?.[1] ?? ''
    const overlayBlock = styleSource.match(/\.overlay-motion-enter-active\s*\{([\s\S]*?)\n\s*\}/)?.[1] ?? ''

    expect(popoverBlock).toMatch(/transform/)
    expect(popoverBlock).toMatch(/opacity/)
    expect(popoverBlock).toContain('var(--duration-popover)')
    expect(popoverBlock).toContain('var(--ease-out)')
    expect(styleSource).toContain('transform-origin: var(--popover-origin, top right)')
    expect(overlayBlock).toContain('var(--duration-overlay-enter)')
    expect(styleSource).toContain('var(--duration-overlay-exit)')
    expect(styleSource).toContain('transform-origin: center')
  })
})
