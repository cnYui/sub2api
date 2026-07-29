# OpenAI 请求前计费授权、原子 Hold 与本地校准最终设计

时间：2026-07-28 16:24:44 +09:00

## 1. 目标

本次同时完成 P0 与 P1：

- 修复已派发失败请求长期冻结流量卡额度的问题。
- 所有指定 OpenAI 入口在调用上游前完成不可变资金授权。
- 套餐与流量卡都采用数据库原子 hold，阻止多个 API Key、高并发请求穿透额度。
- 资金来源固定为套餐、流量卡、402；一次请求只能使用一个来源。
- 账户余额退出 OpenAI 模型计费，但购买、充值、退款业务不变。
- 请求前预算使用可执行上界，结算使用上游真实 usage，并在隔离环境持续校准公式。

本次不处理历史 74 条冻结 reservation 和 2902 条 debt，不切换公网流量，不执行公网数据库迁移。

## 2. 隔离开发环境

开发和真实请求验证使用完整隔离双层环境：

```text
用户授权的测试 API Key
  -> 外层候选 Sub2API :18081
       -> 外层 PostgreSQL 克隆
       -> 外层独立空 Redis
       -> 内层候选 Sub2API :18087
            -> 内层 PostgreSQL 克隆
            -> 内层独立空 Redis
            -> 真实 OpenAI OAuth 账号池
```

- `18080/18086` 继续承载现有公网链路，全程不重启、不迁移、不写入。
- `18081` 从当前项目创建 `codex/` 分支和独立 worktree，承载本次计费修改。
- `18087` 固定当前 `sub2api-upstream-latest` 版本，只负责真实模型转发和账号调度，不重复实现外层用户计费。
- 分别备份外层和内层 PostgreSQL，验证 dump 可读后恢复到独立候选实例。
- Redis 不复制，避免继承公网缓存、并发计数、锁和临时状态。
- 外层克隆库中的内层上游地址只指向候选 `18087`。
- 候选容器、网络、数据目录和 volume 使用独立名称，仅绑定 `127.0.0.1`，不加入公网 Nginx。
- 除真实 OpenAI 请求外，关闭邮件、支付回调、外部备份上传等副作用；保留 usage worker 和 reconciliation worker。
- 数据库 dump 和测试密钥不进入 Git。用户提供的测试 Key 只通过测试进程内变量注入，不写入文档、配置、代码或日志。

## 3. 请求预算和动态上限

请求前 hold 不是最终扣费。最终只按真实 usage 结算，并立即释放未使用金额。

```text
F = 文本输入成本
  + 图片输入成本
  + PDF 提取文本成本
  + PDF 页面视觉成本

V = 文字输出 Token 上界成本
  + GPT Image 2 图片输出上界成本

B = F + V
```

全局单请求硬上限为 `2 USD`，但不固定冻结 2 USD。授权事务直接锁定资金来源并读取真实可用额度：

```text
C_effective = min(2 USD, 当前资金来源的原子可用额度)
```

不采用 `2 -> 1.5 -> 1 -> 0.5` 的循环试探，也不设置 0.5 USD 死区。用户最后无法使用的额度只可能小于最小可执行请求成本，不人为保留固定余额。

完整预算优先尝试套餐，再尝试流量卡。两者都不能覆盖完整预算时，仅允许收紧可调部分后再次按套餐、流量卡顺序授权：

- 未显式指定输出上限：按 `C_effective - F` 计算并写入实际上游请求的输出上限，最低 256 Token。
- 用户显式指定输出上限：不静默修改，预算不足返回 402。
- 多图请求：先尝试完整数量；不足时减少到资金可覆盖数量，至少 1 张。
- 不可缩减输入成本超过 `C_effective`：请求前 402，不访问上游。

资金来源禁止套餐与流量卡混合。一次流量卡 authorization 可以按到期顺序占用多张流量卡，仍属于唯一的 `traffic_credit` 来源。

## 4. 文本、图片和 PDF 预算

### 4.1 文本

```text
文本输入 Token
  = 本地 tokenizer 统计
  + messages/tools/schema 等协议结构开销

文本输入成本 = Token × 价格快照输入单价
文本输出成本 = 实际发送到上游的输出 Token 上限 × 输出单价
```

- 不使用固定 10% 安全系数。
- 协议开销按模型、入口、消息数量和工具结构单独计算并版本化校准。
- 未显式指定输出上限时，根据克隆库历史 usage 分布形成保守默认值，并将该值真实写入上游参数，使输出成本具备可执行边界。
- Embeddings 只有输入成本，仍经过相同授权。

### 4.2 图片输入和图生图

每张图片按对应模型的官方尺寸缩放、分块、`detail` 和 Token 规则计算：

```text
图片输入成本
  = image_input_tokens(width, height, detail, model)
  × 图片输入单价
```

尺寸、`detail`、数量和模型版本必须进入预算。不能因为历史 `image_input_tokens/image_input_cost` 为 0 就跳过。

图生图同时计算上传图片的输入成本和 GPT Image 2 的输出成本。

### 4.3 PDF

按照 OpenAI 的 PDF 输入语义计算：

```text
PDF 成本
  = 提取文本 Token 成本
  + 每一页页面图像的视觉 Token 成本
```

请求前解析可访问的 PDF 内容，获得文本、页数和页面尺寸。无法读取或无法可靠解析的附件不得按零成本放行。视频本次不支持。

### 4.4 生图和混合请求

```text
独立生图
  = 图片输入或编辑成本
  + 可生成图片数量 × GPT Image 2 输出成本

文字与生图混合
  = 外层文字模型成本
  + GPT Image 2 输入与输出成本
```

文字和图片使用同一个 authorization；工具层不得生成超过授权数量的图片。

## 5. 价格快照

- GPT-5.5：官方基础价格的 2 倍。
- GPT-5.6：官方基础价格的 2.5 倍。
- GPT Image 2：官方基础价格的 2 倍。
- 倍率在统一价格快照中只应用一次。
- 预算和结算使用同一份快照，业务路径不得再次乘倍率。

当前 ChatGPT OAuth 上游不支持官方 `POST /v1/responses/input_tokens`，因此本次内部预授权统一使用本地预算器。公开 `/v1/responses/input_tokens` 暂时返回明确的 501，不创建 hold、不计费、不转发。

## 6. 统一 Billing Authorization

将现有流量卡专用 reservation 演进为通用 `billing_authorizations`，迁移并保留现有记录。核心字段包括：

```text
request_id + api_key_id
user_id
billing_source
entitlement_period_id
reserved_usd / settled_usd / debt_usd
pricing_snapshot
estimate_breakdown
estimator_version
status / expires_at / last_error
```

流量卡明细子表继续保留，用于记录一次 authorization 在每张流量卡上的占用、结算和释放。它支持使用多个小额尾款、按过期时间消费和完整审计；只有未来明确限制为“一次请求只使用一张流量卡”时才删除。

usage fact 必须引用 authorization。没有 authorization 的 OpenAI 成本属于内部一致性错误，禁止在响应后重新选择套餐、流量卡或账户余额。

## 7. 套餐原子 Hold

授权事务锁定套餐权益周期，并同时检查日、周和周期总额度：

```text
对应维度已结算用量
+ reserved/dispatched/unknown 状态的活动 hold
+ 本次预算
<= 对应额度上限
```

多个 API Key、每个 Key 5 并发时，所有请求在数据库锁下串行判断可用额度。结算事务同时写 durable usage fact、实际费用并把 authorization 转为 settled，避免已结算用量和活动 hold 重复计算或短暂漏算。

## 8. 状态机与 Reconciliation

```text
reserved
  -> released       派发前失败
  -> dispatched
       -> settled   获得完整或部分真实 usage
       -> released  明确证明上游未计费
       -> unknown   无法确认是否到达或计费
            -> settled
            -> released
            -> suspense
       -> debt      实际费用超过 hold
```

- 所有状态转换检查受影响行数，非法转换立即报警。
- transport、DNS、TCP、TLS 等不确定错误进入 unknown。
- 上游错误只要带 billable usage 就必须结算。
- 流式或客户端中断后继续获取终态；仍不确定则 unknown。
- unknown 每 30 秒 reconciliation，最多保留 5 分钟。
- 五分钟仍无法确认时转平台 suspense，释放用户额度，潜在成本进入平台待核销账。
- 实际费用超过 hold 时不丢弃已完成响应，不切换来源；差额记入同一 authorization 的 debt，后续请求在债务处理前返回 402。
- failover 和重试复用同一个 authorization，不重复 hold。
- WebSocket 每个 turn 创建独立 authorization。

## 9. 统一入口

以下入口请求上游前必须调用同一个授权服务：

- `/v1/responses`
- `/v1/chat/completions`
- OpenAI 分组 `/v1/messages`
- `/v1/embeddings`
- `/v1/images/generations`
- `/v1/images/edits`
- Responses WebSocket 每个 turn
- 自动透传、raw fallback 和 failover 路径

账户余额逻辑从这些入口和 settlement 中全部移除。

## 10. TDD 和校准闭环

实施顺序固定为：

1. 预算器单元测试：文本、tools/schema、图片、PDF、图生图、多图、混合请求、倍率和动态上限。
2. PostgreSQL 集成测试：套餐和流量卡原子 hold、状态转换和结算。
3. 15 并发测试：3 个 API Key，每个 Key 5 并发；只有成功 hold 的请求能进入 mock upstream。
4. 故障注入：HTTP 4xx/5xx、带 usage/无 usage 的失败、transport 错误、SSE 中断、客户端断开、failover、unknown 和 suspense。
5. 克隆库历史数据分析：按模型、入口、上下文长度、图片数量统计实际 Token 和费用分布。
6. 使用用户授权 Key 对 `18081 -> 18087 -> OpenAI` 发真实请求，对比预算、hold、真实 usage、实际扣费和释放金额。

每个请求保存 `estimator_version`、预算分解、价格快照和实际 usage。调整公式时先新增失败测试、提升版本，再重跑完整矩阵；禁止按用户、API Key 或单条请求特判。

估算目标不是强求 hold 与 actual 完全相等。模型提前结束会产生正常剩余 hold，必须及时释放。硬性要求是：不可缩减输入不漏算、输出受实际参数限制、所有能取得完整 usage 的测试中 `actual_cost > hold` 为 0；无法取得完整 usage 的故障请求必须进入 unknown/reconciliation，不能伪装成估算成功。

## 11. 验收条件

- 所有目标 OpenAI 入口都有请求前唯一 authorization。
- 套餐和流量卡在 15 并发下不穿透额度。
- 套餐、流量卡和余额之间不发生混合或响应后改选。
- 所有能取得完整 usage 的测试中 `actual_cost > hold` 为 0；usage 不完整的请求进入 unknown/reconciliation。
- 预算不足时上游调用次数为 0。
- 所有 hold 最终进入 settled、released、受控 unknown/suspense 或明确 debt，不永久冻结。
- 图片和 PDF 输入始终进入预算。
- 2 USD 只作为全局最高限额，资金不足时按真实可用额度精确缩减，不保留固定 0.5 USD 死区。
- 公网 `18080/18086` 容器、数据库、Redis、Nginx 和请求链路全程不变。
- 本轮最终只交付分支、migration、代码、测试、候选环境和校准报告，不部署公网。
