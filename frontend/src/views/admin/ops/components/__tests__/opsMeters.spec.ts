import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

const root = process.cwd()
const concurrencySource = readFileSync(
  join(root, 'src/views/admin/ops/components/OpsConcurrencyCard.vue'),
  'utf8'
)
const dashboardHeaderSource = readFileSync(
  join(root, 'src/views/admin/ops/components/OpsDashboardHeader.vue'),
  'utf8'
)

describe('Ops 实时 meter 动效契约', () => {
  it('并发卡三处负载条使用无补间的 GPU meter', () => {
    expect(concurrencySource.match(/\bmeter-fill meter-fill--realtime\b/g) ?? []).toHaveLength(3)
    expect(concurrencySource.match(/:style="getLoadBarStyle\(/g) ?? []).toHaveLength(3)
    expect(concurrencySource).toContain("return { '--meter-value': meterScale(loadPct, 100) }")
    expect(concurrencySource).toContain("import { meterScale } from '@/utils/meter'")
    expect(concurrencySource).toContain('meterScale(loadPct, 100)')
    expect(concurrencySource).not.toMatch(/:style="\{\s*width:/)
    expect(concurrencySource).not.toContain('transition-all duration-300')
  })

  it('健康环和实时状态不使用追赶或常驻动画', () => {
    expect(dashboardHeaderSource).not.toContain('transition-all duration-1000')
    expect(dashboardHeaderSource).not.toContain('animate-ping')
    expect(dashboardHeaderSource).toContain("'animate-spin': loading")
    expect(dashboardHeaderSource).toContain('transition-colors hover:bg-white/60')
  })

  it('SLA 使用无补间的 GPU meter 并保留阈值颜色', () => {
    const slaStart = dashboardHeaderSource.indexOf('<!-- Card 2: SLA -->')
    const slaEnd = dashboardHeaderSource.indexOf('<!-- Card 4: Request Duration -->')
    const slaBlock = dashboardHeaderSource.slice(slaStart, slaEnd)

    expect(slaStart).toBeGreaterThanOrEqual(0)
    expect(slaEnd).toBeGreaterThan(slaStart)
    expect(dashboardHeaderSource).toContain("import { meterScale } from '@/utils/meter'")
    expect(slaBlock).toContain('meter-fill meter-fill--realtime')
    expect(slaBlock).toContain('--meter-value')
    expect(slaBlock).toContain('meterScale(Math.max((slaPercent ?? 0) - 90, 0), 10)')
    expect(slaBlock).not.toContain('transition-all')
    expect(slaBlock).not.toMatch(/width\s*:/)
    expect(slaBlock).toContain("getSLAThresholdLevel(slaPercent) === 'critical'")
    expect(slaBlock).toContain("getSLAThresholdLevel(slaPercent) === 'warning'")
  })
})
