# 购买页空白排查与修复计划

## 现象

`http://127.0.0.1:18084/purchase` 普通用户可进入页面，左侧“购买订阅”入口和顶部标题都显示，但主内容区视觉为空白。

## 证据

- 当前 DOM 中 `main` 的 `textContent` 已包含 29/39/59/99 元订阅和 GPT 流量包。
- 页面截图中主内容区不可见。
- 浏览器计算样式显示计划列表 grid 宽高为 `0x0`。
- `main.outerHTML` 为：

```html
<main class="p-4 md:p-6 lg:p-8">
  <div class="mx-auto max-w-4xl space-y-6">
    <template></template>
    <!---->
  </div>
</main>
```

套餐内容被放入原生 `<template>` 的 `content`，因此存在于 DOM 文本但不会被浏览器渲染。

## 根因

`frontend/src/views/user/PaymentView.vue` 中 `v-else` 分支下面多包了一层没有指令的裸 `<template>`。该标签在当前构建中作为原生 HTML `<template>` 输出，导致购买卡片全部不可见。

## 修复方案

- 先补前端测试，要求购买页选择态不能渲染原生 `<template>` 包住购买内容。
- 删除 `PaymentView.vue` 中冗余裸 `<template>` wrapper，保留已有 `v-if`/`v-else-if`/`v-else` 结构。
- 跑目标测试。
- 重建 18084 候选镜像并只替换 `sub2api-candidate`，不碰公网。
- 用浏览器验证 `/purchase` 视觉可见。

## 边界

- 不修改公网 `sub2api`、公网 Postgres、公网 Redis。
- 不改候选 DB 数据。
- 不输出用户密码、token 或 API key。
