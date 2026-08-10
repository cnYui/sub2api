import { existsSync, readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const currentDir = dirname(fileURLToPath(import.meta.url))
const viewPath = resolve(currentDir, '../UsageGuideView.vue')
const routerPath = resolve(currentDir, '../../../router/index.ts')
const assetsDir = resolve(currentDir, '../../../assets/usage-guide')
const publicUsageGuideDir = resolve(currentDir, '../../../../public/usage-guide')

describe('UsageGuideView', () => {
  it('包含使用方法栏目与所有主题', () => {
    expect(existsSync(viewPath)).toBe(true)
    const source = readFileSync(viewPath, 'utf8')

    for (const token of [
      "id: 'codex'",
      "id: 'ccswitch-video'",
      "id: 'formal-api'",
      "id: 'copilot-vscode'",
      "id: 'error-codes'",
      "id: 'image-generation'",
      "id: 'trae'",
      "id: 'claude-code-desktop'",
      "const hiddenGuideTopicIds = new Set<GuideTopic['id']>(['image-generation'])",
      'usage-guide-topic-nav-desktop',
      'usage-guide-topic-tabs-mobile',
      'usage-guide-video',
      'X-Sub2API-Error-ID',
      'https://api.aaccx.pw/v1/responses',
      'Authorization: Bearer sk-xxxx',
      'https://api.aaccx.pw/v1beta',
      '当前行为',
      'api_key_in_query_deprecated',
      'insufficient_quota',
      '当前 main 尚未把所有端点统一迁移到 X-Sub2API-Error-ID / S2A-* 契约',
      'const guideTopics = allGuideTopics',
      '.sort((left, right) => right.updatedAt.localeCompare(left.updatedAt))',
    ]) {
      expect(source).toContain(token)
    }
    expect(source).not.toContain('400 INVALID_BASE_URL')

    for (const [id, date] of [
      ['formal-api', '2026-08-05'],
      ['claude-code-desktop', '2026-08-05'],
      ['codex', '2026-08-09'],
      ['error-codes', '2026-08-05'],
      ['ccswitch-video', '2026-07-14'],
      ['copilot-vscode', '2026-07-10'],
      ['image-generation', '2026-07-07'],
      ['trae', '2026-06-24'],
    ]) {
      expect(source).toContain(`id: '${id}'`)
      expect(source).toContain(`updatedAt: '${date}'`)
    }
  })

  it('教程截图、视频和封面资源全部存在', () => {
    const imageNames = [
      'step-01-shop-entry.png',
      'step-02-login-register.png',
      'step-03-subscription-plans.png',
      'step-04-confirm-payment.png',
      'step-05-create-api-key.png',
      'step-06-key-group-advanced.png',
      'step-07-cc-switch-provider-list.png',
      'step-07-cc-switch-edit-provider.png',
      'step-08-cc-switch-active.png',
      'trae-step-01-add-model.png',
      'trae-step-02-custom-config.png',
      'trae-step-03-fill-url-key.png',
      'trae-step-04-select-model.png',
      'claude-code-step-01-select-group.png',
      'claude-code-step-02-ccswitch-route.png',
      'claude-code-step-03-provider-config.png',
      'claude-code-step-04-model-select.png',
      'claude-code-step-05-enable-route.png',
      'claude-code-step-06-restart-desktop.png',
      'codex-ccswitch-step-01.png',
      'codex-ccswitch-step-02.png',
      'codex-ccswitch-step-03.png',
      'codex-ccswitch-step-04.png',
      'codex-ccswitch-step-05.png',
      'codex-ccswitch-step-06.png',
      'codex-ccswitch-step-07.png',
      'codex-ccswitch-step-08.png',
      'codex-ccswitch-step-09.png',
      'codex-ccswitch-step-10.png',
    ]

    for (const imageName of imageNames) {
      expect(existsSync(resolve(assetsDir, imageName))).toBe(true)
    }
    expect(existsSync(resolve(publicUsageGuideDir, 'ccswitch-relay-connection-guide.mp4'))).toBe(true)
    expect(existsSync(resolve(publicUsageGuideDir, 'ccswitch-relay-connection-guide-poster.webp'))).toBe(true)
  })

  it('注册为登录后用户页面并提供中英文入口文案', () => {
    const routerSource = readFileSync(routerPath, 'utf8')
    expect(routerSource).toContain("path: '/usage-guide'")
    expect(routerSource).toContain("component: () => import('@/views/user/UsageGuideView.vue')")
    expect(routerSource).toContain("titleKey: 'usageGuide.title'")
  })
})
