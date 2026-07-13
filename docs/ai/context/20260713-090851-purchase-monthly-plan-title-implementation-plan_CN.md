# Purchase 套餐月度标题实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 `/purchase` 订阅卡片标题从“阅读订阅套餐A/B/...”改为“月度订阅套餐A/B/...”。

**Architecture:** 保留现有 `PaymentView.vue` 的套餐卡片模型构建流程，只替换标题生成前缀。通过现有页面测试覆盖动态套餐序号，通过组件测试保持共享卡片夹具与页面文案一致。

**Tech Stack:** Vue 3、TypeScript、Vitest、Vue Test Utils、pnpm。

---

### Task 1: 修改订阅套餐卡片标题

**Files:**
- Modify: `frontend/src/views/user/PaymentView.vue:637`
- Test: `frontend/src/views/user/__tests__/PaymentView.spec.ts`
- Test: `frontend/src/components/payment/__tests__/PurchaseProductCard.spec.ts`

- [ ] **Step 1: 先修改测试期望**

把上述两个测试文件中用于购买页订阅卡片的“阅读订阅套餐”期望和夹具统一改为“月度订阅套餐”，包括 A、D、F、G 套餐：

```ts
expect(wrapper.text()).toContain('月度订阅套餐A')
expect(wrapper.text()).toContain('月度订阅套餐D')
expect(wrapper.text()).toContain('月度订阅套餐F')
expect(wrapper.text()).toContain('月度订阅套餐G')
```

- [ ] **Step 2: 运行目标测试并确认 RED**

Run:

```bash
cd frontend
pnpm vitest run src/views/user/__tests__/PaymentView.spec.ts src/components/payment/__tests__/PurchaseProductCard.spec.ts
```

Expected: `PaymentView.spec.ts` 因页面仍输出“阅读订阅套餐”而失败；失败原因必须是缺少“月度订阅套餐”文本。

- [ ] **Step 3: 写入最小生产代码**

在 `buildSubscriptionProduct()` 中只替换标题前缀：

```ts
title: `月度订阅套餐${planTitleSuffix(index)}`,
```

- [ ] **Step 4: 运行目标测试并确认 GREEN**

Run:

```bash
cd frontend
pnpm vitest run src/views/user/__tests__/PaymentView.spec.ts src/components/payment/__tests__/PurchaseProductCard.spec.ts
```

Expected: 两个测试文件全部通过。

- [ ] **Step 5: 运行静态验证**

Run:

```bash
cd frontend
pnpm typecheck
cd ..
git diff --check
```

Expected: 类型检查和差异格式检查均以退出码 0 完成。

- [ ] **Step 6: 记录结果**

新建 `docs/ai/context/YYYYMMDD-HHMMSS-purchase-monthly-plan-title-result_CN.md`，记录修改范围、TDD 红绿结果、验证命令和未部署说明；在 `AGENTS.md` 顶部追加结果索引，不覆盖已有历史记录。
