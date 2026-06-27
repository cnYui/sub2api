# /usage-guide 图生图调用说明改造结果

时间：2026-06-24 16:35 JST

## 结果

- 已将普通用户 `/usage-guide` 页面中「生图方法」栏目改为真实可用的图生图调用说明。
- 保持了原有的栏目结构、section 卡片布局和代码块样式，只更新了文案与示例内容。
- 当前页面面向普通用户展示：
  - 公网入口 `https://api.aaccx.pw/v1`
  - 图生图主接口 `POST /v1/images/edits`
  - 模型 `gpt-image-2`
  - JSON 方式 `images[].image_url`
  - multipart 方式 `image=@...`
  - 可选局部编辑提示 `mask`
  - 1K / 2K / 4K 图片单价

## 改动文件

- `frontend/src/views/user/UsageGuideView.vue`
- `frontend/src/views/user/__tests__/UsageGuideView.spec.ts`

## 验证

- `pnpm test -- --run src/views/user/__tests__/UsageGuideView.spec.ts`
  - 结果：通过，4 个测试全部通过。
- `pnpm typecheck`
  - 结果：通过。
- `pnpm build`
  - 结果：通过。
  - 备注：仍有既有的 Vite chunk warning 和 browserslist 提示，但未阻塞构建，也不是本次改动引入。

## 约束确认

- 页面未暴露 `groups.id`、`127.0.0.1:18080`、CLIProxyAPI 内网地址或明文 API Key。
- 本次未改后端、路由、计费、鉴权和公网配置。
