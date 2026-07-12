import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const logoPath = resolve(dirname(fileURLToPath(import.meta.url)), '../../public/logo.png')
const logo = readFileSync(logoPath)
const pngSignature = Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a])

describe('默认品牌图标资源', () => {
  it('使用适合全站小尺寸展示的 256 像素正方形 PNG', () => {
    expect(logo.subarray(0, pngSignature.length)).toEqual(pngSignature)
    expect(logo.toString('ascii', 12, 16)).toBe('IHDR')
    expect(logo.readUInt32BE(16)).toBe(256)
    expect(logo.readUInt32BE(20)).toBe(256)
  })

  it('压缩后不超过 100 KiB', () => {
    expect(logo.byteLength).toBeLessThanOrEqual(100 * 1024)
  })
})
