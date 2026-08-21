# 7 元 30 USD 流量卡上架验证

## 执行结果

- 应用镜像已基于当前工作区构建，manifest 为 `sha256:368cdaec0987b33521f585d7ec6c7ca032ef4331549b19163735f35b6aa8e6bd`。
- 仅替换 `sub2api-official-18082` 应用容器；PostgreSQL、Redis、Nginx、Cloudflare Tunnel 和数据卷未重建。
- 应用启动时已执行并登记 `210_add_traffic_pack_30usd_7cny.sql`。
- 数据库商品核对通过：编码 `traffic_30usd_7cny`，名称“流量包 30 刀”，标价 `7.00`，额度 `30.0000000000 USD`，有效期 `28` 天，平台 `all`，`for_sale=true`，排序 `40`。
- 运行态 `BILLING_FINAL_MULTIPLIER=17`，应用容器状态为 `healthy`。

## 页面与链路核验

- 已通过现有登录浏览器刷新 `https://aaccx.pw/purchase`，页面动态显示新商品。
- 页面展示：标价 `¥7.00`、手续费 `¥0.07`、实付 `¥7.07`、可用额度 `$30`、有效期 `28 天`。
- 未创建真实支付订单，避免产生实际扣款；购买按钮沿用既有 `traffic_pack` 下单流程。
- `127.0.0.1:18082/health`、`127.0.0.1:8080/health`、`https://api.aaccx.pw/health` 均返回 HTTP 200。
- 构建过程仅保留项目既有的 Browserslist、Vite 分包和大 chunk 警告。
