# Codex 接入教程最新版截图更新

## 需求

使用用户提供的 5 张最新版截图，更新认证用户 `/usage-guide` 页面中的“Codex 接入”教程；本次只更新本地源码，不重启或修改公网 `18082` 服务。

## 实现决策

- 继续复用 `UsageGuideView.vue` 现有的“主题 - 步骤 - 图片”数据结构和 `guide-step` 图片渲染组件，不新增平行组件。
- 将 Codex 教程收敛为 4 步：创建或切换 GPT 分组 API Key、打开 CC Switch 的 GPT 栏目并新建凭证、填写供应商/API Key/API 请求地址并保存、启用凭证后重启 Codex 验证成功。
- 第 4 步展示“使用凭证”和“Codex 成功运行”两张连续截图，保持操作链路完整。
- 新截图复制到 `frontend/src/assets/usage-guide/` 并使用描述性文件名；原有旧截图保留，不再被当前 Codex 步骤引用。
- 教程 API 请求地址按截图和用户要求写为 `https://api.aaccx.pw/v1`，不写入任何真实 API Key。

## 变更范围

- 页面：`frontend/src/views/user/UsageGuideView.vue`
- 测试：`frontend/src/views/user/__tests__/UsageGuideView.spec.ts`
- 图片：`frontend/src/assets/usage-guide/codex-latest-step-*.png`

## 验证口径

- 使用方法页面单测检查 Codex 日期和 5 张新截图资源。
- 运行类型检查、Lint 和生产构建，确认静态导入和 Vite 资源打包正常。
- 本次不启动发布流程，不接触公网容器或数据库。
