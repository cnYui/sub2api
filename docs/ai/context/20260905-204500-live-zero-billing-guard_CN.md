# Live(WebRTC) 零计费缺陷——调研与护栏落地

- 时间：2026-09-05 20:45（+09）
- 触发：`20260905-203548-*` 核验最终倍率时顺带发现
- 处置：**加硬护栏 + 记为开启前置，不实现完整计费**
- 生产影响：**零**（上线后可观测行为无任何变化）

> 本仓库是公开仓库。运维敏感值一律用 `${变量名}` 占位。

---

## 1. 缺陷本体

`service/openai_live.go` 的 `finalizeLiveCall` 用 `writeUsageLogBestEffort` 自行写一条
UsageLog：`TotalCost=0`、`ActualCost=0`、`RateMultiplier` 硬编码 `1`、不设任何 token 字段，
**完全绕过 `recordUsageCore` 与 `applyUsageBilling`**。该文件 `applyUsageBilling` 调用数为 0；
全仓非测试调用点只有 `gateway_usage_billing.go` 与 `openai_gateway_usage.go` 两处。

源码 `TODO(billing)` 原文已承认此事，零值语义被
`TestFinalizeLiveCallIsIdempotentAndWritesZeroUsage` 用 `require.Zero` 钉死。

**失败形状**：准入只校验 `balance >= 0`，而 Live 恒零扣费 → 欠费闸永不触发 →
余额正好 `$0.01` 的用户可无限次开满 `liveMaxSessionDuration`（默认 1 小时）的会话。
方向与坑 12 相反：**这次是平台单向净亏**，且 `usage_logs.actual_cost` 落库就是 0，
事后无法重建、无法追回。

---

## 2. 当前敞口是「结构性的零」，不只是配置为零

上一轮只确认了「17 个分组 `allow_live` 全 false、`usage_logs` 无 `request_type=5`」。
本轮查出更强的一层：

- `internal/platform/liveattestation/attestation_unsupported.go` 是 `//go:build !darwin`，
  `Check` 与 `Generate` 均返回 `ErrUnsupportedPlatform`
  （错误文案原文：live attestation 只在 macOS 上支持）。
- `prepareLiveAttestation` 在 `CreateLiveCall` 里位于 **`ValidateLiveCallRequest` →
  `liveStore` → `liveConcurrencyCache` 之后、选账号之前**，是一道硬闸。

生产镜像是 `GOOS=linux` / Alpine，所以**即使把 `allow_live` 全部打开，也会在建会话
第一步就失败**，根本走不到上游。

**但这是平台的偶然属性，不是计费保护。** 改成 macOS 部署、或给非 darwin 补一个
provider，这道闸就没了。所以不能拿它当「Live 安全」的理由——已写进 CLAUDE.md。

---

## 3. 为什么不现在实现完整计费

三条独立调研线一致指向「不划算」：

| 因素 | 结论 |
| --- | --- |
| 上游能不能抄 | **不能**。`Wei-Shaw/sub2api` 的 `finalizeLiveCall` 与本 fork 逐字节相同，`TODO(billing)` 原文仍在。领先 1095 个提交、5 周多仍未实现，同步上游修不了它 |
| 数据够不够 | 时长够（`DurationMs` 已算已写、`duration_ms` 列自迁移 001 就存在），归属字段齐全；**token 不够**——媒体不过服务端，sideband 是否回传 usage 代码里 0 处解析、无 fixture 无 trace |
| 定价有没有 | **完全没有**。仓库里没有任何音频/时长价格。按坑 10/坑 12，接上管道而无价 = 依然记零成本放行，是装饰性修复 |
| 单价谁定 | **业务决策**，需管理员拍板。上游对一次 Live 会话实际收多少不在公开价目表上 |
| 代价 | 按时长端到端约 1-2 天（新迁移号 213、`CalculateLiveCost`、倍率快照、finalize 接线、测试、管理端表单）；走 token 路线再加 3-5 天 |

为一个**在生产平台上跑不起来**、且单价还没定的功能做 1-2 天改造，性价比不成立。
而且 `openai_live.go` 是上游高频改动文件，本地大改等于给以后每次 sync 埋冲突点。

---

## 4. 本次实际改了什么

### ① 硬护栏（`handler/openai_live.go`）

在 `liveEnabledForAPIKey` 前加常量与早退：

```go
const liveBillingImplemented = false

func liveEnabledForAPIKey(apiKey *service.APIKey) bool {
    if !liveBillingImplemented {
        return false
    }
    return apiKey != nil && ... // 原样保留
}
```

**为什么闸设在请求路径而不是管理端**：`groups.allow_live` 有创建 / 编辑 / 复制
三条写入路径，堵管理端要同时改三处、漏一处就漏；而且它管不住「库里已经是 true」
——本项目恰恰有直连数据库改数据的既有作业习惯（坑 22）。请求路径上一处早退即覆盖
全部 Live 路由，对配置层发生了什么完全免疫。

**不选「只打 WARN 日志」**：那正是坑 12 点名的 `pricing_missing_record_zero_cost`
同款反面模式——日志在 `finalizeLiveCall` 里打，此时这一小时已经烧完了，是事后取证不是护栏。

### ② 更正一条假注释（`service/openai_live.go`）

原注释：「Live 按通话时长计费，不属于 token 利润门的语义范围：显式豁免……」
**这个前提不成立**，同文件的 `TODO(billing)` 明说不计费。留着它，下一个人会相信
计费已存在——与坑 16「测错字符串比没测更误导」同类伤害。

已改为说明豁免本身仍合理（利润门按 token 语义设计），但**不要把它当作
「已按时长计费」的证据**，并注明接入时长计费时需重新确认豁免是否继续成立。
`WithOpenAIProfitControlSuppressed` 的行为**未改动**。

### ③ 测试断言（`handler/openai_live_test.go`）

`TestLiveEnabledForAPIKey` 最后一条 `require.True` 翻成 `require.False`，
注释写明接通计费后翻回。

### 验证

```
go build ./internal/handler/... ./internal/service/...   → RC=0
go vet ./internal/handler/                               → RC=0
go test ./internal/handler/ -run 'TestLive...'           → 3/3 PASS
```

按坑 19，`internal/service` 的 `unit` 标签套件当前编译不过，只能跑定向用例。

---

## 5. 生产影响与回滚

**上线后可观测行为零变化**：17 个分组 `allow_live` 全 false，现在走的就是同一个 403
分支，错误文案 `"Live is not enabled for this group"` 未变；且 Linux 下 attestation
本来就先失败。护栏只在有人真去翻开关时才咬人。

回滚：删掉 `liveBillingImplemented` 常量与那三行早退，测试断言翻回 `require.True`。

**改动只在工作区，未提交、未部署。**

---

## 6. 接计费时的执行顺序（若将来要开）

1. **先落价格**，再接管道，最后才开 `allow_live`。顺序反了的中间窗口少收无法追回。
2. 探针（约 1 小时）：临时开一个一次性分组，在 `openai_live.go` 两个已有的 gjson
   解析点打一条 frame type 日志，跑一次短通话，确认 sideband 是否回传 usage。
   有 → 可走 token 路线；无 → 按时长是唯一可行信号。
3. 倍率快照必须在 `CreateLiveCall` 时写进 `LiveCallRecord`（连同 Redis hash 的读写），
   **不能在 finalize 时重新解析**——finalize 可能晚 1 小时，那时分组倍率可能已改，
   快照才让审计可复现（与坑 25 同一道理）。
4. 中途无余额检查是独立敞口：全程只有创建时 `CheckBillingEligibility` 和 sideband
   接入时 `CheckFreshBalanceDebt` 两次。即使按时长计费，用户也能先烧满一小时再结算。
   低成本缓解：复用已有的 20 秒 lease 刷新 tick 做增量结算或负余额中止。
5. 改 `TestFinalizeLiveCallIsIdempotentAndWritesZeroUsage` 时**必须注入非 nil 的计费仓库**
   ——现有构造里 `billingService`/`usageBillingRepo` 都是 nil，接线代码会走 nil 兜底
   继续返回零成本，构成坑 14 同款假阳性。
