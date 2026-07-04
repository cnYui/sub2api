# 10 USD 流量卡“我的订阅”显示与可用性排查结果

## 结论

- 当前前端 `我的订阅` 页面不是完全没有渲染流量卡：页面会调用 `/api/v1/payment/checkout-info`，当 `traffic_credit_summary.total_remaining_usd > 0` 时展示一张 `GPT 流量包` 汇总卡。
- 页面不会展示每张流量卡明细，因此用户看不到“两张 10 USD 卡”逐条记录是当前产品实现限制，不是渲染失败。
- 当前公网 `api.aaccx.pw` 实际由 nginx 反代到 `127.0.0.1:18084`，对应 `sub2api-candidate-postgres`；不要把 `sub2api-postgres/18080` 的数据误认为公网数据。
- 在当前公网 18084 数据层中：
  - `1038686518@qq.com` 没有可用 OpenAI 流量卡，只有 active 订阅。
  - `2799523972@qq.com` 有 2 张旧 OpenAI 流量卡，合计剩余约 `14.995929 USD`，没有 2026-07-02 批次新发的 10 USD 卡。
  - `2799523972@qq.com` 当前用户侧 API Key 列表为空，旧 Key 已软删除；`/groups/available` 也为空，因此该用户当前无法自行创建可调用 Key。
- 当前公网流量卡计费链路可用：用无 active 订阅但有 active Key 的测试用户请求 `/v1/responses` 成功，并从 10 USD 流量卡扣到 `9.995934 USD`，新增扣费流水。

## 前端链路

- 页面文件：`frontend/src/views/user/SubscriptionsView.vue`
- 数据来源：
  - `/api/v1/subscriptions` 返回订阅列表。
  - `/api/v1/payment/checkout-info` 返回 `traffic_credit_summary`。
- 渲染规则：
  - `traffic_credit_summary.total_remaining_usd > 0` 时展示 `GPT 流量包`。
  - 当前只展示汇总：`总计 $0.00 / $xx.xx`。
  - 当前到期文案硬编码为 `剩余 365 天`，未展示每张卡的独立到期时间。

## 真实浏览器验证

- 使用 `2799523972@qq.com / 123123` 登录 `https://api.aaccx.pw/login` 成功。
- 打开 `https://api.aaccx.pw/subscriptions` 后页面显示：
  - `GPT 流量包`
  - `有效`
  - `到期时间 剩余 365 天`
  - `总计 $0.00 / $15.00`
- 页面显示 `$15.00` 是因为前端对 `14.995929` 使用 `toFixed(2)` 四舍五入。

## 数据库与 API 验证

### 当前公网 18084 / candidate

- nginx 配置中 `api.aaccx.pw` 和 `aaccx.pw` 的 Sub2API API 路由均反代 `127.0.0.1:18084`。
- `2799523972@qq.com`：
  - `user_traffic_credits` 有 2 张可用卡：
    - 10 USD 卡剩余约 `9.995929`
    - 5 USD 卡剩余 `5.000000`
  - `/api/v1/payment/checkout-info` 返回 `traffic_credit_summary.total_remaining_usd=14.995929`
  - `/api/v1/subscriptions` 返回空数组
  - `/api/v1/keys` 返回空列表
  - `/api/v1/groups/available` 返回空数组
- `1038686518@qq.com`：
  - `user_traffic_credits` 当前无可用 OpenAI 流量卡
  - 有 active 订阅 `codex-pool-29-usd`

### 本地 18080 / sub2api

- `sub2api-postgres` 中仍可看到 2026-07-02 批次发放结果：
  - `1038686518@qq.com` 有 1 张 10 USD 卡
  - `2799523972@qq.com` 有 3 张卡，合计约 `24.991713 USD`
- 但当前公网请求没有使用这套数据层。

## 可用性判断

- 流量卡计费能力本身正常。
- 若用户有非删除 active API Key，且订阅不可用或超限，请求可以消耗流量卡。
- `2799523972@qq.com` 当前不能直接调用的原因是没有可用 API Key，也没有可绑定分组，不是流量卡扣费逻辑失效。
- `1038686518@qq.com` 在当前公网 18084 看不到 2026-07-02 10 USD 卡，是因为这张卡不在当前公网数据层中，不是前端漏渲染。

## 后续建议

1. 先确认当前公网应以 18084 candidate 还是 18080 sub2api 为准。
2. 如果 18084 是正式公网数据层，需要将 2026-07-02 的 10 USD 发放动作在 18084 上按批次补齐或从正确备份迁移。
3. 如果 18080 才是正式公网数据层，需要检查并修正 nginx 当前指向 18084 的配置。
4. 若希望用户能看到“两张 10 USD 卡”明细，应新增后端明细接口或扩展 `traffic_credit_summary`，再改前端展示；当前接口只支持汇总。

## 敏感信息

- 本次结果不记录完整 API Key、access token、内部 token、SMTP 密码或 HMAC secret。
