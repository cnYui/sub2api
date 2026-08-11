# 并发容量调研：60~300 并发能否稳定支撑

## 背景

用户场景：接入的上游全部是「别人的中转站」，每个渠道只配了一个 API key。
用户从 Sub2API 前端计费 → 后端 → 请求上游中转站。上游中转站并发充足、账号多。
预期负载：60 人在两天内持续使用，峰值并发估计 300。

核心疑问：
1. 本地服务能否稳定支撑、用户不卡顿？
2. 一个渠道配 2~5 个 key（都填别人中转站）对并发有无缓解？
3. 调高并发后有无隐藏限制？

## 并发模型：两道闸门（Redis 有序集合槽位）

每个请求必须**同时**抢到两个槽位才能发往上游（`backend/internal/handler/gateway_helper.go`）：

- **用户槽位** `concurrency:user:{userID}`：限制单用户同时在飞请求数
- **账号槽位** `concurrency:account:{accountID}`：限制单个上游账号同时在飞请求数

两者按 ID **独立命名**——这是「多账号=多独立并发池」的根据。

## 关键数值（全部来自代码确认）

| 参数 | 默认值 | 出处 |
|---|---|---|
| **账号并发 concurrency** | **3** | `backend/ent/schema/account.go:100` `Default(3)` |
| 用户并发 concurrency | 20 | `backend/ent/schema/user.go:56` `Default(20)` |
| 等待队列额外槽位 | 20 | `concurrency_service.go:221` `defaultExtraWaitSlots=20` |
| 用户队列容量 | 用户并发+20 | `CalculateMaxWait = userConcurrency + 20` |
| 抢槽最大等待 | 30 秒 | `gateway_helper.go:95` `maxConcurrencyWait=30s` |
| 退避 | 100ms→2s 指数+抖动 | `gateway_helper.go:429` |
| 槽位 TTL | 30 分钟 | `gateway.concurrency_slot_ttl_minutes=30` |

行为差异：
- 用户闸抢不到 → 进队列（容量=并发+20），满则拒绝 `WaitQueueFullError`，否则等 ≤30s
- 账号闸抢不到 → 直接等（**账号侧无队列容量上限**），仅 30s 超时，超时抛 `ConcurrencyError`

## 瓶颈定位：账号并发默认 3

用户把所有 60~300 人路由到**同一个账号**，该账号并发=默认 3。
→ 任意时刻只允许 **3** 个请求真正打到上游，第 4 个起全部排队。

用运维页实测延迟推算（平均 27s / P50 16s）：
- 单账号吞吐 ≈ 3 ÷ 27s ≈ **每分钟 6~11 个请求**
- 60 人只要同时在飞十几个就远超 3，后续全卡进 30s 队列
- 等不到即超时报错（对应运维页「异常数 11」）

**连带解释了健康分低**：请求时长 P99 137s、TTFT P99 99s，很大一部分是**排队等这 3 个槽位的时间**，不是上游慢。瓶颈在本地 `concurrency=3` 这道闸，不在上游。

## 隐藏限制核查：调高并发后连接池是否够（结论：够）

走别人中转站 = HTTP 上游，所有请求打到少数几个 host（中转站域名）。相关默认值：

| 参数 | 默认值 | 出处 | 是否瓶颈 |
|---|---|---|---|
| `gateway.max_conns_per_host` | **1024** | config.go:2332 | 关键闸；300 连接对单 host 够，甚至可上 2400+ |
| `gateway.max_idle_conns` | 2560 | config.go:2330 | 空闲连接总数，够 |
| `gateway.max_idle_conns_per_host` | 120 | config.go:2331 | 仅影响复用；活跃超 120 照建，不限流 |
| `gateway.max_upstream_clients` | 5000 | config.go:2334 | 客户端缓存数，够 |
| `gateway.concurrency_slot_ttl_minutes` | 30 | config.go:2336 | 请求 P99 137s ≪ 30min，槽位不会提前过期 |
| `database.max_idle_conns` | 128 | config.go:2045 | DB 仅计费时短占，够 |

动态连接池 `DynamicMaxConnsByAccountConcurrencyEnabled=true`、factor=1.0、
`effective=ceil(concurrency*1.0)`，但 `max_conns_per_account=128` 仅对 **OpenAI WS 池**生效。
走 HTTP 中转站不经 WS，不受此限；若未来对 OpenAI 账号把并发调到 >128，需同步调 `max_conns_per_account`。

资源侧：300 在飞连接对 Go 仅几百协程（当前 205，警告线 8000）；CPU 1.3%、内存 0.6%。机器完全带得动。

## 结论与建议

**当前扛不住，但改一个数即可解决。**

### 首选：调高单账号 concurrency（最省事）
- 60 人日常（真实在飞 10~25）：账号并发设 **30~50**
- 峰值 300「同时在飞」：设 **≥300**；「300 在线但错峰」：设 **80~120**
- 这是解排队问题的**唯一关键动作**：账号闸（默认 3）才是瓶颈

### 澄清：`DefaultConcurrency`（后台「默认并发量」）与本瓶颈无关
- 作用：**新用户注册时**赋给该用户的并发上限，作用在**用户闸** `concurrency:user:{userID}`
  （`auth_service.go:808 resolveSignupGrantPlan` → `GetDefaultConcurrency`，DB user 表默认 20）
- **不影响已存在用户**（老用户并发在注册时已写库，需单独/批量改）
- **与账号并发（默认 3）是两回事**：账号并发=单账号能被同时打多少；用户并发=单用户能同时发多少
- 本场景瓶颈是账号闸，`DefaultConcurrency`（20）通常无需动；除非有用户跑批量脚本需单用户高并发

### 加 key 的价值（次选/补充）
账号槽位按 accountID 独立，加 N 个账号 = 账号级天花板 `N × concurrency`，线性放大排队缓解。
但要区分：
- **填不同上游 key**（上游是不同账号）→ 本地+上游双层放开，真·翻倍
- **同一 key 复制成多账号** → 仅放开本地槽位；上游若按 key 限流，只是把「本地排队」变「上游被限流」

上游是稳定大池、不按 key 卡时 → **直接调高单账号并发更简单**。
加多账号的独有收益：①上游按 key 限流时可绕开；②故障转移（一个 key 挂了自动切）。

### 别忘了用户闸
账号池放大后，若单用户并发（默认 20）不够（如批量脚本），需单独调高该用户并发。

### 若调高 max_conns_per_host 到 2400+
流式/HTTP1.1 场景每连接占一个 host 连接，若单账号并发 + 多账号叠加逼近 1024，把 `gateway.max_conns_per_host` 调到 2400+（注释已建议）。

## 附带发现（与本次相关）
运维页健康分算法（`ops_health_score.go`）对 LLM 流式场景校准偏严：
TTFT 3s 归零不现实、无最小样本保护、后台 job 15min 过期误判。
详见同目录健康分调研（若后续单独落档）。当前低分很大程度是账号并发=3 导致排队，放开并发后延迟指标会显著改善。
