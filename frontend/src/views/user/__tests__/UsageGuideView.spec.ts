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
      'usage-guide-topic-nav-desktop',
      'usage-guide-topic-tabs-mobile',
      'usage-guide-video',
      'X-Sub2API-Error-ID',
      'https://api.aaccx.pw/v1/responses',
    ]) {
      expect(source).toContain(token)
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
