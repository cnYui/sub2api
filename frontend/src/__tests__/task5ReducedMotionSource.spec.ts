import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

const root = process.cwd()
const read = (path: string) => readFileSync(join(root, path), 'utf8')

const styleSource = read('src/style.css')
const accountsSource = read('src/views/admin/AccountsView.vue')
const homeSource = read('src/views/HomeView.vue')
const worldMapSource = read('src/components/layout/WorldMapBackground.vue')
const spinnerSource = read('src/components/common/LoadingSpinner.vue')
const opsDashboardHeaderSource = read('src/views/admin/ops/components/OpsDashboardHeader.vue')

describe('Task 5 reduced-motion 与静态背景源码契约', () => {
  it('在最终 reduced-motion 规则中关闭平滑滚动，但不把所有动效压成 1ms', () => {
    const reduceBlocks = [...styleSource.matchAll(/@media\s*\(prefers-reduced-motion:\s*reduce\)\s*\{([\s\S]*?)\n\s*\}/g)]
      .map((match) => match[0])

    expect(reduceBlocks.length).toBeGreaterThan(0)
    expect(styleSource).toMatch(
      /@media\s*\(prefers-reduced-motion:\s*reduce\)[\s\S]*?html\s*\{[\s\S]*?scroll-behavior:\s*auto\s*;/
    )
    expect(styleSource).not.toMatch(/transition-duration:\s*1ms/)
  })

  it('自动刷新仅在真实请求期间旋转图标', () => {
    expect(accountsSource).toContain(":class=\"[autoRefreshFetching ? 'animate-spin' : '']\"")
    expect(accountsSource).not.toContain("autoRefreshEnabled ? 'animate-spin' : ''")
  })

  it('首页终端采用短 stagger、桌面 hover 媒体条件和 reduced-motion 静态可读规则', () => {
    expect(homeSource).toContain('animation: line-appear 180ms var(--ease-out) forwards;')
    expect(homeSource).toContain('animation-delay: 0ms;')
    expect(homeSource).toContain('animation-delay: 40ms;')
    expect(homeSource).toContain('animation-delay: 80ms;')
    expect(homeSource).toContain('animation-delay: 120ms;')
    expect(homeSource).not.toContain('animation-delay: 2.5s;')
    expect(homeSource).toMatch(/@media\s*\(hover:\s*hover\)\s+and\s+\(pointer:\s*fine\)[\s\S]*?\.terminal-window:hover\s*\{/)
    expect(homeSource).toMatch(/@media\s*\(prefers-reduced-motion:\s*reduce\)[\s\S]*?\.code-line\s*\{[\s\S]*?animation:\s*none;[\s\S]*?opacity:\s*1;/)
    expect(homeSource).toMatch(/@media\s*\(prefers-reduced-motion:\s*reduce\)[\s\S]*?\.terminal-window\s*\{[\s\S]*?transform:\s*none;/)
  })

  it('世界地图背景保持本地纹理和垂直居中，但不再循环或声明 will-change', () => {
    expect(worldMapSource).toContain("import worldMapDotsUrl from '@/assets/auth/world-map-dots.webp'")
    expect(worldMapSource).toContain('height: 125%')
    expect(worldMapSource).toContain('top: 50%')
    expect(worldMapSource).toContain('transform: translate3d(0, -50%, 0)')
    expect(worldMapSource).not.toContain('world-map-scroll 75s')
    expect(worldMapSource).not.toContain('@keyframes world-map-scroll')
    expect(worldMapSource).not.toContain('will-change')
    expect(worldMapSource).toMatch(/\.world-map-track\s*\{[\s\S]*?opacity:\s*0\.[0-9]+;/)
    expect(worldMapSource).toMatch(/@media\s*\(prefers-reduced-motion:\s*reduce\)[\s\S]*?\.world-map-track\s*\{[\s\S]*?transform:\s*translate3d\(0, -50%, 0\);/)
  })

  it('加载旋转器正常旋转，reduced-motion 下保持静态且保留可访问性语义', () => {
    expect(spinnerSource).toContain('role="status"')
    expect(spinnerSource).toContain(':aria-label="t(\'common.loading\')"')
    expect(spinnerSource).toContain('animation: spin 0.75s linear infinite;')
    expect(spinnerSource).toMatch(/@media\s*\(prefers-reduced-motion:\s*reduce\)[\s\S]*?\.spinner\s*\{[\s\S]*?animation:\s*none;/)
  })

  it('Ops 仪表盘心跳线保持静态路径，不使用持续 SVG 动画', () => {
    expect(opsDashboardHeaderSource).toContain('d="M0 16 Q 20 16, 40 16')
    expect(opsDashboardHeaderSource).not.toContain('<animate')
    expect(opsDashboardHeaderSource).not.toContain('repeatCount="indefinite"')
  })
})
