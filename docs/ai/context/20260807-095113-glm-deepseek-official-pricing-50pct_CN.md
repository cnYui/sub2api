# GLM 与 DeepSeek 官方定价五折调整

时间：2026-08-07 09:51（Asia/Tokyo）

## 官方来源

- 智谱国内 API 定价页：https://open.bigmodel.cn/pricing
  - GLM-5.2：输入 ¥8、输出 ¥28、缓存命中 ¥2 / 1M tokens。
  - GLM-5.1：输入长度 `<32K` 时为输入 ¥6、输出 ¥24、缓存命中 ¥1.3；输入长度 `>=32K` 时为输入 ¥8、输出 ¥28、缓存命中 ¥2 / 1M tokens。
- DeepSeek 官方 API 定价页：https://api-docs.deepseek.com/quick_start/pricing
  - DeepSeek V4 Flash：缓存命中 `$0.0028`、缓存未命中输入 `$0.14`、输出 `$0.28` / 1M tokens。
  - DeepSeek V4 Pro：缓存命中 `$0.003625`、缓存未命中输入 `$0.435`、输出 `$0.87` / 1M tokens。

## 换算与定价

国产模型按项目既定口径 `1 USD = 7 CNY` 换算基础美元价；用户页面再应用分组倍率 `0.5x`。隐藏的 `BILLING_FINAL_MULTIPLIER=15` 不参与模型广场展示，实际结算仍由服务端在基础成本与分组倍率之后应用。

| 模型 | 官方基础价（输入 / 输出 / 缓存读取，USD / 1M） | 模型广场实付价（0.5x，USD / 1M） |
| --- | --- | --- |
| GLM-5.1，输入 `<32K` | `$0.8571428571 / $3.428571429 / $0.1857142857` | `$0.4285714286 / $1.714285714 / $0.09285714286` |
| GLM-5.1，输入 `>=32K` | `$1.142857143 / $4.00 / $0.2857142857` | `$0.5714285714 / $2.00 / $0.1428571429` |
| GLM-5.2 | `$1.142857143 / $4.00 / $0.2857142857` | `$0.5714285714 / $2.00 / $0.1428571429` |
| DeepSeek V4 Flash | `$0.14 / $0.28 / $0.0028` | `$0.07 / $0.14 / $0.0014` |
| DeepSeek V4 Pro | `$0.435 / $0.87 / $0.003625` | `$0.2175 / $0.435 / $0.0018125` |

## 实现

- `BillingService` 固定 GLM-5.1、GLM-5.2、DeepSeek V4 Flash/Pro 的官方校准基础价，避免远程目录同步改变用户计费口径。
- GLM-5.1 按官方 32K 输入阈值使用两套输入、输出、缓存读取单价；缓存读取增加独立长上下文倍率，避免错误地跟随输入倍率。
- 模型广场 API 与前端均支持官方价和实付价的上下文分档展示，复用计费服务的同一基础价。
- 生产 `groups.id=6`（GLM）和 `groups.id=8`（DeepSeek）均更新为 `rate_multiplier=0.5`；DeepSeek 名称同步为“【国产】DeepSeek（5折）”。未改写历史用量和余额；两个分组没有用户专属倍率覆盖。

## 验证与发布

- `go test -tags unit -v ./internal/service -run 'TestGetModelPricing_(GLM51UsesCalibratedFallbackOverDynamicPrice|DeepSeekUsesOfficialFallbackOverDynamicPrice)$|TestListPlazaGroups_(GLMUsesOfficialDomesticPriceTiers|DeepSeekUsesOfficialPrice)$' -count=1` 通过。
- `go test -tags unit -v ./internal/handler -run '^TestToModelPlazaGroupDTO_UserRateAndFieldWhitelist$' -count=1` 通过。
- `pnpm exec vitest run src/components/modelPlaza/__tests__/PlazaModelPricingTable.spec.ts`（14 项）与 `pnpm typecheck` 通过。
- 仅重建并替换 `sub2api-official-18082` 应用容器；PostgreSQL、Redis、Nginx 和 Cloudflare Tunnel 未重建。
- 容器健康检查通过。本地 `http://127.0.0.1:18082/api/v1/model-plaza` 与公网 `https://aaccx.pw/api/v1/model-plaza` 返回相同的 GLM、DeepSeek 五折价格和 GLM-5.1 两档信息。
