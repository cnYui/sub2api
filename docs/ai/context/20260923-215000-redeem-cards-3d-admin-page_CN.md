# 兑换卡：后台制作页 + 3D 分享页（2026-09-23）

## 需求

站长原话：「发放兑换码的卡片格式有两种：黑色和白色。你把这个加到管理员账号里……在左侧的导航栏中单独加一个页面，给管理员设定兑换码的」。

追问后确定：

- 兑换码来源：可以选已有的未使用兑换码，也可以在页面里直接生成；
- 产出：3D 卡片网页，通过链接发给用户；
- 正面的主理人信息、背面的二维码要能改；
- 3D 模板在 Claude Design（文件 `兑换卡 3D.html`），厚度用 0.6mm，页面上除了卡片其它文字全删。

## 实现

| 部分 | 位置 |
| --- | --- |
| 表 `redeem_cards` | `backend/migrations/217_redeem_cards.sql`、`backend/ent/schema/redeem_card.go` |
| 服务 | `backend/internal/service/redeem_card_service.go`（卡片增删改查、卡面信息存取、公开读取与打开计数） |
| 后台接口 | `GET/POST /api/v1/admin/redeem-cards`、`GET /admin/redeem-cards/by-code/:code_id`、`DELETE /admin/redeem-cards/:id`、`GET/PUT /admin/redeem-card-profile` |
| 公开接口 | `GET /api/v1/redeem-cards/:token`（PublicIP 限流 + OptionalJWT + BackendModeUserGuard，响应 `no-store`、`noindex`） |
| 卡面组件 | `frontend/src/components/redeemCard/`：`redeemCardModel.ts`（尺寸、热力图、分组、自动填写、侧边几何）、`RedeemCardFace.vue`（正反面）、`RedeemCardScaled.vue`（平面预览）、`RedeemCard3D.vue`（3D） |
| 后台页 | `frontend/src/views/admin/RedeemCardsView.vue`，路由 `/admin/redeem-cards`，侧栏「兑换码」下方 |
| 分享页 | `frontend/src/views/public/RedeemCardView.vue`，路由 `/card/:token` |

要点：

- 一个兑换码最多一张卡（唯一索引），保存是「有则更新、无则新建」，链接（token）不变。两个管理员同时建卡时，唯一索引挡下后一个，再查一次改成更新。
- token 是 32 位小写 hex（128 位随机），公开接口先用正则校验再查库。
- 卡面公共信息存 settings `redeem_card_profile`：文字限 64 字，简介最多 4 行，步骤最多 5 步，二维码只收 png/jpeg/webp 的 data URL，最大 512KB。
- 兑换码按原文显示，4 位一组只靠 CSS 间距，照卡片抄写必须能兑换成功（兑换是大小写敏感的精确匹配）。
- 自动填写：余额码填 `$面值 / 余额充值`，套餐码填 `¥价格 / 档位名 · 天数 / 每期 $额度 × 期数`，有效期取兑换码的 `expires_at`，编号取兑换码 ID 补零到 4 位，热力图种子随机。
- 兑换码未使用但已过 `expires_at` 时，公开接口报 `expired`；已兑换 / 过期 / 停用会在背面盖章。

## 3D 分享页

- 卡面按设计尺寸 1712×1080（20px/mm）排版，整个场景按容器宽度缩放；厚度 0.6mm = 12 设计像素，两面各 `translateZ(±6px)`。
- 侧边：沿圆角矩形外轮廓切成 108 个竖直小条（长边约 66px 一条、每个圆角 8 条），每条从背面描边色渐变到正面描边色。背面翻转过，同一点的背面颜色取 `borderColorAt(W - x, y)`。
- 交互：拖动旋转（整张卡宽 = 180°），松手带惯性后回正到最近的一面；轻点翻面；背面朝前时轻点兑换码复制（面板发绿光，不弹文字）；静止时轻微浮动，`prefers-reduced-motion` 下不浮动。手机上在卡片外面拖也能转。
- 页面上只有卡片。链接失效时只显示一句「链接无效或已被撤销」。
- 取数不走 `apiClient`：它会带本地保存的令牌，令牌过期时接口回 401，拦截器会跳登录页。收卡的人不需要登录。
- 设计稿兑换码面板的 `backdrop-filter` 换成了垫底色：面板后面只有 6% 透明度的网格，模糊后等于纯底色，视觉一致；而 iOS 上 `backdrop-filter` 和 3D 变换叠用会有渲染问题。

## 没做到的

**站长的 3D 模板没读到。** 链接是 `claude.ai/design/p/...`：DesignSync 工具只能在用户启动的 `/design-sync` 里用，Artifact 读取只认 artifact 链接，内置浏览器没有登录 claude.ai。现在的 3D 效果是按「厚度 0.6mm、页面只留卡片」自行实现的。站长把 HTML 导出到 Downloads 后，按模板调 `RedeemCard3D.vue` 即可，数据和接口都不用动。

## 验证

- 后端：`redeem_card_service_test.go` 6 个用例；`go test -tags=unit ./...` 全量。
- 前端：`redeemCardModel.spec.ts`（厚度换算、热力图、分组、自动填写、盖章、描边颜色、侧边首尾相接且总长等于周长）、`RedeemCardView.spec.ts`（页面只有卡片、两面 ±6px、侧边高 12px、盖章、404 与网络错误）；`vitest run` 全量 1441 个通过，`vue-tsc`、ESLint、`npm run build` 通过。
- 浏览器（Vite + 本地 mock 接口）：
  - 分享页：黑色、白色、已兑换三种卡都实际看过，翻面、侧面 0.6mm 边、手机 375px 宽都正常；真实点击兑换码后剪贴板写入成功；
  - 后台：选码自动填写、生成新码并自动选中、保存出链接、改卡面信息后预览同步、撤销后链接 404，都实际走过一遍。
