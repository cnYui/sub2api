# 99 元套餐使用方法页同步 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把用户“使用方法”页里的生图说明从 `29/39/59` 同步为包含 `99`，并补齐对应测试。

**Architecture:** 这是一次静态文案同步改动，不改接口、不改数据流，只修改使用方法页中的固定说明文案，并让对应测试先失败再通过。实现保持最小范围，避免影响其他教程内容。

**Tech Stack:** Vue 3、TypeScript、Vitest

---

### Task 1: 更新使用方法页的生图套餐文案

**Files:**
- Modify: `frontend/src/views/user/__tests__/UsageGuideView.spec.ts`
- Modify: `frontend/src/views/user/UsageGuideView.vue`

- [ ] **Step 1: 先写失败测试**

把 [frontend/src/views/user/__tests__/UsageGuideView.spec.ts](/Users/wujianxiang/CodeSpace/sub2api/frontend/src/views/user/__tests__/UsageGuideView.spec.ts) 里的断言从：

```ts
'29/39/59 元套餐已支持生图和图生图'
```

改成：

```ts
'29/39/59/99 元套餐已支持生图和图生图'
```

- [ ] **Step 2: 运行单测，确认它先失败**

Run:

```bash
cd /Users/wujianxiang/CodeSpace/sub2api/frontend && npx vitest run src/views/user/__tests__/UsageGuideView.spec.ts
```

Expected:

```text
FAIL
缺少生图教程信息：29/39/59/99 元套餐已支持生图和图生图
```

- [ ] **Step 3: 写最小实现**

把 [frontend/src/views/user/UsageGuideView.vue](/Users/wujianxiang/CodeSpace/sub2api/frontend/src/views/user/UsageGuideView.vue) 第 `guideTopics -> image-generation -> 可用范围` 中的文案：

```ts
'29/39/59 元套餐已支持生图和图生图，使用你已经生成的 API Key 即可直接请求图片接口。'
```

改成：

```ts
'29/39/59/99 元套餐已支持生图和图生图，使用你已经生成的 API Key 即可直接请求图片接口。'
```

- [ ] **Step 4: 重新运行单测，确认通过**

Run:

```bash
cd /Users/wujianxiang/CodeSpace/sub2api/frontend && npx vitest run src/views/user/__tests__/UsageGuideView.spec.ts
```

Expected:

```text
PASS
1 test file passed
```

- [ ] **Step 5: 运行支付页相关回归测试**

Run:

```bash
cd /Users/wujianxiang/CodeSpace/sub2api/frontend && npx vitest run src/views/user/__tests__/PaymentView.spec.ts src/views/user/__tests__/UsageGuideView.spec.ts
```

Expected:

```text
PASS
2 test files passed
```
