# yui.web shop-flow 旧发 Key 退役测试失败分析

## 现象

- 精简 Shop 首页后，`node --test test/shop-flow.test.js` 当前为 112 个测试、88 通过、24 失败。
- 之前全量 `npm test` 报 46 个失败，主要来自同一批 `shop-flow` 旧发 Key 成功路径；当前 `test/shop-flow.test.js` 已有人开始引入 `seedHistoricalRedeemedOrderForTest()`，所以失败数已下降。

## 根因

- `SHOP_LEGACY_KEY_ISSUANCE_DISABLED=true` 是新架构下的正确生产行为。
- yui.web 中 6 个旧写接口已被 `rejectLegacyKeyIssuanceWhenDisabled()` 锁死并返回 `410 SHOP_LEGACY_KEY_ISSUANCE_DISABLED`：
  - `POST /api/account/invites/redeem`
  - `POST /api/admin/invites`
  - `POST /api/admin/api-keys`
  - `POST /api/admin/session-invites`
  - `POST /api/admin/session-api-keys`
  - `POST /api/invites/redeem`
- 失败不是 Shop 首页精简导致，也不应通过把 410 改回 201 修复。
- 失败来自测试仍用旧接口创建邀请码、导入 API Key、兑换订单来构造前置数据；这些路径现在应被当作“已退役写路径”。

## 失败分类

### A. 应改写为退役 410 的旧成功路径测试

这些测试过去验证旧发 Key 成功，现在应验证 410 和无新增写入：

- 用户用手机号和邀请码兑换后，从未使用 API key 池分配一个 key 并写入 SQLite 订单
- 登录态邀请码兑换只绑定当前 session 手机号，忽略请求体手机号
- 登录态邀请码兑换成功后同步 API key 到 CLIProxyAPI 入口配置
- CLIProxyAPI 入口配置同步失败时兑换事务回滚
- 旧邀请码和 API key 管理接口仍只接受后端管理员 token
- 管理员 session 可访问 invite console、生成邀请码和导入 API key 池
- 手机号包含字母或位数不对时，兑换接口会拒绝
- 邀请码使用 SQLite 主键精确匹配，大小写归一后只能兑换一次
- 没有未使用 API key 时，邀请码不能被兑换且不会被标记为已使用
- API key 具有唯一性，重复导入会被拒绝

### B. 应保留为历史只读能力，但前置数据不能再走旧写接口

这些测试仍有价值，但应直接 seed 历史订单、API key、订阅或 usage 账本：

- Shop 数据库包含 API key hash 和 usage_events 账本表
- 配置 API key 加密 secret 后，新导入 key 写入密文且 reveal 可解密
- 管理员 usage summary 返回 Shop 和未托管 key 的聚合用量
- 新兑换订单的兑换时间和到期时间使用中国东八区格式存储
- API key 结果页需要账户登录才能访问
- 当前订单接口只返回 result token 绑定的订单
- 登录后的订单查询接口只返回当前 session 手机号的数据
- Account usage summary 只聚合当前登录手机号关联的 token 用量
- Account 模型总览接口使用托管 API key 探测模型并按官方美元价格返回
- Account 模型总览接口会跳过不可用托管 API key 继续探测同账号其他 key
- 内部 API key 状态接口返回未托管、未兑换、有效和过期状态
- 内部 API key 状态查询使用 hash 查找，不依赖明文列
- 历史兑换手机号可以补密码注册并通过 account session 只查看自己的订单

### C. 已存在的正确退役测试

- `Sub2API migration disables legacy invite and API key issuance endpoints` 已覆盖 6 个旧写接口返回 410。
- 后续修复不应删除这个测试，而应把其它旧成功路径测试向它对齐。

## 建议修复方向

1. 保持生产代码 `rejectLegacyKeyIssuanceWhenDisabled()` 行为不变。
2. 把旧写成功路径测试改成退役测试：断言 410、稳定 code、数据库没有新增 invite/order/api_key。
3. 把历史只读测试改为直接 seed 历史数据，不再调用旧写接口：
   - 可继续完善当前 dirty diff 中的 `seedHistoricalRedeemedOrderForTest()`。
   - seed 时必须同时写 `users`、`invite_codes`、`api_keys`、`orders`，必要时补 `account_subscriptions`、`usage_events`、`api_charge_records`。
   - 加密场景用 `apiKeyEncryptionSecret` 生成 ciphertext/nonce，并确认 `toOrder()` 能解出明文用于 reveal/model overview。
4. 测试命名同步新语义：
   - “新兑换订单”改成“历史兑换订单”。
   - “管理员生成邀请码/导入 API key 池成功”改成“旧管理员写接口已退役”。
5. 不把这些测试移动回 yui.web 新发 Key 成功路径；新 Key、套餐、用量事实源应在 Sub2API 测试或端到端验收里覆盖。

## 风险

- 如果简单删除失败测试，会丢掉历史查账、hash 查询、加密 reveal、用户隔离等重要只读能力覆盖。
- 如果把 `SHOP_LEGACY_KEY_ISSUANCE_DISABLED` 在测试默认关掉，会让测试继续鼓励旧双事实源架构。
- 如果 seed 数据不贴近真实迁移数据，会让只读测试假绿，尤其是加密 key、订阅有效性和 usage 汇总。
