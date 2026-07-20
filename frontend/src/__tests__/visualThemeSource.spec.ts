import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

const root = process.cwd()

// 当前只守卫共享基础块，页面级扫描会在对应迁移任务完成后扩大。
const checkedFiles = ['src/style.css', 'tailwind.config.js']
const homeSource = readFileSync(join(root, 'src/views/HomeView.vue'), 'utf8')
const authLayoutSource = readFileSync(join(root, 'src/components/layout/AuthLayout.vue'), 'utf8')
const userDashboardSource = readFileSync(join(root, 'src/views/user/DashboardView.vue'), 'utf8')
const userStatsSource = readFileSync(join(root, 'src/components/user/dashboard/UserDashboardStats.vue'), 'utf8')
const userChartsSource = readFileSync(join(root, 'src/components/user/dashboard/UserDashboardCharts.vue'), 'utf8')
const userQuickActionsSource = readFileSync(join(root, 'src/components/user/dashboard/UserDashboardQuickActions.vue'), 'utf8')
const userRecentUsageSource = readFileSync(join(root, 'src/components/user/dashboard/UserDashboardRecentUsage.vue'), 'utf8')
const adminDashboardSource = readFileSync(join(root, 'src/views/admin/DashboardView.vue'), 'utf8')
const tokenTrendSource = readFileSync(join(root, 'src/components/charts/TokenUsageTrend.vue'), 'utf8')
const modelDistributionSource = readFileSync(join(root, 'src/components/charts/ModelDistributionChart.vue'), 'utf8')

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

  it('公开页和认证页使用系统字、实色面板和 8px 终端预览', () => {
    expect(homeSource).not.toContain('font-semibold italic')
    expect(homeSource).not.toContain('linear-gradient(145deg')
    expect(homeSource).not.toContain('rotateX(1deg) rotateY(-1deg)')
    expect(homeSource).toContain('background: #111827;')
    expect(homeSource).toContain('border-radius: 8px;')
    expect(homeSource).toContain('shadow-card')

    expect(authLayoutSource).not.toContain('font-semibold italic')
    expect(authLayoutSource).not.toContain('shadow-soft')
    expect(authLayoutSource).not.toContain('card-glass')
    expect(authLayoutSource).toContain('rounded-md border border-gray-200 bg-white p-6 shadow-card')
  })

  it('用户端 Dashboard 删除卡片套卡片式动作和装饰性 hover 动效', () => {
    const userDashboardSurfaceSources = [
      userStatsSource,
      userChartsSource,
      userQuickActionsSource,
      userRecentUsageSource,
    ]

    expect(userDashboardSource).toContain('space-y-5')
    expect(userDashboardSource).toContain('gap-4 lg:grid-cols-3')
    expect(userStatsSource).toContain('grid grid-cols-2 gap-3 lg:grid-cols-4')
    expect(userChartsSource).toContain('card p-3')
    expect(userQuickActionsSource).toContain('rounded-md border border-gray-200 bg-white p-3')
    expect(userRecentUsageSource).toContain('rounded-md border border-gray-200 bg-white p-3')

    for (const source of userDashboardSurfaceSources) {
      expect(source).not.toContain('transition-all')
      expect(source).not.toContain('group-hover:scale-105')
      expect(source).not.toContain('rounded-xl')
      expect(source).not.toContain('shadow-soft')
    }
  })

  it('管理端 Dashboard 使用紧凑实色表面，不回退到营销式层级', () => {
    expect(adminDashboardSource).toContain('space-y-5')
    expect(adminDashboardSource).toContain('grid grid-cols-2 gap-3 lg:grid-cols-4')
    expect(adminDashboardSource.match(/card p-3/g)?.length ?? 0).toBeGreaterThanOrEqual(8)
    expect(tokenTrendSource).toContain('card p-3')
    expect(tokenTrendSource).toContain('h-44')
    expect(modelDistributionSource).toContain('card p-3')
    expect(modelDistributionSource).toContain('flex flex-col gap-4 sm:flex-row sm:items-center')

    for (const source of [adminDashboardSource, tokenTrendSource, modelDistributionSource]) {
      expect(source).not.toContain('transition-all')
      expect(source).not.toContain('shadow-soft')
      expect(source).not.toContain('rounded-2xl')
    }
  })
})
