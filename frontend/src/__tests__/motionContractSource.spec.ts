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
  'components/common/AnnouncementBell.vue',
  'components/common/AnnouncementPopup.vue',
  'components/account/AccountGroupsCell.vue',
  'components/common/HelpTooltip.vue',
  'components/common/Toast.vue',
].map((path) => ({ path, source: readFileSync(join(root, 'src', path), 'utf8') }))
const keysViewSource = readFileSync(join(root, 'src/views/user/KeysView.vue'), 'utf8')
const subscriptionUsageCardSource = readFileSync(
  join(root, 'src/components/user/SubscriptionUsageCard.vue'),
  'utf8'
)
const adminSubscriptionsSource = readFileSync(
  join(root, 'src/views/admin/SubscriptionsView.vue'),
  'utf8'
)
const adminRiskControlSource = readFileSync(
  join(root, 'src/views/admin/RiskControlView.vue'),
  'utf8'
)
const adminSettingsSource = readFileSync(
  join(root, 'src/views/admin/SettingsView.vue'),
  'utf8'
)

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

  it('Toast 与 meter 只使用 GPU 属性并支持 reduced-motion', () => {
    const toastSource = readFileSync(join(root, 'src/components/common/Toast.vue'), 'utf8')
    const toastProgressBlock = sharedBlock('toast-progress')
    const toastKeyframes = styleSource.match(/@keyframes\s+toast-progress-shrink\s*\{([\s\S]*?)\n\s*\}\s*\}/)?.[1] ?? ''
    const toastMotionEnterBlock = sharedBlock('toast-motion-enter-active')
    const toastMotionLeaveBlock = sharedBlock('toast-motion-leave-active')
    const meterBlock = styleSource.match(/\.meter-fill\s*\{([\s\S]*?)\n\s*\}/)?.[1] ?? ''

    expect(toastSource).toContain('name="toast-motion"')
    expect(toastMotionEnterBlock).toContain('transform')
    expect(toastMotionEnterBlock).toContain('opacity')
    expect(toastMotionEnterBlock).toContain('var(--duration-overlay-enter)')
    expect(toastMotionEnterBlock).toContain('var(--ease-out)')
    expect(toastMotionLeaveBlock).toContain('transform')
    expect(toastMotionLeaveBlock).toContain('opacity')
    expect(toastMotionLeaveBlock).toContain('var(--duration-overlay-exit)')
    expect(toastMotionLeaveBlock).toContain('var(--ease-out)')
    expect(toastMotionLeaveBlock).toContain('position: absolute')
    expect(toastMotionLeaveBlock).toContain('right: 0')
    expect(toastMotionLeaveBlock).toContain('width: 100%')
    expect(toastSource).toContain('w-[min(28rem,calc(100vw-2rem))]')
    expect(toastSource).toMatch(
      /class="pointer-events-none[^\"]*\bflex\b[^\"]*\bflex-col\b[^\"]*\bgap-3\b/
    )
    expect(toastSource).toContain("'pointer-events-auto w-full")
    expect(toastProgressBlock).toContain('transform-origin')
    expect(toastProgressBlock).toContain('transform: scaleX(1)')
    expect(toastProgressBlock).not.toContain('width:')
    expect(toastKeyframes).toContain('transform: scaleX(1)')
    expect(toastKeyframes).toContain('transform: scaleX(0)')
    expect(toastKeyframes).not.toContain('width:')
    expect(styleSource).toContain('@media (prefers-reduced-motion: reduce)')
    expect(styleSource).toMatch(/\.toast-motion-enter-active[\s\S]*?transition-property:\s*opacity/)
    expect(meterBlock).toContain('transform-origin: left')
    expect(meterBlock).toContain('transform: scaleX(var(--meter-value, 0))')
    expect(meterBlock).toContain('transition: transform var(--duration-popover) var(--ease-out)')
    expect(meterBlock).toContain('width: 100%')
    expect(meterBlock).toContain('height: 100%')
    expect(meterBlock).not.toContain('position: absolute')
    expect(meterBlock).not.toContain('inset:')
    expect(meterBlock).not.toContain('transition-all')
    expect(styleSource).toMatch(/\.meter-fill--realtime\s*\{[\s\S]*?transition:\s*none/)

    const toastContractIndex = styleSource.indexOf('/* ============ Toast 动效契约 ============ */')
    const reducedMotionIndex = styleSource.indexOf('@media (prefers-reduced-motion: reduce)', toastContractIndex)
    const nextSectionIndex = styleSource.indexOf('/* ============ 侧边栏 ============ */', reducedMotionIndex)
    const reducedMotionBlock = styleSource.slice(reducedMotionIndex, nextSectionIndex)

    expect(reducedMotionIndex).toBeGreaterThan(styleSource.indexOf('.meter-fill {', toastContractIndex))
    expect(reducedMotionBlock).toContain('.toast-motion-enter-active')
    expect(reducedMotionBlock).toContain('transition-property: opacity')
    expect(reducedMotionBlock).toContain('.toast-motion-move')
    expect(reducedMotionBlock).toContain('.toast-progress')
    expect(reducedMotionBlock).toContain('.meter-fill')
  })

  it('公告浮层统一复用 overlay motion contract', () => {
    const announcementBellSource = readFileSync(
      join(root, 'src/components/common/AnnouncementBell.vue'),
      'utf8'
    )
    const announcementPopupSource = readFileSync(
      join(root, 'src/components/common/AnnouncementPopup.vue'),
      'utf8'
    )

    expect(announcementBellSource.match(/<Transition name="overlay-motion">/g)).toHaveLength(2)
    expect(announcementBellSource.match(/class="[^"]*overlay-motion[^"]*"/g)).toHaveLength(4)
    expect(announcementBellSource.match(/\boverlay-motion-content\b/g)).toHaveLength(2)
    expect(announcementPopupSource).toContain('<Transition name="overlay-motion">')
    expect(announcementPopupSource).toMatch(/class="[^"]*overlay-motion[^"]*"/)
    expect(announcementPopupSource).toContain('overlay-motion-content')

    for (const source of [announcementBellSource, announcementPopupSource]) {
      expect(source).not.toMatch(/\banimate-ping\b/)
      expect(source).not.toMatch(/\bhover:scale-105\b/)
      expect(source).not.toMatch(/\b(?:modal|popup)-fade\b/)
    }
  })

  it('用户端配额与限流进度条只通过 meter-fill 的 GPU 变换更新', () => {
    expect(keysViewSource.match(/\bmeter-fill\b/g) ?? []).toHaveLength(7)
    expect(keysViewSource.match(/--meter-value/g) ?? []).toHaveLength(7)
    expect(keysViewSource.match(/:style="\{\s*width:/g) ?? []).toHaveLength(0)
    expect(keysViewSource.match(/h-full rounded-full transition-all/g) ?? []).toHaveLength(0)
    expect(keysViewSource).toContain('--meter-value')
  })

  it('订阅用量卡使用共享 meter-fill，不再绑定动态 width', () => {
    expect(subscriptionUsageCardSource).toContain('meter-fill')
    expect(subscriptionUsageCardSource).toContain('--meter-value')
    expect(subscriptionUsageCardSource).not.toContain('transition-all')
    expect(subscriptionUsageCardSource).not.toMatch(/:style="\{\s*width:/)
  })

  it('管理端订阅用量条使用共享 meter-fill 和安全比例', () => {
    expect(adminSubscriptionsSource.match(/\bmeter-fill\b/g) ?? []).toHaveLength(3)
    expect(adminSubscriptionsSource.match(/--meter-value/g) ?? []).toHaveLength(3)
    expect(adminSubscriptionsSource).toContain('getProgressScale')
    expect(adminSubscriptionsSource).not.toContain('getProgressWidth')
    expect(adminSubscriptionsSource).not.toMatch(/:style="\{\s*width:/)
    expect(adminSubscriptionsSource.match(/h-1\.5 flex-1 overflow-hidden/g) ?? []).toHaveLength(3)
  })

  it('管理端风控实时与评分条按 GPU meter 更新', () => {
    expect(adminRiskControlSource.match(/\bmeter-fill\b/g) ?? []).toHaveLength(6)
    expect(adminRiskControlSource.match(/--meter-value/g) ?? []).toHaveLength(4)
    expect(adminRiskControlSource.match(/meter-fill--realtime/g) ?? []).toHaveLength(2)
    expect(adminRiskControlSource).toContain('meterScale')
    expect(adminRiskControlSource).not.toContain('preBlockAPIKeyLoadWidth')
    expect(adminRiskControlSource).not.toContain('queueUsageStyle')
    expect(adminRiskControlSource).not.toContain('percentWidth')
    expect(adminRiskControlSource).not.toMatch(/:style="\{\s*width:/)
    expect(adminRiskControlSource).not.toContain('transition-all duration-300')
  })

  it('管理端 Settings 仅迁移网页搜索配额条', () => {
    const start = adminSettingsSource.indexOf('<!-- Usage display -->')
    const end = adminSettingsSource.indexOf('<!-- Proxy + Test on same row -->')
    const quotaBlock = adminSettingsSource.slice(start, end)

    expect(start).toBeGreaterThanOrEqual(0)
    expect(end).toBeGreaterThan(start)
    expect(quotaBlock).toContain('meter-fill')
    expect(quotaBlock).toContain('--meter-value')
    expect(quotaBlock).toContain('overflow-hidden')
    expect(quotaBlock).not.toContain('transition-all')
    expect(quotaBlock).not.toMatch(/width\s*:/)
    expect(adminSettingsSource).toContain("'rounded-lg border px-3 py-1.5 text-sm font-medium transition-all'")
  })
})
