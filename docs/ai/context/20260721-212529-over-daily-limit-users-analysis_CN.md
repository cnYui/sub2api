# 超日额度用户原因分析

## 范围

本次只读分析本地开发库，未修改数据库和业务代码。

## 结论

当前两个超过新日额度的 active 订阅用户都在 `codex-pool-19-usd`，新日额度为 15 USD：

| 用户 | 今日窗口用量 | 超额 | 请求数 | 主要原因 |
|---|---:|---:|---:|---|
| `xunskyler@gmail.com` | 249.1998836 USD | 234.1998836 USD | 1666 | 大量 `/v1/responses` 流式请求，主要是 `gpt-5.6-sol` |
| `luzhiyuan2026@163.com` | 204.9496876 USD | 189.9496876 USD | 2039 | 大量 `/v1/responses` 流式请求，主要是 `gpt-5.6-sol` 与 `gpt-5.6-terra` |

不是生图主因。

## 成本拆分

`xunskyler@gmail.com`：

- 总实际成本：249.1998836 USD
- 输入成本：97.3048165 USD
- 输出成本：22.6047375 USD
- cache read 成本：129.2903296 USD
- 可能图片相关实际成本：3.42406 USD
- 非图片实际成本：245.7758236 USD
- 请求量最大模型：`gpt-5.6-sol`，1082 次，209.086242 USD

`luzhiyuan2026@163.com`：

- 总实际成本：204.9496876 USD
- 输入成本：67.537194 USD
- 输出成本：27.455604 USD
- cache read 成本：109.9568896 USD
- 可能图片相关实际成本：0.200552 USD
- 非图片实际成本：204.7491356 USD
- 请求量最大模型：`gpt-5.6-sol`，1088 次，128.037765 USD

## 图片记录说明

两人当天只有两条带 `image_count` 的记录：

- `xunskyler@gmail.com`：1 条 `/v1/responses`，`image_count=3`，`actual_cost=3.42406`，`billing_incomplete=true`
- `luzhiyuan2026@163.com`：1 条 `/v1/responses`，`image_count=1`，`actual_cost=0.200552`，`billing_incomplete=true`

这些是缺少 `image_output_tokens` 的旧记录。迁移 173 把已有 `actual_cost` 归入 `image_output_cost` 并标记不完整，用于展示和审计提示；不要把 `image_output_cost` 与 input/output/cache 成本再次相加。

## shadow 说明

这两个用户当天的 `usage_facts` 基本都是 shadow 观测：

- `xunskyler@gmail.com`：1666 条 facts，其中 1663 条 `skip_billing=true`，1663 条 `BillingType=3`
- `luzhiyuan2026@163.com`：2039 条 facts，全部 `skip_billing=true`，全部 `BillingType=3`

所以这些请求过去并没有按订阅真实扣掉日额度；当前本地回填/校准把 shadow 的 `actual_cost` 纳入 Dashboard 和日窗口，作为开发库参考用量与额度展示数据。

如果希望“历史 shadow 只展示明细、不参与 carryover 债务”，需要单独调整回填与校准口径；当前实现按“shadow 也是参考用量”计算。
