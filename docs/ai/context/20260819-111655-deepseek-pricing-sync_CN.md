# DeepSeek 新定价核对与同步

## 页面取价

2026-08-19 通过浏览器读取 `https://api.ai-genesis.app/model-plaza?embedded=1` 的“【国产】DeepSeek（5折）”分组。页面标注该分组倍率为 `3.5x`，单位为每百万 token；“实付价格”是官方基准价乘分组倍率。

| 模型 | 页面实付输入 | 页面实付输出 | 页面实付缓存读取 | 页面官方基准输入 | 页面官方基准输出 | 页面官方基准缓存读取 | 分组倍率 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| `deepseek-v4-pro` | `$2.31` | `$6.93` | `$0.077` | `$0.66` | `$1.98` | `$0.022` | `3.5x` |
| `deepseek-v4-flash` | `$0.77` | `$2.31` | `$0.0245` | `$0.22` | `$0.66` | `$0.007` | `3.5x` |

页面说明国产模型展示按 `1 USD = 7 CNY` 换算；DeepSeek 这一组的官方基准本身以美元列出，因此本次不再做人民币换算。

## 同步前本地状态

生产 `sub2api-official-18082` 的 `/api/v1/model-plaza` 在同步前返回：

- 分组 `id=8`：`DeepSeek0.35倍率`，`rate_multiplier=0.35`。
- `deepseek-v4-pro` 基础价：`$0.435 / $0.87 / $0.003625`。
- `deepseek-v4-flash` 基础价：`$0.14 / $0.28 / $0.0028`。
- 分组仅绑定账号 `id=6`；账号 `rate_multiplier=30` 是账号统计口径，不参与用户余额扣费，本次不修改。
- `user_group_rate_multipliers` 没有 `group_id=8` 的用户专属覆盖。

因此本地页面实付价既缺少新的基础价，也使用了错误的 `0.35x` 分组倍率。

## 变更方案

1. 将计费服务 DeepSeek V4 Pro/Flash 兜底基础价更新为页面官方基准价。
2. 将生产 `groups.id=8` 的倍率更新为 `3.5`，展示名更新为 `【国产】DeepSeek（5折）`，与页面一致。
3. 不修改历史 `usage_logs`、订单、余额、账号统计倍率和用户专属倍率。
4. 重新构建并仅替换应用容器；PostgreSQL、Redis、Nginx、Cloudflare Tunnel 和数据卷不重建。

最终余额扣费仍遵循现有隐藏最终倍率 `BILLING_FINAL_MULTIPLIER=18`：标准基础价 × DeepSeek 分组倍率 `3.5` × 最终倍率 `18`。本次“涨价”会影响发布后的新请求，历史用量不会被重算。

## 验证要求

- DeepSeek 定向 Go 单元测试和模型广场测试通过。
- 本地 `/api/v1/model-plaza` 与公网 `/api/v1/model-plaza` 均返回上述两模型价格和 `3.5x`。
- 应用容器 healthy；PostgreSQL、Redis、Nginx 和公网健康端点保持正常。

## 执行结果

- 定向 Go 测试通过：`go test -tags unit ./internal/service -run "TestGetModelPricing_DeepSeekUsesOfficialFallbackOverDynamicPrice|TestListPlazaGroups_DeepSeekUsesOfficialPrice" -count=1`。
- 前端模型广场组件测试通过：`PlazaModelPricingTable.spec.ts` 共 14 项通过；发布后再次复跑仍全部通过。
- 生产事务将 `groups.id=8` 从 `DeepSeek0.35倍率 / 0.3500` 更新为 `【国产】DeepSeek（5折） / 3.5000`；未触碰历史用量和用户专属倍率。
- 基于当前工作区构建并发布 `deploy-sub2api:latest`，镜像摘要为 `sha256:6761f7817f6b08dae4bba0270ae99a28bb0ec90d848dba1ae5649c8ac3101472`；仅替换 `sub2api-official-18082`，PostgreSQL、Redis、Nginx、Tunnel 和数据卷未重建。
- 发布后本地与公网模型广场均返回：Flash 实付 `$0.77/$2.31/$0.0245`，Pro 实付 `$2.31/$6.93/$0.077`，分组倍率 `3.5x`。
- `127.0.0.1:18082`、本地 Nginx、`aaccx.pw`、`www.aaccx.pw`、`api.aaccx.pw` 的 `/health` 均为 HTTP 200；应用容器为 `healthy`。
