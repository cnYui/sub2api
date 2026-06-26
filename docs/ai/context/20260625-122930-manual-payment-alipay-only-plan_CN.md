# Purchase 手动支付仅保留支付宝 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Purchase 手动支付弹窗只保留支付宝二维码，并使用用户提供的新收款图片。

**Architecture:** 保持现有 `ManualPaymentDialog.vue` 组件边界不变，只移除组件内部的支付方式切换和微信二维码分支。二维码仍作为前端静态资源 import，提交态和兑换码跳转事件不变。

**Tech Stack:** Vue 3、Vitest、Vue Test Utils、Vite 静态资源 import。

---

### Task 1: 更新组件行为测试

**Files:**
- Modify: `frontend/src/components/payment/__tests__/ManualPaymentDialog.spec.ts`

- [ ] **Step 1: 写失败测试**

将默认展示微信和点击切换支付宝的断言，改成以下行为：

```ts
it('shows plan amount and only renders Alipay QR code', () => {
  const wrapper = mountDialog()

  expect(wrapper.text()).toContain('29 元订阅池')
  expect(wrapper.text()).toContain('¥29.00')
  expect(wrapper.find('[data-testid="manual-payment-tab-wxpay"]').exists()).toBe(false)
  expect(wrapper.find('[data-testid="manual-payment-wxpay-qr"]').exists()).toBe(false)
  expect(wrapper.find('[data-testid="manual-payment-tab-alipay"]').exists()).toBe(false)
  expect(wrapper.find('[data-testid="manual-payment-alipay-qr"]').exists()).toBe(true)
})
```

- [ ] **Step 2: 运行测试确认失败**

Run: `pnpm vitest run src/components/payment/__tests__/ManualPaymentDialog.spec.ts`

Expected: FAIL，失败点来自旧实现仍渲染微信 tab 或默认微信二维码。

### Task 2: 实现支付宝单一二维码

**Files:**
- Modify: `frontend/src/components/payment/ManualPaymentDialog.vue`
- Replace asset: `frontend/src/assets/payment/manual-alipay.png`

- [ ] **Step 1: 替换支付宝二维码资源**

把用户提供的收款图片复制到 `frontend/src/assets/payment/manual-alipay.png`，组件 import 改为该 PNG 文件。

- [ ] **Step 2: 删除微信支付渲染链路**

在 `ManualPaymentDialog.vue` 中移除：

```ts
import wxpayQr from '@/assets/payment/manual-wxpay.jpg'
const activeMethod = ref<'wxpay' | 'alipay'>('wxpay')
function tabClass(method: 'wxpay' | 'alipay') { ... }
activeMethod.value = 'wxpay'
```

模板中移除 tab 容器、微信二维码 `img` 和 `v-if/v-else` 分支，只保留：

```vue
<img
  :src="alipayQr"
  :alt="t('payment.manual.alipayQrAlt')"
  class="mx-auto max-h-[52vh] w-full max-w-sm rounded-md object-contain"
  data-testid="manual-payment-alipay-qr"
/>
```

- [ ] **Step 3: 运行测试确认通过**

Run: `pnpm vitest run src/components/payment/__tests__/ManualPaymentDialog.spec.ts`

Expected: PASS。

### Task 3: 最终验证和记录

**Files:**
- Add: `docs/ai/context/YYYYMMDD-HHMMSS-manual-payment-alipay-only-result_CN.md`

- [ ] **Step 1: 运行类型检查**

Run: `pnpm typecheck`

Expected: PASS，或者如存在既有无关失败，需要记录失败文件和原因。

- [ ] **Step 2: 检查差异**

Run: `git diff -- frontend/src/components/payment/ManualPaymentDialog.vue frontend/src/components/payment/__tests__/ManualPaymentDialog.spec.ts frontend/src/assets/payment/manual-alipay.png`

Expected: 差异只包含手动支付弹窗、对应测试和支付宝二维码资源。

- [ ] **Step 3: 写结果文档**

记录实际修改文件、验证命令和结果，保存在 `docs/ai/context/`，不覆盖历史文档。
