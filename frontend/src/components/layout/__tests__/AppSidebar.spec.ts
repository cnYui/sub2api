import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AppSidebar.vue')
const componentSource = readFileSync(componentPath, 'utf8')
const layoutPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AppLayout.vue')
const layoutSource = readFileSync(layoutPath, 'utf8')
const headerPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AppHeader.vue')
const headerSource = readFileSync(headerPath, 'utf8')
const stylePath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../style.css')
const styleSource = readFileSync(stylePath, 'utf8')

describe('AppSidebar custom SVG styles', () => {
  it('does not override uploaded SVG fill or stroke colors', () => {
    expect(componentSource).toContain('.sidebar-svg-icon {')
    expect(componentSource).toContain('color: currentColor;')
    expect(componentSource).toContain('display: block;')
    expect(componentSource).not.toContain('stroke: currentColor;')
    expect(componentSource).not.toContain('fill: none;')
  })
})

describe('AppSidebar header styles', () => {
  it('does not clip the version badge dropdown', () => {
    const sidebarHeaderBlockMatch = styleSource.match(/\.sidebar-header\s*\{[\s\S]*?\n {2}\}/)
    const sidebarBrandBlockMatch = componentSource.match(/\.sidebar-brand\s*\{[\s\S]*?\n\}/)

    expect(sidebarHeaderBlockMatch).not.toBeNull()
    expect(sidebarBrandBlockMatch).not.toBeNull()
    expect(sidebarHeaderBlockMatch?.[0]).not.toContain('@apply overflow-hidden;')
    expect(sidebarBrandBlockMatch?.[0]).not.toContain('overflow: hidden;')
  })
})

describe('AppSidebar user usage guide nav', () => {
  it('adds usage guide only to the regular user menu', () => {
    const navFunctionMatch = componentSource.match(/function buildSelfNavItems\(withDashboard: boolean\): NavItem\[] \{[\s\S]*?\n\}/)

    expect(navFunctionMatch).not.toBeNull()
    const navFunction = navFunctionMatch?.[0] ?? ''
    expect(navFunction).toContain("if (withDashboard) {\n    items.push({ path: '/usage-guide', label: t('nav.usageGuide')")
    expect(navFunction).not.toContain("items.push(\n    { path: '/usage-guide'")
  })
})

describe('AppSidebar mobile overlay stacking', () => {
  it('keeps the mobile backdrop above the header and below the sidebar', () => {
    expect(headerSource).toContain('z-30')
    expect(styleSource).toContain('left-0 z-40 flex')
    expect(componentSource).toContain('sidebar-backdrop')
    expect(componentSource).toContain('z-[35]')
    expect(componentSource).not.toContain('class="fixed inset-0 z-30 bg-black/40 lg:hidden"')
  })
})

describe('App shell motion contracts', () => {
  it('keeps content geometry and sidebar header geometry static', () => {
    expect(layoutSource).not.toContain('transition-all')

    const sidebarBlock = styleSource.match(/\.sidebar\s*\{[\s\S]*?\n {2}\}/)?.[0] ?? ''
    const sidebarHeaderBlock = styleSource.match(/\.sidebar-header\s*\{[\s\S]*?\n {2}\}/)?.[0] ?? ''

    expect(sidebarBlock).not.toContain('transition-property: width')
    expect(sidebarBlock).not.toContain('transition: width')
    expect(sidebarHeaderBlock).not.toMatch(/transition(?:-property)?\s*:[\s\S]*(padding|gap)/)
  })

  it('only animates sidebar labels and branding opacity/transform', () => {
    const brandBlock = componentSource.match(/\.sidebar-brand\s*\{[\s\S]*?\n\}/)?.[0] ?? ''
    const labelBlock = componentSource.match(/\.sidebar-label\s*\{[\s\S]*?\n\}/)?.[0] ?? ''

    expect(brandBlock).not.toMatch(/transition(?:-property)?\s*:[\s\S]*max-width/)
    expect(labelBlock).not.toMatch(/transition(?:-property)?\s*:[\s\S]*max-width/)
  })

  it('uses the global popover transition as the only AppHeader dropdown motion source', () => {
    const dropdownBlock = styleSource.match(/\.dropdown\s*\{[\s\S]*?\n {2}\}/)?.[0] ?? ''
    const enterBlock = styleSource.match(/\.popover-motion-enter-active\s*\{[\s\S]*?\n {2}\}/)?.[0] ?? ''

    expect(headerSource).toContain('<transition name="popover-motion">')
    expect(headerSource).toContain('class="dropdown popover-motion')
    expect(dropdownBlock).toContain('transform-origin: var(--popover-origin, top right);')
    expect(dropdownBlock).not.toContain('animation:')
    expect(enterBlock).toContain('transform var(--duration-popover) var(--ease-out)')
    expect(enterBlock).toContain('opacity var(--duration-popover) var(--ease-out)')
  })

  it('uses separate enter and leave durations for the mobile backdrop', () => {
    const enterBlock = componentSource.match(/\.sidebar-backdrop-enter-active\s*\{[\s\S]*?\}/)?.[0] ?? ''
    const leaveBlock = componentSource.match(/\.sidebar-backdrop-leave-active\s*\{[\s\S]*?\}/)?.[0] ?? ''

    expect(enterBlock).toContain('--duration-overlay-enter')
    expect(leaveBlock).toContain('--duration-overlay-exit')
  })
})

describe('AppSidebar user channel monitor nav', () => {
  it('hides the user-facing monitor entry while keeping admin monitor management', () => {
    const navFunctionMatch = componentSource.match(/function buildSelfNavItems\(withDashboard: boolean\): NavItem\[] \{[\s\S]*?\n\}/)

    expect(navFunctionMatch).not.toBeNull()
    const navFunction = navFunctionMatch?.[0] ?? ''

    expect(navFunction).not.toContain("path: '/monitor'")
    expect(componentSource).toContain("path: '/admin/channels/monitor'")
  })
})
