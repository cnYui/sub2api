# /purchase 订阅与流量卡统一商品卡结果

## 改动

- 新增通用卡片组件 `frontend/src/components/payment/PurchaseProductCard.vue`。
- 新增通用展示模型 `frontend/src/components/payment/purchaseProductCard.ts`。
- 删除旧的 `SubscriptionPlanCard.vue` 与 `TrafficPackCard.vue`，避免继续维护两套卡片外观。
- `/purchase` 选择态现在把订阅套餐和 GPT 流量卡合并到同一个网格中，全部使用 `PurchaseProductCard` 渲染。
- 订阅卡标题改为 `阅读订阅套餐A/B/C/D...`，价格、日限额、`24点刷新`、手续费详情保持原逻辑。
- 流量卡标题改为 `5刀流量卡`、`10刀流量卡`、`20刀流量卡`，不再显示 `一次性流量包-有效期 365天`。
- 流量卡第二行从 `可用范围` 改成 `刷新时间`，值使用后端返回的有效期，当前为 `365天`。
- 流量包确认态也去掉 `可用范围` 字段，改为显示 `刷新时间` 与 `可用额度`。
- 续费弹窗也改用同一个通用卡片组件。

## 未改内容

- 未修改后端、支付 API、订单创建、支付回调、运行态数据库或 nginx。
- 流量卡和订阅点击后的确认页/支付流程仍沿用原有 `selectPlan`、`selectTrafficPack` 和 `createOrder` 路径。
- 其他已有未提交后端/部署文件未触碰。

## 验证与运行态

- 已先更新测试并确认旧实现无法满足统一组件和合并网格要求。
- `pnpm --dir frontend test:run src/components/payment/__tests__/PurchaseProductCard.spec.ts src/views/user/__tests__/PaymentView.spec.ts` 通过：2 个测试文件，23 个测试。
- `pnpm --dir frontend typecheck` 通过。
- `git diff --check` 对本轮相关文件通过。
- 已按要求重启 `5174`：`http://127.0.0.1:5174/`，代理到 `http://127.0.0.1:18080`。
- `curl -I --max-time 5 http://127.0.0.1:5174/purchase` 返回 `HTTP/1.1 200 OK`。
