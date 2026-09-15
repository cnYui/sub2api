import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import en from '@/i18n/locales/en'
import zh from '@/i18n/locales/zh'

const here = dirname(fileURLToPath(import.meta.url))
const read = (path: string) => readFileSync(resolve(here, path), 'utf8')

// 递归收集键路径，保证 zh/en 的整棵键树（不只是顶层）完全对称
function keyPaths(value: unknown, prefix = ''): string[] {
  if (!value || typeof value !== 'object') return [prefix]
  return Object.keys(value as Record<string, unknown>)
    .sort()
    .flatMap((key) => keyPaths((value as Record<string, unknown>)[key], prefix ? `${prefix}.${key}` : key))
}

describe('Reimbursement integration surface', () => {
  it('registers the user route and the admin route with the right guards', () => {
    const router = read('../../router/index.ts')

    expect(router).toContain("path: '/reimbursement'")
    const userRoute = router.slice(router.indexOf("path: '/reimbursement'"), router.indexOf("path: '/affiliate'"))
    expect(userRoute).toContain("name: 'Reimbursement'")
    expect(userRoute).toContain("component: () => import('@/views/user/ReimbursementView.vue')")
    expect(userRoute).toContain('requiresAuth: true')
    expect(userRoute).toContain('requiresAdmin: false')
    expect(userRoute).toContain("titleKey: 'reimbursement.title'")
    expect(userRoute).toContain("descriptionKey: 'reimbursement.description'")
    expect(userRoute).not.toContain('requiresPayment')

    expect(router).toContain("path: '/admin/reimbursements'")
    const adminRoute = router.slice(
      router.indexOf("path: '/admin/reimbursements'"),
      router.indexOf("path: '/admin/promo-codes'")
    )
    expect(adminRoute).toContain("name: 'AdminReimbursements'")
    expect(adminRoute).toContain("component: () => import('@/views/admin/ReimbursementsView.vue')")
    expect(adminRoute).toContain('requiresAuth: true')
    expect(adminRoute).toContain('requiresAdmin: true')
    expect(adminRoute).toContain("titleKey: 'admin.reimbursements.title'")
    expect(adminRoute).toContain("descriptionKey: 'admin.reimbursements.description'")
  })

  it('blocks both pages in simple mode via restrictedPaths', () => {
    const router = read('../../router/index.ts')
    const start = router.indexOf('const restrictedPaths = [')
    expect(start).toBeGreaterThan(-1)
    const block = router.slice(start, router.indexOf(']', start))
    expect(block).toContain("'/reimbursement'")
    expect(block).toContain("'/admin/reimbursements'")
  })

  it('adds a sidebar entry for users and one for admins, both hidden in simple mode', () => {
    const sidebar = read('../../components/layout/AppSidebar.vue')

    const userItem = sidebar.slice(sidebar.indexOf("path: '/reimbursement'"), sidebar.indexOf("path: '/usage-guide'"))
    expect(userItem).toContain("label: t('nav.reimbursement')")
    expect(userItem).toContain('hideInSimpleMode: true')

    const adminItem = sidebar.slice(
      sidebar.indexOf("path: '/admin/reimbursements'"),
      sidebar.indexOf("path: '/admin/affiliates'")
    )
    expect(adminItem).toContain("label: t('nav.reimbursements')")
    expect(adminItem).toContain('icon: DocumentIcon')
    expect(adminItem).toContain('hideInSimpleMode: true')
    expect(sidebar).toContain('const DocumentIcon = {')
  })

  it('keeps zh/en locale trees symmetric for reimbursement, admin.reimbursements and nav entries', () => {
    expect(keyPaths(zh.reimbursement)).toEqual(keyPaths(en.reimbursement))
    expect(keyPaths(zh.admin.reimbursements)).toEqual(keyPaths(en.admin.reimbursements))
    expect(zh.nav.reimbursement).toBeTruthy()
    expect(en.nav.reimbursement).toBeTruthy()
    expect(zh.nav.reimbursements).toBeTruthy()
    expect(en.nav.reimbursements).toBeTruthy()
    expect(zh.reimbursement.status.pending).toBe('审核中')
    expect(zh.admin.reimbursements.statusLabels.pending).toBe('待处理')
  })

  it('exposes the API modules on the barrels with the contract paths', () => {
    const userApi = read('../../api/reimbursement.ts')
    expect(userApi).toContain("'/reimbursement/parse'")
    expect(userApi).toContain("'/reimbursement/requests'")
    expect(userApi).toContain('/reimbursement/requests/${id}/pdf')
    expect(userApi).toContain("responseType: 'blob'")
    expect(read('../../api/index.ts')).toContain("from './reimbursement'")

    const adminApi = read('../../api/admin/reimbursements.ts')
    expect(adminApi).toContain("'/admin/reimbursement/requests'")
    expect(adminApi).toContain('/admin/reimbursement/requests/${id}/pdf')
    expect(adminApi).toContain("'/admin/reimbursement/config'")
    expect(adminApi).toContain("'/admin/reimbursement/config/test'")
    expect(adminApi).toContain("'Content-Type': 'multipart/form-data'")
    const adminIndex = read('../../api/admin/index.ts')
    expect(adminIndex).toContain('reimbursements: reimbursementsAPI')
  })
})
