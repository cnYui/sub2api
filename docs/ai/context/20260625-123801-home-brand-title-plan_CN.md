# /home 首页品牌文案调整计划

## 目标

将默认 `/home` 首屏中连续展示的 `Sub2API` 与 `Subscription to API Conversion Platform` 调整为只展示 `天才程序员小站`，其他首页内容、按钮、套餐卡片、模型列表、路由和鉴权行为保持不变。

## 现状

- `frontend/src/views/HomeView.vue` 的 Hero 标题来自 `siteName`，副标题来自 `siteSubtitle`。
- 前端默认品牌常量已包含 `天才程序员小站`，但本地截图仍显示旧文案，说明当前运行态优先使用后端公共设置返回的默认 `site_name/site_subtitle`。
- 后端 `backend/internal/service/setting_service.go` 当前在公共设置为空时回退到 `Sub2API` 和 `Subscription to API Conversion Platform`。

## 设计

- 后端公共设置默认值改为 `site_name=天才程序员小站`、`site_subtitle=`，从源头消除默认旧文案。
- 前端 Home 页只在公共设置明确提供 `site_subtitle` 时展示副标题；空字符串不再回退到前端默认副标题，避免“只改标题但仍显示副标题”。
- 前端默认品牌副标题同步为空，保持无后端配置、测试和文档标题的默认语义一致。
- 管理后台表单初始化默认 `site_name` 改为 `天才程序员小站`，默认 `site_subtitle` 改为空；已有后台配置不被覆盖。

## 实施计划

1. 先更新单测期望，覆盖默认品牌常量、HomeView 默认渲染、后端公共设置合约中的默认 `site_name/site_subtitle`。
2. 运行相关测试，确认新期望会在旧实现下失败。
3. 修改最小实现：前端默认常量、HomeView subtitle 计算、后台设置默认值、后端公共设置默认值。
4. 运行相关前端与后端测试，确认只影响目标文案。

## 验证

- `pnpm --dir frontend test:run src/constants/__tests__/branding.spec.ts src/views/__tests__/HomeView.spec.ts`
- 后端公共设置合约相关测试按实际测试名运行。

## 风险

- 如果线上数据库已经保存了旧 `site_name/site_subtitle`，代码默认值不会自动覆盖数据库配置；需要通过管理后台或数据库更新现有配置。
