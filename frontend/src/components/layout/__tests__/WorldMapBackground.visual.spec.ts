import { existsSync, readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const testDirectory = dirname(fileURLToPath(import.meta.url))
const componentPath = resolve(testDirectory, '../WorldMapBackground.vue')
const assetPath = resolve(testDirectory, '../../../assets/auth/world-map-dots.webp')
const source = readFileSync(componentPath, 'utf8')

describe('WorldMapBackground visual contract', () => {
  it('uses a local decorative world map texture', () => {
    expect(existsSync(assetPath)).toBe(true)
    expect(source).toContain("import worldMapDotsUrl from '@/assets/auth/world-map-dots.webp'")
    expect(source).toContain('aria-hidden="true"')
    expect(source).toContain('pointer-events-none')
    expect(source).toContain('v-for="copy in 3"')
    expect(source).toMatch(/<img[\s\S]*?aria-hidden="true"[\s\S]*?alt=""/)
    expect(source).toContain('alt=""')
    expect(source).toContain(':draggable="false"')
  })

  it('fills the container vertically and scrolls one complete map tile', () => {
    expect(source).toContain('height: 125%')
    expect(source).toContain('top: 50%')
    expect(source).toContain('animation: world-map-scroll 75s linear infinite')
    expect(source).toContain('translate3d(0, -50%, 0)')
    expect(source).toContain('translate3d(-33.333333%, -50%, 0)')
  })

  it('keeps the map vertically centered when reduced motion is enabled', () => {
    expect(source).toContain('@media (prefers-reduced-motion: reduce)')
    expect(source).toContain('animation: none')
    expect(source).toContain('transform: translate3d(0, -50%, 0)')
  })
})
