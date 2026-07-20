import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

const root = process.cwd()

// 当前只守卫共享视觉入口，避免页面迁移完成前把历史存量误判为本任务回归。
const checkedFiles = ['src/style.css', 'tailwind.config.js']

const forbiddenPatterns = ['mesh-gradient', 'shadow-glow', 'shadow-primary']

const forbiddenPatternsRegex = [
  /bg-gradient-(?:to-[a-z]+)\s+from-(?:blue|indigo|violet|purple)-[0-9]+\s+to-(?:blue|indigo|violet|purple)-[0-9]+/,
  /(?:shadow|drop-shadow)-\[[^\]]*(?:0_0|glow)/,
]

describe('视觉主题源码守卫', () => {
  it('共享视觉入口不重新引入旧渐变和随机发光', () => {
    const offenders: string[] = []

    for (const file of checkedFiles) {
      const source = readFileSync(join(root, file), 'utf8')
      for (const pattern of forbiddenPatterns) {
        if (source.includes(pattern)) {
          offenders.push(`${file}: ${pattern}`)
        }
      }

      for (const pattern of forbiddenPatternsRegex) {
        if (pattern.test(source)) {
          offenders.push(`${file}: ${pattern}`)
        }
      }
    }

    expect(offenders).toEqual([])
  })
})
