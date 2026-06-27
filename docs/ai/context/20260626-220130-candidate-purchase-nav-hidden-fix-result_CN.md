# 18084 候选环境购买入口隐藏修复结果

## 结论

`http://127.0.0.1:18084` 普通用户侧栏缺少“购买订阅/我的订单”的原因不是 main 代码删除了入口，而是候选 DB 清洗脚本把 `payment_enabled` 写成了 `false`。

`payment_enabled=false` 会同时影响：

- `AppSidebar.vue` 的 `FeatureFlags.payment`，隐藏 `/purchase` 和 `/orders` 菜单。
- `router/index.ts` 的 `meta.requiresPayment` 守卫，直接访问 `/purchase` 会被重定向。
- 服务端注入到 HTML 的 `window.__APP_CONFIG__`，需要重启 app 容器后才会刷新。

## 已修复

- 候选 worktree 的 `deploy/sql/candidate-sanitize.sql` 已改为保留 `payment_enabled=true`。
- 继续关闭候选环境外部副作用：
  - `payment_visible_method_alipay_enabled=false`
  - `payment_visible_method_wxpay_enabled=false`
  - `ENABLED_PAYMENT_TYPES=[]`
  - `payment_provider_instances.enabled=false`
  - SMTP、监控、通知等副作用关闭
- 候选脚本测试 `deploy/test-candidate-rehearsal-scripts.sh` 新增断言：
  - 必须包含 `('payment_enabled', 'true'`
  - 不能包含 `('payment_enabled', 'false'`
- 当前 18084 候选克隆库已把 `settings.payment_enabled` 更新为 `true`。
- 已重启 18084 候选 app，使 HTML 注入的 `window.__APP_CONFIG__` 同步为 `payment_enabled=true`。

## 验证

- `GET http://127.0.0.1:18084/api/v1/settings/public` 返回 `payment_enabled=true`。
- `GET http://127.0.0.1:18084/` 的内联 `window.__APP_CONFIG__` 返回 `payment_enabled=true`。
- 普通用户登录后侧栏显示：
  - `购买订阅`
  - `我的订单`
- `GET http://127.0.0.1:18080/health` 仍为 200。

## 边界

- 未修改公网 DB。
- 未重建公网 `sub2api`。
- 未重建公网 Postgres/Redis。
- 只影响 18084 候选环境和候选预演脚本。
