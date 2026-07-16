import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const currentDir = dirname(fileURLToPath(import.meta.url))
const viewPath = resolve(currentDir, '../GroupsView.vue')

describe('GroupsView 图片配置契约', () => {
  it('只保留图片生成能力开关', () => {
    const source = readFileSync(viewPath, 'utf8')

    expect(source).toContain('allow_image_generation')
    for (const field of [
      'image_rate_independent',
      'image_rate_multiplier',
      'image_price_1k',
      'image_price_2k',
      'image_price_4k',
    ]) {
      expect(source).not.toContain(field)
    }
  })
})
