# 18084 候选环境购买入口隐藏排查与修复计划

## 背景

用户在 `http://127.0.0.1:18084/subscriptions` 登录普通用户后，左侧菜单缺少“购买/订阅续费”入口。截图显示“我的订阅”页面可访问，但侧栏只展示“我的订阅”“兑换”“使用方法”等入口。

## 排查证据

- `GET /api/v1/settings/public` 返回 `payment_enabled=false`、`purchase_subscription_enabled=false`。
- 前端路由仍存在 `/purchase`，页面文件仍存在 `frontend/src/views/user/PaymentView.vue`。
- 侧栏 `frontend/src/components/layout/AppSidebar.vue` 中 `/purchase` 和 `/orders` 入口依赖 `FeatureFlags.payment`。
- `FeatureFlags.payment` 绑定公开设置 `payment_enabled`。
- 前端路由守卫中 `meta.requiresPayment` 也依赖 `payment_enabled`；当该值为 false 时会把普通用户重定向到 `/dashboard`。
- 18084 候选环境来自生产 DB 克隆，但候选清洗脚本会写入 `payment_enabled=false`，目的是关闭外部支付副作用。

## 根因判断

这不是最新 main 删除了购买页，也不是路由丢失，而是候选环境清洗策略把“支付系统总开关”关掉，导致前端侧栏入口和 `/purchase` 路由守卫一起隐藏购买页面。

候选预演需要验证购买页、套餐、流量包和 checkout 数据；因此不能关闭 `payment_enabled`。为了避免真实外部支付，应关闭具体支付 provider 或可见支付方式，而不是关闭购买页总入口。

## 修复范围

只处理本地候选环境：

- 修改候选预演 worktree 的 `deploy/sql/candidate-sanitize.sql`，保留 `payment_enabled=true`，继续关闭外部 provider、支付可见方法、SMTP、监控等副作用。
- 只更新 18084 候选克隆 DB 的设置，让当前候选环境立即恢复购买入口。
- 不重建、不切换、不写公网 `sub2api`、公网 Postgres 或公网 Redis。

## 验证计划

- `127.0.0.1:18084/api/v1/settings/public` 应返回 `payment_enabled=true`。
- 普通用户登录后侧栏应出现“购买/订阅续费”入口。
- `/purchase` 页面可访问，能展示订阅套餐和 GPT 流量包。
- 因候选 provider 被禁用，下单按钮应保持不可提交或显示支付不可用，不触发真实外部支付。
- `127.0.0.1:18080/health` 仍为 200，确认公网容器未受影响。
