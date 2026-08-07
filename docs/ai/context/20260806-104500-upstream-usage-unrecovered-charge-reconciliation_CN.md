# 上游 Usage 未追回扣费对账

## 范围与证据

- 上游导出：`usage_2026-07-30_to_2026-08-05(1).xlsx`，共 `97,500` 行；实际覆盖 `2026-08-03 12:42:26` 至 `2026-08-05 18:24:13`（+08）。
- 本地生产库：`usage_logs` 同期间记录 `35,087` 行。
- 本地异常来源：`ops_system_logs` 的 `record_usage_failed`。对应的 `billing_reconciliation_cases` 已有 `3,933` 条待核对项；其中上游账号 `accounts.id=1128`（GPT 0.15）共有 `2,952` 条，均为 `INSUFFICIENT_BALANCE` 后扣失败，且没有 `usage_logs`。
- Excel 只有上游账号 `xiaobianfuai@gmail.com`，不是本站终端用户；上游和本站 request ID 不透传，不能直接以 request ID 关联。

## 匹配方法

1. 限定 Excel 的 `Codex（日常）`（GPT 0.15）分组，和本地失败事件的上游账号 `1128` 对应。
2. 以模型相同、时间窗口 `+-5 秒` 建候选边。
3. 按绝对时间差从小到大一对一分配，禁止一条上游账单分配给多个本地失败事件。
4. 高置信结果使用 `+-3 秒`：`2,870` 条。该窗口内时间差中位数 `-0.97s`，90% 位 `-0.04s`，99% 位 `1.32s`，符合上游时间略早于本地失败日志的已知异步后扣链路。

## 确认未追回

计费口径按本次要求的历史公式：

```text
应追回金额 = 上游原始 × 上游倍率 × 15
```

高置信 `2,870` 笔合计应追回：`$652.17178575`。

| 用户 | API Key | 笔数 | 应追回 USD |
| --- | --- | ---: | ---: |
| `xunskyler@gmail.com` | `codex` | 1,671 | 384.62055975 |
| `1510623550@qq.com` | `chatgpt_used` | 326 | 96.20221050 |
| `853436957@qq.com` | `codex` | 313 | 83.13938100 |
| `2047431647@qq.com` | `ubuntu` | 360 | 40.65844500 |
| `itjiangzengwen@gmail.com` | `Codex_Test` | 22 | 19.32078375 |
| `2246950894@qq.com` | `codex` | 11 | 12.12801975 |
| `441565547@qq.com` | `测试` | 41 | 6.31560150 |
| `changjunwang123@gmail.com` | `yui.web legacy key sk-8d2c17931...2e1124` | 75 | 5.75445825 |
| `yannisy0225@163.com` | `Claude` | 48 | 3.78929025 |
| `2328833955@qq.com` | `ubuntu` | 3 | 0.24303600 |

模型构成：`gpt-5.6-sol` 1,539 笔、`gpt-5.6-terra` 1,255 笔、`gpt-5.5` 52 笔、`codex-auto-review` 21 笔、`gpt-5.6-luna` 3 笔。

## 待人工复核

- 另有 `62` 笔只能在 `3-5 秒` 窗口匹配，按相同公式为 `$15.99961050`，不纳入上述确认金额。
- 还有 `20` 条失败事件无可唯一匹配的上游账单：`xunskyler@gmail.com` 16 条、`853436957@qq.com` 2 条、`2047431647@qq.com` 2 条。不能据此估算或扣款。
- 若把扩展窗口也视为确认，匹配总数为 `2,932`，合计 `$668.17139625`；该数字仅作复核上限，不能替代高置信金额。

## 执行边界

- 本次只完成只读对账，没有写入 usage_logs、扣减余额、更新 `billing_reconciliation_cases` 或变更 API Key 状态。
- 生产当前 `billing_reconciliation_cases` 的 `2,952` 条对应事件仍全部为 `pending_external_usage`、`amount_usd` 为 0。
- 后续追回必须经管理员明确授权，并在同一事务中：写入可追溯 usage/billing 记录、扣减用户余额（不足则按既定欠款规则）、将对应 reconciliation case 标为 `reconciled`、保存上游请求 ID 和匹配时间差，防止重复扣款。
