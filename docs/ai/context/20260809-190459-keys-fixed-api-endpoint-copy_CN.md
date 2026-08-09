# `/keys` 固定 API 端点复制

## 背景

`/keys` 原有 `EndpointPopover` 仅在公开设置返回 `api_base_url` 或自定义端点时显示。生产页面公开设置为空时，用户无法从 API 密钥页复制接入地址。

## 决策

默认端点固定为 `https://api.aaccx.pw/v1`，始终渲染既有 `EndpointPopover`。保留公开设置中的 `custom_endpoints`，用于展示额外线路；不修改 `api_base_url` 在密钥使用向导等其它流程中的既有含义。

## 实现

- `KeysView.vue` 向既有组件传入固定端点，移除依赖公开设置的显示条件。
- 继续复用组件的点击 URL、复制按钮、键盘访问、复制成功状态和测速链接，不新增平行样式或复制逻辑。
- 增加视图测试，验证公开设置为空时端点仍可见且复制目标正确。

## 验证

- `pnpm exec vitest run src/views/user/__tests__/KeysView.spec.ts src/components/keys/__tests__/EndpointPopover.spec.ts`：2 个测试文件、12 项测试通过。
- `pnpm run build`：`vue-tsc -b && vite build` 通过。构建保留项目既有的动态导入分包和 chunk 体积提示，无新增构建错误。
