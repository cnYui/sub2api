# 99 元订阅套餐（89 USD/日，30 天）改动调研与设计

## 背景

- 现有在售月订阅套餐为 29 / 39 / 59 元。
- 用户新增需求：增加一个 99 元订阅套餐。
- 套餐规则：
  - 价格：99 元
  - 有效期：30 天
  - 每日额度：89 USD
  - 其他行为与现有三档订阅一致

## 当前代码事实

### 1. `/purchase` 购买页的订阅卡片不是写死的

- 用户购买页 [frontend/src/views/user/PaymentView.vue](/Users/wujianxiang/CodeSpace/sub2api/frontend/src/views/user/PaymentView.vue) 通过 `paymentAPI.getCheckoutInfo()` 拉取 `/api/v1/payment/checkout-info`。
- 订阅卡片列表直接遍历 `checkout.plans`。
- 后端 [backend/internal/handler/payment_handler.go](/Users/wujianxiang/CodeSpace/sub2api/backend/internal/handler/payment_handler.go) 的 `GetCheckoutInfo` 从 `PaymentConfigService.ListPlansForSale()` 读取在售订阅计划，再拼上分组信息后返回。

结论：

- 如果只是让 `/purchase` 多出一个 99 元卡片，购买页组件本身通常不需要改代码。
- 关键在于是否已有对应 `group`，以及是否新增对应 `subscription_plan`。

### 2. 套餐展示依赖 `subscription_plans`，额度依赖 `groups`

- 订阅计划表结构见 [backend/ent/schema/subscription_plan.go](/Users/wujianxiang/CodeSpace/sub2api/backend/ent/schema/subscription_plan.go)。
- 分组结构见 [backend/ent/schema/group.go](/Users/wujianxiang/CodeSpace/sub2api/backend/ent/schema/group.go)。
- 套餐价格、名称、有效期、排序来自 `subscription_plans`。
- 每日/每周/月额度、倍率、平台、生图开关、图片价格来自 `groups`。

结论：

- `99 元套餐` 如果要有独立 `89 USD/日` 限额，不能只复制一个 plan 记录指向旧 group。
- 必须有一个新的订阅分组，或把某个现有未售分组改造成 89 USD/日 的订阅分组。

### 3. 支付履约主链路不需要为“新档位”额外写逻辑

- 下单时 [backend/internal/service/payment_order.go](/Users/wujianxiang/CodeSpace/sub2api/backend/internal/service/payment_order.go) 会把 `plan_id`、`subscription_group_id` 和换算后的 `subscription_days` 写入订单。
- 支付完成后的履约走 [backend/internal/service/payment_fulfillment.go](/Users/wujianxiang/CodeSpace/sub2api/backend/internal/service/payment_fulfillment.go)。
- 这里不区分 “29/39/59/99”，只认 plan 和 group。

结论：

- 只要 `group` 和 `subscription_plan` 配置正确，支付、订阅开通、用量扣减、续费链路都能复用现有逻辑。

### 4. 后台已经支持创建订阅分组和套餐

- 分组管理页 [frontend/src/views/admin/GroupsView.vue](/Users/wujianxiang/CodeSpace/sub2api/frontend/src/views/admin/GroupsView.vue) 已支持创建 `subscription_type='subscription'` 的分组，并填写 `daily_limit_usd`。
- 套餐管理页 [frontend/src/views/admin/orders/AdminPaymentPlansView.vue](/Users/wujianxiang/CodeSpace/sub2api/frontend/src/views/admin/orders/AdminPaymentPlansView.vue) 与 [frontend/src/views/admin/orders/PlanEditDialog.vue](/Users/wujianxiang/CodeSpace/sub2api/frontend/src/views/admin/orders/PlanEditDialog.vue) 已支持新建套餐并绑定已有订阅分组。
- 后端已有对应 CRUD：
  - [backend/internal/handler/admin/group_handler.go](/Users/wujianxiang/CodeSpace/sub2api/backend/internal/handler/admin/group_handler.go)
  - [backend/internal/handler/admin/payment_handler.go](/Users/wujianxiang/CodeSpace/sub2api/backend/internal/handler/admin/payment_handler.go)
  - [backend/internal/service/payment_config_plans.go](/Users/wujianxiang/CodeSpace/sub2api/backend/internal/service/payment_config_plans.go)

结论：

- 从功能能力上说，新增 99 元档位不一定需要新增“业务逻辑代码”。
- 更可能是“新增数据配置 + 少量前端静态文案同步”。

## 需要调整的地方

### A. 必须有的运行态改动

#### 方案 A1：新增一个订阅分组 + 新增一个订阅计划

这是推荐方案。

- 新建分组，例如：
  - `name`: `codex-pool-89-usd`
  - `platform`: `openai`
  - `subscription_type`: `subscription`
  - `daily_limit_usd`: `89`
  - `weekly_limit_usd`: `NULL`
  - `monthly_limit_usd`: `NULL`
  - `rate_multiplier`: 与现有三档保持一致
  - `allow_image_generation`: 与现有三档保持一致，当前应为 `true`
  - `image_price_1k/2k/4k`: 与现有三档保持一致
  - 账号池绑定策略：复制现有 OpenAI 订阅池使用的账号来源
- 新建订阅计划，例如：
  - `name`: `99 元订阅池`
  - `group_id`: 新分组 ID
  - `price`: `99`
  - `validity_days`: `30`
  - `validity_unit`: 建议沿用当前实际使用的 `day`
  - `for_sale`: `true`
  - `sort_order`: 放在 59 元后面

优点：

- 套餐隔离清晰，后续售卖、迁移、审计简单。
- 不会污染现有 29/39/59 三档行为。
- 续费和运营脚本更容易识别。

风险：

- 需要同步给新分组绑定账号池，否则用户买到后可能无可用账号。

#### 方案 A2：复用一个现有未售订阅分组，再新建 plan

- 只在确定存在“空闲且未使用”的订阅分组时可考虑。
- 需要先核对该分组是否已被 API Key、订阅记录、运营脚本引用。

缺点：

- 风险高，容易把历史语义搞乱。
- 不符合当前项目已经把 29/39/59 与 `group` 清晰对应的模式。

结论：

- 不推荐。

### B. 可能需要的代码改动

#### B1. 首页默认文案

- 默认首页 [frontend/src/views/HomeView.vue](/Users/wujianxiang/CodeSpace/sub2api/frontend/src/views/HomeView.vue) 三张“套餐介绍”卡片读的是固定 i18n key。
- 中文文案在 [frontend/src/i18n/locales/zh.ts](/Users/wujianxiang/CodeSpace/sub2api/frontend/src/i18n/locales/zh.ts) 中硬编码为 `29 / 39 / 59 元套餐`。
- 对应测试 [frontend/src/views/__tests__/HomeView.spec.ts](/Users/wujianxiang/CodeSpace/sub2api/frontend/src/views/__tests__/HomeView.spec.ts) 也写死三档。

如果希望首页同步展示 99 元档，则需要改：

- `zh.ts` 的首页三张套餐文案结构
- `HomeView.spec.ts`
- 可能还要改 `HomeView.vue`，因为当前布局天然是三列三卡

注意：

- 这里不只是改词，还是信息架构问题。
- 当前首页三区块分别对应 29 / 39 / 59。要加第 4 档，必须决定：
  - 改成四卡
  - 只保留三档精选，99 不上首页
  - 改成“29 起”类概述而不是枚举全部档位

#### B2. 使用方法页面文案

- [frontend/src/views/user/UsageGuideView.vue](/Users/wujianxiang/CodeSpace/sub2api/frontend/src/views/user/UsageGuideView.vue) 里有写死文案：
  - `29/39/59 元套餐已支持生图和图生图`
- 对应测试 [frontend/src/views/user/__tests__/UsageGuideView.spec.ts](/Users/wujianxiang/CodeSpace/sub2api/frontend/src/views/user/__tests__/UsageGuideView.spec.ts) 也断言了这个固定字符串。

如果希望面对用户的教程页也反映 99 元档，则需要改：

- `UsageGuideView.vue`
- `UsageGuideView.spec.ts`

#### B3. 可能涉及的运营脚本/文档

- AGENTS 和记忆文档里当前长期上下文明确写了 29/39/59 三档与 group 的关系。
- 如果 99 元档上线，应补新的 context/result 文档，避免后续继续按“三档”思维操作。
- 现有脚本里有历史分组映射，例如 [scripts/migrate-yuiweb-legacy-api-keys.mjs](/Users/wujianxiang/CodeSpace/sub2api/scripts/migrate-yuiweb-legacy-api-keys.mjs)，虽然当前与新增售卖档位没有直接强耦合，但未来若做数据迁移，需要留意这里不是动态发现。

## 不需要修改的核心业务逻辑

在“30 天订阅 + 每日额度 89 USD + 其他规则完全复用现有三档”的前提下，以下通常不需要新增业务代码：

- 用户购买页订阅卡片渲染逻辑
- 创建订单逻辑
- 支付轮询/回调逻辑
- 订阅开通履约逻辑
- 用量扣费主逻辑
- 图生图/生图接口逻辑

前提是：

- 新分组配置正确
- 新分组已绑定可用账号池
- 新 plan 正确指向新分组

## 需要特别注意的坑

### 1. 不要只加 `subscription_plan` 不加 `group`

- 那样只能多一张卡，不能得到独立的 89 USD/日限额。

### 2. 不要把现有 59 元分组直接改成 89 USD/日

- 会影响已购买 59 元套餐的用户订阅配额。

### 3. 新分组必须复制现有生图能力配置

- 根据当前项目记忆，29/39/59 三档已开启生图能力。
- 如果 99 元新分组漏配 `allow_image_generation` 和对应价格，用户会出现“买了更贵套餐反而不能生图”的回归。

### 4. 首页是否展示 99 元档，需要先做产品决策

- 购买页会自动展示。
- 首页不会自动展示，因为首页不是动态 plan 列表，而是静态营销型说明区。

## 落地方案对比

### 方案 1：只新增后台配置，不改前端代码

- 新增订阅分组
- 新增订阅计划
- 复制账号池和生图配置

结果：

- `/purchase` 会自动出现 99 元档，可直接购买。
- 首页、教程页仍显示 29/39/59。

适用：

- 目标是先上线售卖能力，最小风险。

### 方案 2：后台配置 + 同步用户可见静态文案

- 包含方案 1
- 再修改首页和使用方法文案及测试

结果：

- 购买链路和站点对外文案一致。

适用：

- 目标是正式对外售卖，避免用户看到“站内可买 99，但首页没写”。

### 方案 3：把首页套餐区改造成动态订阅计划列表

- 把首页三张固定文案卡改为读后端套餐接口

优点：

- 后续再加档位无需重复改首页文案。

缺点：

- 改动面明显更大，涉及首页信息架构和文案策略，不适合这次只加一个档位时顺手做。

结论：

- 当前不推荐。

## 推荐方案

推荐采用“方案 2”：

1. 新增一个独立订阅分组，日额度 `89 USD`，其他能力对齐现有三档。
2. 新增一个 `99 元订阅池` 的 `subscription_plan` 指向该分组。
3. 同步更新首页和使用方法中的静态文案。
4. 补前端测试，避免后续又被改回“三档”。

## 预计需要动到的文件

如果只做配置，不改代码：

- 运行态数据库/后台配置，不必改仓库代码文件。

如果做“配置 + 文案同步”：

- [frontend/src/i18n/locales/zh.ts](/Users/wujianxiang/CodeSpace/sub2api/frontend/src/i18n/locales/zh.ts)
- [frontend/src/views/HomeView.vue](/Users/wujianxiang/CodeSpace/sub2api/frontend/src/views/HomeView.vue) 可能需要
- [frontend/src/views/__tests__/HomeView.spec.ts](/Users/wujianxiang/CodeSpace/sub2api/frontend/src/views/__tests__/HomeView.spec.ts)
- [frontend/src/views/user/UsageGuideView.vue](/Users/wujianxiang/CodeSpace/sub2api/frontend/src/views/user/UsageGuideView.vue)
- [frontend/src/views/user/__tests__/UsageGuideView.spec.ts](/Users/wujianxiang/CodeSpace/sub2api/frontend/src/views/user/__tests__/UsageGuideView.spec.ts)

## 当前建议

- 如果你只是问“当前项目里要改哪些前后端代码”，严格来说：
  - 核心前后端业务代码大概率都不用改
  - 必须新增运行态 `group + subscription_plan` 配置
  - 如果要站内文案一致，则补少量前端静态页面和测试

- 真正高风险点不在代码，而在：
  - 新分组账号池是否绑定正确
  - 生图能力是否完整复制
  - 首页是否要同步展示第 4 档
