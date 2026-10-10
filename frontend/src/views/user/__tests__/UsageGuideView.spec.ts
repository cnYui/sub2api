import { existsSync, readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import UsageGuideView from '../UsageGuideView.vue'

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
      "id: 'deepseek-harness'",
      "title: 'DeepSeek Harness 接入中转站 DeepSeek 模型'",
      'const deepseekHarnessSetupSteps: GuideStep[]',
      'API 地址填写 https://api.aaccx.pw/v1',
      "id: 'codex'",
      "id: 'ccswitch-video'",
      "id: 'formal-api'",
      "id: 'copilot-vscode'",
      "id: 'error-codes'",
      "id: 'image-generation'",
      "title: '生图方法'",
      'const imageEndpointRows: GuideEndpointRow[]',
      '403 permission_error',
      'GPT生图1倍率',
      'gpt-image-2.5-flare',
      'gpt-image-2.5-sunburst',
      'https://api.aaccx.pw/v1/images/generations',
      'https://api.aaccx.pw/v1/images/edits',
      'Image generation is not enabled for this group',
      "id: 'trae'",
      "id: 'claude-desktop'",
      "title: 'Claude Desktop 接入中转站 Claude 渠道模型方法'",
      'const claudeDesktopSetupSteps: GuideStep[]',
      '请求地址填写 https://api.aaccx.pw，不要在末尾添加斜杠',
      "id: 'codex-gpt'",
      "title: 'Codex 接入 GPT 模型'",
      'const codexGptSetupSteps: GuideStep[]',
      '请求地址填写 https://api.aaccx.pw 即可',
      "id: 'workbuddy'",
      'usage-guide-topic-nav-desktop',
      'usage-guide-topic-tabs-mobile',
      'usage-guide-video',
      'X-Sub2API-Error-ID',
      'https://api.aaccx.pw/v1/responses',
      'https://api.aaccx.pw/v1',
      'gpt-5.5',
      'WorkBuddy',
      "title: 'Codex 接入中转站除GPT模型以外的外部模型'",
      "title: 'VS Code Copilot 接入中转站所有模型'",
      "title: 'Trae 接入中转站所有模型'",
      "title: 'WorkBuddy 接入中转站所有模型'",
      'Authorization: Bearer sk-xxxx',
      'https://api.aaccx.pw/v1beta',
      '当前行为',
      'api_key_in_query_deprecated',
      'insufficient_quota',
      '当前 main 尚未把所有端点统一迁移到 X-Sub2API-Error-ID / S2A-* 契约',
      'const guideTopics: GuideTopic[] = [',
      '.sort((left, right) => right.updatedAt.localeCompare(left.updatedAt))',
    ]) {
      expect(source).toContain(token)
    }
    expect(source).not.toContain('400 INVALID_BASE_URL')
    expect(source).not.toContain('hiddenGuideTopicIds')
    // 生图已改为独立分组按张计费，且 gpt-image 系列固定返回 base64；旧教程的套餐档位、Token 计费和 response_format 示例不能再出现。
    for (const stale of ['29/39/59/79/99', '套餐有效倍率', '"response_format"', 'response_format=']) {
      expect(source).not.toContain(stale)
    }

    for (const [id, date] of [
      ['deepseek-harness', '2026-09-10'],
      ['formal-api', '2026-08-05'],
      ['claude-desktop', '2026-08-11'],
      ['codex-gpt', '2026-08-11'],
      ['codex', '2026-08-09'],
      ['error-codes', '2026-08-05'],
      ['ccswitch-video', '2026-07-14'],
      ['copilot-vscode', '2026-07-10'],
      ['image-generation', '2026-10-09'],
      ['trae', '2026-06-24'],
      ['workbuddy', '2026-08-10'],
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
      'workbuddy-step-01-add-custom-model.png',
      'workbuddy-step-02-select-custom-provider.png',
      'workbuddy-step-03-fill-custom-model.png',
      'workbuddy-step-04-start-chat.png',
      'claude-desktop-step-01-select-and-add.png',
      'claude-desktop-step-02-create-key.png',
      'claude-desktop-step-03-provider-fields.png',
      'claude-desktop-step-04-provider-result.png',
      'claude-desktop-step-05-enable-and-restart.png',
      'claude-desktop-step-06-quit-menu.png',
      'claude-desktop-step-07-select-model.png',
      'codex-gpt-step-01-create-key.png',
      'codex-gpt-step-02-select-gpt.png',
      'codex-gpt-step-03-provider-config.png',
      'codex-gpt-step-04-enable-restart.png',
      'deepseek-harness-step-01-clone-repo.png',
      'deepseek-harness-step-02-local-start.png',
      'deepseek-harness-step-03-open-settings.png',
      'deepseek-harness-step-04-add-custom-provider.png',
      'deepseek-harness-step-05-fill-url-key.png',
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

  it('生图方法在桌面和移动端导航里可见，点开后展示分组、模型、接口与报错说明', async () => {
    const wrapper = mount(UsageGuideView, {
      global: { stubs: { AppLayout: { template: '<div><slot /></div>' } } },
    })

    for (const selector of [
      '[data-test="usage-guide-topic-nav-desktop"]',
      '[data-test="usage-guide-topic-tabs-mobile"]',
    ]) {
      const titles = wrapper.get(selector).findAll('button').map((button) => button.text())
      expect(titles.some((title) => title.includes('生图方法'))).toBe(true)
    }

    const imageTab = wrapper
      .get('[data-test="usage-guide-topic-nav-desktop"]')
      .findAll('button')
      .find((button) => button.text().includes('生图方法'))
    expect(imageTab).toBeDefined()
    await imageTab?.trigger('click')

    expect(wrapper.get('#usage-guide-heading').text()).toBe('生图方法')
    const text = wrapper.text()
    for (const expected of [
      'GPT生图1倍率',
      'gpt-image-2',
      'gpt-image-1.5',
      'gpt-image-2.5-flare',
      'gpt-image-2.5-sunburst',
      '按张计费',
      'https://api.aaccx.pw/v1/images/generations',
      'https://api.aaccx.pw/v1/images/edits',
      'https://api.aaccx.pw/v1/models',
      'permission_error',
      'model_not_found',
      'content_policy_violation',
      'image=@/absolute/path/input.png',
    ]) {
      expect(text).toContain(expected)
    }
  })
})
