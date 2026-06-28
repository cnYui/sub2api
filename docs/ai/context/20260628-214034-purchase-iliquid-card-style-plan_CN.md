# /purchase 购买卡片 iLiquid 风格替换计划

## 背景

- 用户提供 `/Users/wujianxiang/Downloads/stitch_wwdc25/screen.png` 与同目录 `code.html`，要求只学习截图中 8 个商品展示卡片的代码风格，把用户前端 `/purchase` 的购买卡片替换为相同风格。
- 本轮只改前端展示与相关前端测试，不改后端、支付 API、订单流、运行态配置。

## 参考卡片要点

- 黑色卡片表面，约 16px 圆角，细白色半透明描边，顶部描边更亮。
- 卡片内部为纵向布局：`PLAN` 小标签与分割线、标题、`Price` 价格行、三行规格信息、底部白色胶囊按钮。
- hover 时卡片轻微上浮，黑色阴影加深，按钮从白底黑字切到透明白字。
- 截图为桌面 4 列网格，卡片最小高度约 380px，列间距约 48px。

## 实现选择

推荐方案：改造现有 `SubscriptionPlanCard.vue` 和 `TrafficPackCard.vue` 的模板/样式，让两类购买商品保持同一视觉系统；`PaymentView.vue` 只调整列表容器宽度和网格间距。

不选整页复刻方案：截图中的 iLiquid 顶部导航、hero 和 footer 不是用户要求的购买页卡片样式，搬入会影响现有 AppLayout 和支付页信息架构。

不选新增卡片组件方案：当前订阅和流量包已有独立组件与测试，直接重构更小，能保留下单事件和 feeRate 计算。

## 数据映射

- 订阅卡标题：`月度订阅-时间 {validity_days}天`。
- 订阅价格：展示含手续费应付价，例如 `¥39.39`。
- 订阅规格：`日限额`、`刷新时间`、`手续费详情`，用实际 `daily_limit_usd`、`validity_days`、`price` 和 `feeRate` 生成。
- 流量包标题：`一次性流量包-有效期 {validity_days}天`。
- 流量包规格：`可用额度`、`可用范围`、`手续费详情`，用实际 `credit_usd`、`platform`、`price` 和 `feeRate` 生成。

## 测试计划

- 先更新 `SubscriptionPlanCard.spec.ts` 与 `TrafficPackCard.spec.ts`，断言新卡片的黑色玻璃表面、`PLAN/Price`、规格行、胶囊按钮和实际金额。
- 先运行相关测试确认失败，再改组件，最后运行同一批测试和必要的购买页测试。

