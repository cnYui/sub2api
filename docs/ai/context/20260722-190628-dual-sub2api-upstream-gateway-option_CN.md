# 双 Sub2API 上游网关方案

## 问题

用户提出：是否可以让 GitHub original 最新版 Sub2API 替代 CPA 功能，用它请求上游模型；本地定制版 Sub2API 只负责用户、套餐、流量卡和计费。

## 结论

可行，适合作为 CPA 下线前的过渡方案，但不应作为最终架构长期维护。

推荐命名：

- 外层：`sub2api-billing`，使用当前本地定制版，负责公网入口、用户 Key、订阅、流量卡、`usage_facts`、Dashboard 和支付。
- 内层：`sub2api-upstream`，使用 GitHub original 最新版，负责真实 OpenAI/Codex 凭证、账号调度、OAuth refresh、协议适配、failover、模型能力探测。

链路：

```text
用户
  -> sub2api-billing /v1/*
  -> 请求前预授权与唯一计费来源
  -> sub2api-billing 的一个 OpenAI upstream account
  -> sub2api-upstream /v1/*
  -> 真实 OpenAI/Codex accounts
  -> 上游模型
```

## 必须满足

1. 内层 `sub2api-upstream` 必须只作为内部网关，不对公网用户开放。
2. 外层 `sub2api-billing` 中配置一个指向内层的 OpenAI account：
   - `base_url=https://sub2api-upstream:port/v1` 或同机内网地址。
   - `api_key` 使用内层专用内部 Key，不使用用户 Key。
   - 可继续使用 `pool_mode=true` 或语义等价配置，让外层知道这是聚合上游，不是单个静态 OpenAI Key。
3. 内层必须避免对真实业务做二次商业计费：
   - 内层只记录运维用 usage，不作为用户扣费事实。
   - 内层内部 Key 应绑定内部 admin/service 用户、无限额或零费率组。
   - 外层仍是唯一用户计费事实源。
4. 外层成功响应后仍只写本地 `usage_facts`，不能接入内层 usage event 做扣费。
5. 内层错误要尽量透传 OpenAI 标准错误结构和 `Retry-After`，外层再按本地错误契约渲染。
6. 两层 request_id 必须一致或可关联；至少外层要把 `X-Request-ID` 透给内层，便于跨日志排障。

## 优点

- 比直接移植 original 1152 个提交风险低。
- 能更快使用 original 最新 OpenAI/Codex 协议、账号调度和凭证管理能力。
- CPA 可以先停掉，凭证仍放在一个 Sub2API 项目里，只是放在内层 original Sub2API。
- 外层定制计费逻辑不需要立刻大改。

## 缺点

- 仍是双系统：两个 Sub2API、两套数据库、两套配置、两套日志。
- 外层只看到一个“内层网关账号”，看不到真实上游账号级别的调度事实，Dashboard 会丢真实账号归因。
- 内层 original 的 payment/subscription/usage 表会存在，但语义上不应参与业务计费，需要强约束配置。
- 长期维护容易混淆：两个 Sub2API 版本、迁移和配置漂移会增加排障成本。

## 与 CPA 方案对比

它可以替代 CPA 的“上游账号池和协议适配”角色，但不是纯粹消除中间层，而是把 CPA 换成 original Sub2API。

比 CPA 更好的地方：

- original Sub2API 与本项目同源，后续能力回迁更容易。
- 管理 UI、账号 schema、OpenAI gateway 与本地代码概念相近。
- 能逐步把内层能力移植回外层，最终收敛到单体 Sub2API。

比最终单体差的地方：

- 仍有跨进程错误归因、内部 Key、网络/TLS、双日志问题。
- 真实账号级用量不能天然成为外层计费事实。

## 推荐执行

1. 短期：采用“双 Sub2API”替代 CPA，保持外层计费不变。
2. 中期：从 original 移植 OpenAI/Codex 协议、调度、Agent Identity、usage 解析到外层。
3. 最终：外层直接管理真实账号，停用内层 `sub2api-upstream`。

## 实施边界

- 不建议把用户直接暴露给内层 original Sub2API。
- 不建议让内层承担余额、套餐、流量卡或用户订单。
- 不建议外层消费内层 usage event 做用户扣费；外层必须基于自己的请求前预授权和响应 usage 落 `usage_facts`。
- 上线前必须准备回滚：外层保留 CPA upstream account 或旧内层地址配置，按账号启停切换。
