import { describe, expect, it } from 'vitest'
import { Color } from 'three'

import { BACK, CARD_H, CARD_THICKNESS_M, CARD_W, EDGE, FRONT, cardGeometry, uvToFacePixel } from '../cardStage'
import { CARD_HEIGHT, CARD_WIDTH, regionContains } from '../redeemCardModel'

// 取某一组材质的全部顶点下标。
function groupVertices(geometry: ReturnType<typeof cardGeometry>, materialIndex: number): number[] {
  const out: number[] = []
  for (const group of geometry.groups) {
    if (group.materialIndex !== materialIndex) continue
    for (let i = group.start; i < group.start + group.count; i++) out.push(i)
  }
  return out
}

describe('cardStage 卡片几何（照 3D 模板）', () => {
  const geometry = cardGeometry(CARD_THICKNESS_M)
  const pos = geometry.attributes.position
  const uv = geometry.attributes.uv
  const box = geometry.boundingBox!

  it('按 CR80 尺寸挤出，厚度 0.6mm，以原点为中心', () => {
    expect(CARD_THICKNESS_M).toBeCloseTo(0.0006, 9)
    expect(box.max.x - box.min.x).toBeCloseTo(CARD_W, 7)
    expect(box.max.y - box.min.y).toBeCloseTo(CARD_H, 7)
    expect(box.max.z - box.min.z).toBeCloseTo(0.0006, 9)
    expect(box.max.z + box.min.z).toBeCloseTo(0, 9)
  })

  it('正面、背面、侧边分成三组材质，两个端面分别贴在 +z、−z', () => {
    expect(new Set(geometry.groups.map((group) => group.materialIndex))).toEqual(new Set([FRONT, BACK, EDGE]))
    const front = groupVertices(geometry, FRONT)
    const back = groupVertices(geometry, BACK)
    expect(front.length).toBeGreaterThan(0)
    expect(back.length).toBeGreaterThan(0)
    expect(front.every((i) => Math.abs(pos.getZ(i) - box.max.z) < 1e-9)).toBe(true)
    expect(back.every((i) => Math.abs(pos.getZ(i) - box.min.z) < 1e-9)).toBe(true)
    // 侧边（含倒角）不能有整片落在端面上的三角形，否则会挡住贴图。
    const edge = groupVertices(geometry, EDGE)
    for (let i = 0; i < edge.length; i += 3) {
      const zs = [pos.getZ(edge[i]), pos.getZ(edge[i + 1]), pos.getZ(edge[i + 2])]
      expect(zs.every((z) => Math.abs(z - box.max.z) < 1e-9) || zs.every((z) => Math.abs(z - box.min.z) < 1e-9)).toBe(false)
    }
  })

  it('两面贴图都不镜像：正面 u 随 x 增大，背面从 −z 看过去左右翻回来', () => {
    const front = groupVertices(geometry, FRONT)
    const back = groupVertices(geometry, BACK)
    const byX = (list: number[]) => [...list].sort((a, b) => pos.getX(a) - pos.getX(b))
    const frontSorted = byX(front)
    const backSorted = byX(back)
    expect(uv.getX(frontSorted[0])).toBeCloseTo(0, 2)
    expect(uv.getX(frontSorted[frontSorted.length - 1])).toBeCloseTo(1, 2)
    // 从背面看，世界坐标 x 最大的那一侧在观察者左手边，对应贴图左边 u = 0。
    expect(uv.getX(backSorted[0])).toBeCloseTo(1, 2)
    expect(uv.getX(backSorted[backSorted.length - 1])).toBeCloseTo(0, 2)
  })

  it('侧边顶点色是 青 → 紫 → 粉 的对角渐变：左上接起点色，右下接终点色', () => {
    const colors = geometry.attributes.color
    const edge = groupVertices(geometry, EDGE)
    const nearest = (x: number, y: number) =>
      edge.reduce((best, i) => (Math.hypot(pos.getX(i) - x, pos.getY(i) - y) < Math.hypot(pos.getX(best) - x, pos.getY(best) - y) ? i : best))
    const topLeft = nearest(-CARD_W / 2, CARD_H / 2)
    const bottomRight = nearest(CARD_W / 2, -CARD_H / 2)
    const start = new Color('#2b8a99')
    const end = new Color('#8e2f72')
    for (const [i, expected] of [[topLeft, start], [bottomRight, end]] as const) {
      expect(colors.getX(i)).toBeCloseTo(expected.r, 1)
      expect(colors.getY(i)).toBeCloseTo(expected.g, 1)
      expect(colors.getZ(i)).toBeCloseTo(expected.b, 1)
    }
  })

  it('端面 UV 换算成卡面设计像素，左上角为原点', () => {
    expect(uvToFacePixel({ x: 0, y: 1 })).toEqual({ x: 0, y: 0 })
    expect(uvToFacePixel({ x: 1, y: 0 })).toEqual({ x: CARD_WIDTH, y: CARD_HEIGHT })
    expect(uvToFacePixel({ x: 0.5, y: 0.5 })).toEqual({ x: CARD_WIDTH / 2, y: CARD_HEIGHT / 2 })
  })

  it('轻点落在兑换码面板里才算点中', () => {
    const panel = { x: 88, y: 80, width: 1536, height: 260, radius: 28 }
    expect(regionContains(panel, 88, 80)).toBe(true)
    expect(regionContains(panel, 856, 200)).toBe(true)
    expect(regionContains(panel, 87, 200)).toBe(false)
    expect(regionContains(panel, 856, 341)).toBe(false)
  })
})
