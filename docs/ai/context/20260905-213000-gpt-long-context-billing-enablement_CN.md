# GPT 渠道开启 >272K 长上下文额外计费

- 时间：2026-09-05 20:40 ~ 21:20（+09）
- 环境：生产 `https://aaccx.pw`
- 操作方式：浏览器既有管理员会话调管理 API（未输入任何凭证），未直连数据库
- 管理员指令：「开启超长上下文额外计费，先把 gpt 的价格修改，其他模型先不动」

---

## 1. 改动结果

对 6 个 GPT 上游账号设 `extra.openai_long_context_billing_enabled = true`：

| 账号 | 名称 | 分组 | 分组倍率 | 上游 | PUT | flag |
| --- | --- | --- | --- | --- | --- | --- |
| `#1128` | GPT模型官方0.15倍价格 | GPT0.15倍率(日常1) | 0.15 | `api.ai-genesis.app` | 200 | `false → true` |
| `#1129` | GPT模型官方0.35倍稳定 | Gpt0.35倍率(优质) | 0.35 | `api.ai-genesis.app` | 200 | `false → true` |
| `#1130` | GPT模型官方0.1倍低价 | Gpt0.1倍率(优惠)（已下架） | 0.1 | `api.ai-genesis.app` | 200 | `false → true` |
| `#1132` | Codex新0.15倍（生产路径1） | GPT0.16倍率(日常2) | 0.16 | `huoshenai.net` | 200 | `false → true` |
| `#1164` | Codex新0.15倍（生产路径2） | GPT0.16倍率(日常3) | 0.16 | `huoshenai.net` | 200 | `false → true` |
| `#1168` | GPT0.28倍率 | GPT0.28倍率 | 0.28 | `huoshenai.net` | 200 | `false → true` |

**未改动**：GLM(`#4`)、Kimi(`#5`)、DeepSeek(`#6`)、GPT-Image-2生图(`#1131`)，以及全部 Claude / Gemini / Grok 账号。
生图账号虽然也是 GPT 品牌，但 `gpt-image-*` 没有长上下文档位，开了是空操作，按「其他模型先不动」保持原状。

上游账号凭证、分组绑定、分组倍率、白名单、隐藏最终倍率 `18x`、用户余额、订单、历史用量均未修改。

## 2. 改动前的敞口实测

开启前扫描 4000 条 `usage_logs`（2026-09-04 09:16 ~ 09-05 19:47，约 34 小时）：

- `long_context_billing_applied = true`：**0 条**
- 总上下文（`input + cache_write + cache_read`）> 272,000 的：**153 条**，全部按短上下文价结算
- 最大单条上下文：**1,067,149 token**（`gpt-5.6-luna`）

| 模型 | 条数 | 实收 | 按长档应收 | 差额 | 最大上下文 |
| --- | --- | --- | --- | --- | --- |
| `gpt-5.6-sol` | 67 | $121.52 | $239.85 | $118.34 | 597,374 |
| `gpt-5.6-terra` | 63 | $59.79 | $118.48 | $58.69 | 825,648 |
| `gpt-5.5` | 3 | $9.93 | $19.61 | $9.69 | 320,776 |
| `gpt-5.6-luna` | 10 | $2.66 | $5.27 | $2.61 | 1,067,149 |
| **合计** | **143** | **$193.89** | **$383.21** | **$189.33** | |

约 `$134/天`。另 10 条超 272K 的是 `claude-opus-4-8`，Claude 未登记该档位，不适用。

> 跨过阈值后是**整个请求**的单价翻倍，不是只对超出部分（`billing_service.go:1107-1122` 直接改 `inputPrice/outputPrice/cacheReadPrice`），所以差额接近 2 倍。

## 3. 写入方法与安全性

`PUT /api/v1/admin/accounts/{id}` 的 `extra` 是**整体替换**（`normalizeOpenAILongContextBillingExtra` 里 `maps.Clone(input.Extra)`），
只传单个键会抹掉 `model_rate_limits`、`openai_responses_supported`、`upstream_billing_probe` 等运行态键。

采用与 `EditAccountModal.vue:4608/4634` 完全一致的做法：**GET 完整 `extra` → `{...currentExtra}` 只改一个键 → 整体 PUT**。

- 先用**零流量**的 `#1130`（分组已下架）做金丝雀，确认写入语义后再做其余 5 个。
- 逐个回读核验：`extra` **零丢键**；`credentials_status.has_api_key=true`、`base_url`、`model_mapping`（8/9 个）、分组绑定、并发、`status`、`schedulable` 全部未变。
- `extra` 在 GET 时只被 `redactAccountManagedExtra` 剥掉 3 个 `ollama_cloud_usage*` 键（`mappers.go:398`），这 6 个账号都没有该键，故 GET→PUT 回环无损。

> 更干净的替代接口：`POST /api/v1/admin/accounts/bulk-update`（`{account_ids, extra}`）走 JSONB **key 级合并**，单键翻转不需要回传整个 `extra`。本次未使用。

## 4. 差点做出的错误结论（重要）

执行前发现 `repository/scheduler_cache.go:1000` 的 `filterSchedulerExtra()` 是**白名单**，且**不含** `openai_long_context_billing_enabled`；
而网关的 `hydrateSelectedAccount()` 读的正是 `schedulerSnapshot`。
据此几乎得出「**改了不会生效，必须改代码加白名单再出镜像**」的结论。

**这是错的。** 关键在于同名不同物：

- `repository.schedulerCache` —— 确实过滤 `Extra`，且 `buildSchedulerCredentialMetadata` 连 `api_key`/`base_url` 都不保留。
- 网关持有的 `schedulerSnapshot` 是 **`service.SchedulerSnapshotService`** 包装层。其 `GetAccount()`（`scheduler_snapshot_service.go:266-289`）读完缓存**直接丢弃**，恒走 `accountRepo.GetByID` 回源：

  ```go
  var cached *Account
  if s.cache != nil { ... cached = account }
  // 账号快照不再保存上游凭证。生产路径必须回源，确保请求只从受控仓储取得解密后的凭证。
  if s.accountRepo == nil { return cached, nil }   // 仅测试接线
  ...
  return s.accountRepo.GetByID(fallbackCtx, accountID)
  ```

  `wire.go:341` 注入的是真 repo，故生产恒回源，`Extra` 完整。

**最快的证伪手段不是读调用链**：快照里连 `api_key` 和 `base_url` 都没有——若请求真用快照 account，所有 apikey 账号会认证失败、自定义上游会被打到 `api.openai.com`。生产明明是通的，就说明用的不是快照。

另注意：现有单测 `openai_gateway_record_usage_test.go:1122` 是手工构造 `Extra:{flag:true}` 的 Account 直接塞进 `RecordUsage`，
**绕过了真实的账号获取路径**，证明不了端到端生效——与坑 14 同类的假阳性。

## 5. 生效条件（开关只是三个 AND 之一）

```
applyLongCtx = (len(resolved.Intervals) == 0) && *LongContextBillingEnabled     // billing_service.go:1062
eligible     = applyLongCtx
               && pricing.LongContextInputThreshold > 0
               && (inputMul > 1 || outputMul > 1)
               && input + cache_write + cache_read > 272000                      // :1319-1328（严格大于）
```

- **有效**：`gpt-6-astra`/`gpt-6`、`gpt-5.6`/`-sol`/`-terra`/`-luna`、`gpt-5.4`、`gpt-5.5`/`-pro`
  ——`applyModelSpecificPricingPolicy`（`billing_service.go:1276-1315`）会回填阈值与倍率，不依赖远端目录，故坑 13 不影响。
- **空操作**：`gpt-5.3-codex`、`gpt-5.2`、`gpt-5.4-mini/nano` 等，`LongContextInputThreshold` 为 0。
- 若渠道/分组配了**区间定价**，`len(resolved.Intervals) != 0`，开关失效（区间自带分层）。
- 影子账号按**母账号**的开关判定（`openai_gateway_usage.go:207-213`）。
- 超阈倍率：**2x 输入、2x 缓存读、2x 缓存写、1.5x 输出**。

## 6. 验证状态

- **代码层**：已确认可即时生效，**无需**重建调度快照、无需清 `sched:acc:*`、无需重启容器——每个请求都回源读库。
- **实测**：**尚未命中**。水位线 `usage_logs.id = 405580`（2026-09-05 20:43:41）。
  改后 30 分钟内新增 19 条日志，最大上下文 204,985，未跨过 272K。
  待自然流量出现 >272K 请求后，核对该行 `long_context_billing_applied` 是否为 `true`、
  且 `total_cost` 是否等于「长档单价 × token 数」。

  > 若要强制验证：`gpt-5.6-luna` 在 0.16 分组下发一条 ~273K 输入的请求，成本约 `$0.16`（未生效）或 `$0.31`（已生效），
  > 结果本身即可区分。按坑 24，只有验「扣费金额对不对」才值得走真实网关。

## 7. 决策口径（管理员 2026-09-05 明确，已结）

原本挂了一条待确认：「上游是否也对我们按长上下文加价」——若上游没加价，这次开启就只是「多收」而非「止损」。

**管理员答复：这个前提不影响决策。**
> 「我这里多收了是好事，即使上游没多收，我这里多收了也是我的收入。」

因此：

- 上游是否加价**不必再核**，本次开启无条件成立。
- 第 2 节那 `$189` 无论定性为「亏损」还是「未捕获的收入」，结论都一样：**该收而没收，已止住**。
- 更一般的口径：**只有「少收 / 零成本放行」是缺陷，「多收」不是。** 排查计费问题先看方向——
  钱是漏出去了还是留下来了。已写入 `AGENTS.md` 五点五节首。
- 连带影响：`gpt-5.6-sol` 按旧价多收那条**按新口径不再是缺陷**，且广场展示的就是同一组数字、上游也按同价结算给我们，
  照官方 `$4/$20` 改反而是主动放弃收入 + 对上游倒挂。原先记录的「先核实官方价再改」已过期。
