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
  })

  it('scrolls one complete map period and respects reduced motion', () => {
    expect(source).toContain('width: calc(100% + 1800px)')
    expect(source).toContain('background-size: 1800px 900px')
    expect(source).toContain('animation: world-map-scroll 60s linear infinite')
    expect(source).toContain('translate3d(-1800px, 0, 0)')
    expect(source).toContain('@media (prefers-reduced-motion: reduce)')
    expect(source).toContain('animation: none')
  })
})
