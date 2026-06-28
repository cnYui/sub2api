import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolveDocumentTitle, resolveRouteDocumentTitle } from '@/router/title'

describe('resolveDocumentTitle', () => {
  it('路由存在标题时，使用“路由标题 - 站点名”格式', () => {
    expect(resolveDocumentTitle('Usage Records', 'My Site')).toBe('Usage Records - My Site')
  })

  it('路由无标题时，回退到站点名', () => {
    expect(resolveDocumentTitle(undefined, 'My Site')).toBe('My Site')
  })

  it('站点名为空时，回退默认站点名', () => {
    expect(resolveDocumentTitle('Dashboard', '')).toBe('Dashboard - 天才程序员小站')
    expect(resolveDocumentTitle(undefined, '   ')).toBe('天才程序员小站')
  })

  it('站点名变更时仅影响后续路由标题计算', () => {
    const before = resolveDocumentTitle('Admin Dashboard', 'Alpha')
    const after = resolveDocumentTitle('Admin Dashboard', 'Beta')

    expect(before).toBe('Admin Dashboard - Alpha')
    expect(after).toBe('Admin Dashboard - Beta')
  })
})

describe('resolveRouteDocumentTitle', () => {
  it('自定义页面菜单加载后，使用菜单名称作为标题', () => {
    const route = {
      name: 'CustomPage',
      params: { id: 'scheduler' },
      meta: {
        title: 'Custom Page'
      }
    }

    expect(resolveRouteDocumentTitle(route, 'EzouAPI')).toBe('Custom Page - EzouAPI')
    expect(resolveRouteDocumentTitle(route, 'EzouAPI', [
      {
        id: 'scheduler',
        label: '账号调度器',
        icon_svg: '',
        url: 'https://example.com',
        visibility: 'admin',
        sort_order: 0
      }
    ])).toBe('账号调度器 - EzouAPI')
  })
})

describe('/purchase route header', () => {
  it('登录后的购买页只展示标题，不展示标题下方说明', () => {
    const routerSource = readFileSync('src/router/index.ts', 'utf8')
    const purchaseRoute = routerSource.match(/path: '\/purchase'[\s\S]*?path: '\/orders'/)?.[0] ?? ''

    expect(purchaseRoute).toContain("titleKey: 'nav.buySubscription'")
    expect(purchaseRoute).toContain('requiresPayment: true')
    expect(purchaseRoute).not.toContain('descriptionKey')
  })

  it('不保留购买页旧说明文案', () => {
    const zhLocale = readFileSync('src/i18n/locales/zh.ts', 'utf8')
    const enLocale = readFileSync('src/i18n/locales/en.ts', 'utf8')

    expect(zhLocale.includes('通过内嵌页面完成订阅购买')).toBe(false)
    expect(enLocale.includes('Purchase subscriptions via the embedded page')).toBe(false)
  })
})
