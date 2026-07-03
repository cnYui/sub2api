# Purchase Product Card Light Theme Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 `/purchase` 产品卡片在浅色模式下呈现方案 B 的白底黑字轻高级白卡，同时保留深色模式现有黑卡效果。

**Architecture:** 只修改共享产品卡组件 `PurchaseProductCard.vue` 的 Tailwind 类名，不改组件接口、DOM 结构和支付业务逻辑。测试同步更新组件单测，让浅色默认类和深色 `dark:` 回退都有明确断言。

**Tech Stack:** Vue 3、Tailwind CSS、Vitest、Vue Test Utils。

---

### Task 1: 更新组件单测以锁定浅色/深色主题边界

**Files:**
- Modify: `frontend/src/components/payment/__tests__/PurchaseProductCard.spec.ts`

- [ ] **Step 1: 写出浅色默认白卡与深色黑卡回退断言**

将第一个测试里的样式断言从固定 `bg-black` 改为浅色与深色并存：

```ts
expect(card.classes()).toEqual(expect.arrayContaining([
  'rounded-2xl',
  'border',
  'bg-gradient-to-b',
  'from-white',
  'to-gray-50',
  'text-gray-900',
  'dark:bg-black',
  'dark:from-black',
  'dark:to-black',
]))
expect(button.classes()).toEqual(expect.arrayContaining([
  'rounded-full',
  'bg-gray-950',
  'text-white',
  'dark:bg-white',
  'dark:text-black',
  'py-4',
]))
```

- [ ] **Step 2: 运行单测确认先失败**

Run:

```bash
cd frontend && pnpm vitest run src/components/payment/__tests__/PurchaseProductCard.spec.ts
```

Expected:

```text
FAIL src/components/payment/__tests__/PurchaseProductCard.spec.ts
AssertionError: expected [...] to deeply equal ArrayContaining [...]
```

原因是当前组件仍只有固定 `bg-black`、`text-white` 和白色按钮。

### Task 2: 实现方案 B 的浅色产品卡样式

**Files:**
- Modify: `frontend/src/components/payment/PurchaseProductCard.vue`

- [ ] **Step 1: 替换外层卡片类名**

将外层卡片固定黑卡类替换为浅色默认 + 深色回退：

```vue
:class="[
  'group relative flex min-h-[380px] flex-col overflow-hidden rounded-2xl border border-gray-200 border-t-gray-300 bg-gradient-to-b from-white to-gray-50 text-gray-900 shadow-[0_22px_44px_rgba(15,23,42,0.14)] transition-[transform,box-shadow,border-color] duration-500 ease-out hover:-translate-y-2 hover:border-gray-300 hover:border-t-gray-400 hover:shadow-[0_26px_52px_rgba(15,23,42,0.18)] dark:border-white/15 dark:border-t-white/40 dark:bg-black dark:from-black dark:to-black dark:text-white dark:shadow-[0_20px_40px_rgba(0,0,0,0.35)] dark:hover:border-white/30 dark:hover:border-t-white/70 dark:hover:shadow-[0_24px_48px_rgba(0,0,0,0.7)]',
  product.active ? 'border-gray-300 dark:border-white/25' : '',
]"
```

- [ ] **Step 2: 替换顶部渐变遮罩**

```vue
<div class="pointer-events-none absolute inset-0 bg-[linear-gradient(180deg,rgba(15,23,42,0.035)_0%,rgba(15,23,42,0)_100%)] opacity-80 transition-opacity duration-500 group-hover:opacity-100 dark:bg-[linear-gradient(180deg,rgba(255,255,255,0.035)_0%,rgba(255,255,255,0)_100%)]" />
```

- [ ] **Step 3: 替换文字、分隔线和按钮类名**

将模板内固定颜色改为：

```vue
<span class="mb-4 block border-b border-gray-200 pb-2 text-[12px] font-medium uppercase leading-4 tracking-normal text-gray-500 dark:border-white/10 dark:text-[#999999]">PLAN</span>
<h3 class="mb-2 text-[32px] font-normal leading-[40px] tracking-normal text-gray-950 dark:text-white">{{ product.title }}</h3>

<span class="text-base font-normal leading-6 text-gray-500 dark:text-[#999999]">Price</span>
<span class="text-[40px] font-semibold leading-none tracking-normal text-gray-950 dark:text-white">{{ product.priceText }}</span>

<ul class="space-y-3 border-t border-gray-200 pt-4 text-sm leading-6 text-gray-500 dark:border-white/10 dark:text-[#999999]">
  <li v-for="item in product.detailRows" :key="item.label" class="flex justify-between gap-4">
    <span>{{ item.label }}</span>
    <span class="text-right font-medium text-gray-950 dark:text-white">{{ item.value }}</span>
  </li>
</ul>

<button
  type="button"
  class="mt-8 w-full rounded-full border border-gray-950 bg-gray-950 px-6 py-4 text-[12px] font-bold leading-4 tracking-normal text-white transition-all duration-300 hover:scale-[0.98] hover:bg-white hover:text-gray-950 active:scale-[0.96] dark:border-white dark:bg-white dark:text-black dark:hover:bg-transparent dark:hover:text-white"
  @click="emit('select', product)"
>
```

- [ ] **Step 4: 运行组件单测确认通过**

Run:

```bash
cd frontend && pnpm vitest run src/components/payment/__tests__/PurchaseProductCard.spec.ts
```

Expected:

```text
PASS src/components/payment/__tests__/PurchaseProductCard.spec.ts
```

### Task 3: 做购买页相关回归验证

**Files:**
- Test only: `frontend/src/views/user/__tests__/PaymentView.spec.ts`

- [ ] **Step 1: 运行购买页单测**

Run:

```bash
cd frontend && pnpm vitest run src/views/user/__tests__/PaymentView.spec.ts
```

Expected:

```text
PASS src/views/user/__tests__/PaymentView.spec.ts
```

- [ ] **Step 2: 运行前端类型检查**

Run:

```bash
cd frontend && pnpm typecheck
```

Expected:

```text
无 TypeScript 类型错误
```

- [ ] **Step 3: 写结果上下文文档**

Create:

```text
docs/ai/context/YYYYMMDD-HHMMSS-purchase-product-card-light-theme-result_CN.md
```

内容记录：

```md
# 购买页产品卡片浅色模式结果

## 改动

- `PurchaseProductCard.vue` 浅色模式改为方案 B 白底黑字轻高级白卡。
- 深色模式保留黑底白字黑卡效果。
- 更新组件单测，覆盖浅色默认类和深色回退类。

## 验证

- `cd frontend && pnpm vitest run src/components/payment/__tests__/PurchaseProductCard.spec.ts`
- `cd frontend && pnpm vitest run src/views/user/__tests__/PaymentView.spec.ts`
- `cd frontend && pnpm typecheck`
```

## Self-Review

- Spec coverage: 覆盖了只改共享产品卡、浅色方案 B、深色保留、测试更新和结果文档。
- Placeholder scan: 无 `TBD`、`TODO`、`implement later` 或未定义步骤。
- Type consistency: 未新增类型，现有 `PurchaseProductCardModel`、props 和 emit 不变。
