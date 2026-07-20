import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'

const root = process.cwd()

// 只纳入已稳定的核心文件，避免尚未迁移页面把视觉守卫变成永久红灯。
const checkedFiles = [
  'src/style.css',
  'tailwind.config.js',
  'src/components/layout/AppLayout.vue',
  'src/components/layout/AppHeader.vue',
  'src/components/layout/AppSidebar.vue',
  'src/components/layout/AuthLayout.vue',
  'src/components/common/SubscriptionProgressMini.vue',
  'src/views/HomeView.vue',
  'src/views/user/RedeemView.vue',
  'src/views/admin/DashboardView.vue',
  'src/components/user/dashboard/UserDashboardStats.vue',
  'src/components/user/dashboard/UserDashboardCharts.vue',
  'src/components/user/dashboard/UserDashboardRecentUsage.vue',
  'src/components/charts/TokenUsageTrend.vue',
  'src/components/charts/ModelDistributionChart.vue',
  'src/components/charts/EndpointDistributionChart.vue',
  'src/utils/platformColors.ts',
  'src/utils/billingMode.ts',
  'src/i18n/locales/zh.ts',
  'src/i18n/locales/en.ts',
]

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
