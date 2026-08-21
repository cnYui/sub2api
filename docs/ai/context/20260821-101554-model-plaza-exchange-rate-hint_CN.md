# 模型广场汇率提示

## 背景

用户要求在本地模型广场页面增加与上游页面一致的汇率说明，解释国产模型价格按人民币口径展示时看起来更高的原因。

## 实现

- 在 `frontend/src/components/modelPlaza/ModelPlazaContent.vue` 的页面标题下方增加信息提示条。
- 提示条复用 `infoCircle` 图标和模型广场现有圆角、边框、暗色模式样式，因此独立页面与登录后的内嵌页面均可见。
- 中文文案说明国产模型按 `1 USD = 7 CNY`、国外模型按 `1 USD = 1 CNY` 展示，并提示比较国产模型时除以 7。
- 英文语言包增加对应文案，避免切换语言时出现缺失键。

## 验证

- `frontend/pnpm exec vitest run src/components/modelPlaza/__tests__/PlazaModelPricingTable.spec.ts`：14 项通过。
- `frontend/pnpm run typecheck`：通过。
- 仅出现仓库已有的 Browserslist 数据过期提示。

## 取舍

提示采用前端固定文案，而不是管理员可编辑描述：该说明是模型广场价格展示的统一口径，不应因后台可选营销描述缺失而消失，也避免管理员误删关键计价解释。
