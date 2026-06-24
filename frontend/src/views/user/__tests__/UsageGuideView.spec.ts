import { existsSync, readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const currentDir = dirname(fileURLToPath(import.meta.url))
const viewPath = resolve(currentDir, '../UsageGuideView.vue')
const routerPath = resolve(currentDir, '../../../router/index.ts')
const assetsDir = resolve(currentDir, '../../../assets/usage-guide')

describe('UsageGuideView', () => {
  it('声明使用方法控制台和三个教程栏目', () => {
    expect(existsSync(viewPath)).toBe(true)
    if (!existsSync(viewPath)) return

    const source = readFileSync(viewPath, 'utf8')

    expect(source).toContain('const guideTopics')
    expect(source).toContain("id: 'codex'")
    expect(source).toContain("title: 'Codex 接入'")
    expect(source).toContain("id: 'image-generation'")
    expect(source).toContain("title: '生图方法'")
    expect(source).toContain("id: 'trae'")
    expect(source).toContain("title: 'Trae 接入'")
    expect(source).toContain('activeTopicId')
    expect(source).toContain('data-test="usage-guide-topic-nav-desktop"')
    expect(source).toContain('data-test="usage-guide-topic-tabs-mobile"')
  })

  it('Codex 接入声明 8 个使用步骤和 10 张截图，保持指定图片顺序', () => {
    expect(existsSync(viewPath)).toBe(true)
    if (!existsSync(viewPath)) return

    const source = readFileSync(viewPath, 'utf8')
    const codexSource = source.slice(
      source.indexOf('const codexSetupSteps'),
      source.indexOf('const imageEditRequestExample'),
    )
    const expectedTokens = [
      "title: '访问 aaccx.pw/shop 页面，点击图中的进入按钮'",
      "alt: '步骤 1 截图 1'",
      "title: '新用户注册，老用户登录'",
      "alt: '步骤 2 截图 1'",
      "title: '选择订阅的页面，选择合适的套餐'",
      "alt: '步骤 3 截图 1'",
      "title: '完成支付后，悠一会给你一个兑换码'",
      "alt: '步骤 4 截图 1'",
      "alt: '步骤 4 截图 2'",
      "title: '兑换成功后，去 API Key 页面生成密钥'",
      "alt: '步骤 5 截图 1'",
      "title: '选择分组，并且可以设置高级功能'",
      "alt: '步骤 6 截图 1'",
      "title: '启动 cc-switch，粘贴 API Key 和请求端口'",
      "alt: '步骤 7 截图 1'",
      "alt: '步骤 7 截图 2'",
      "imagePosition: 'beforeTitle'",
      "alt: '步骤 8 截图 1'",
      "title: '保存配置后，重启 Codex，即可使用！'",
    ]

    let previousIndex = -1
    for (const token of expectedTokens) {
      const index = codexSource.indexOf(token)
      expect(index, `缺少或顺序错误：${token}`).toBeGreaterThan(previousIndex)
      previousIndex = index
    }

    expect(source.match(/data-test="usage-guide-step"/g)?.length).toBe(1)
    expect(codexSource.match(/step: /g)).toHaveLength(8)
    expect(codexSource.match(/alt: '/g)).toHaveLength(10)
  })

  it('生图方法教程只展示用户需要知道的接入和扣费信息', () => {
    expect(existsSync(viewPath)).toBe(true)
    if (!existsSync(viewPath)) return

    const source = readFileSync(viewPath, 'utf8')
    const expectedTokens = [
      "title: '生图方法'",
      '29/39/59/99 元套餐已支持生图和图生图',
      '客户端 Base URL 填 https://api.aaccx.pw/v1',
      '接口路径填 /images/edits',
      '如果工具要求完整 URL，使用 https://api.aaccx.pw/v1/images/edits',
      '文本生图完整 URL 是 https://api.aaccx.pw/v1/images/generations',
      'images[].image_url',
      'image=@/absolute/path/input.png',
      '$0.10 / 张',
      '$0.20 / 张',
      '$0.40 / 张',
      'Authorization: Bearer sk-xxxx',
      'gpt-image-2',
    ]

    for (const token of expectedTokens) {
      expect(source, `缺少生图教程信息：${token}`).toContain(token)
    }

    expect(source).not.toContain('groups.id')
    expect(source).not.toContain('127.0.0.1:18080')
    expect(source).not.toContain('POST /v1/images/edits')
  })

  it('Trae 接入教程复用步骤样式展示自定义模型配置流程', () => {
    expect(existsSync(viewPath)).toBe(true)
    if (!existsSync(viewPath)) return

    const source = readFileSync(viewPath, 'utf8')
    const traeSource = source.slice(
      source.indexOf('const traeSetupSteps'),
      source.indexOf('const guideTopics'),
    )
    const expectedTokens = [
      "title: 'Trae 接入'",
      '把这里生成的 API Key 配置到 Trae 自定义模型中使用。',
      "title: '点击添加模型'",
      "alt: 'Trae 接入步骤 1 截图'",
      "title: '选择自定义配置'",
      "alt: 'Trae 接入步骤 2 截图'",
      "title: '填入 https://api.aaccx.pw/v1、自己的 API Key 和 gpt-5.5'",
      "alt: 'Trae 接入步骤 3 截图'",
      "title: '点击自定义模型中的 gpt-5.5 即可使用'",
      "alt: 'Trae 接入步骤 4 截图'",
    ]

    for (const token of expectedTokens) {
      expect(source, `缺少 Trae 接入教程信息：${token}`).toContain(token)
    }

    expect(traeSource.match(/step: /g)).toHaveLength(4)
    expect(traeSource.match(/alt: '/g)).toHaveLength(4)
    expect(source).not.toContain('sk-LOCAL')
  })

  it('14 张教程截图资源都存在', () => {
    const expectedAssetNames = [
      'step-01-shop-entry.png',
      'step-02-login-register.png',
      'step-03-subscription-plans.png',
      'step-04-payment-submitted.png',
      'step-04-redeem-code.png',
      'step-05-create-api-key.png',
      'step-06-key-group-advanced.png',
      'step-07-cc-switch-provider-list.png',
      'step-07-cc-switch-edit-provider.png',
      'step-08-cc-switch-active.png',
      'trae-step-01-add-model.png',
      'trae-step-02-custom-config.png',
      'trae-step-03-fill-url-key.png',
      'trae-step-04-select-model.png',
    ]

    for (const assetName of expectedAssetNames) {
      expect(
        existsSync(resolve(assetsDir, assetName)),
        `缺少教程截图资源：${assetName}`,
      ).toBe(true)
    }
  })

  it('注册为登录后用户页面路由', () => {
    const routerSource = readFileSync(routerPath, 'utf8')

    expect(routerSource).toContain("path: '/usage-guide'")
    expect(routerSource).toContain("name: 'UsageGuide'")
    expect(routerSource).toContain("component: () => import('@/views/user/UsageGuideView.vue')")
    expect(routerSource).toContain("titleKey: 'usageGuide.title'")
    expect(routerSource).toContain("descriptionKey: 'usageGuide.description'")
    expect(routerSource).toContain('requiresAuth: true')
    expect(routerSource).toContain('requiresAdmin: false')
  })
})
