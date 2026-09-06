# GLM 分组与渠道下架

- 时间：2026-09-05 22:20 ~ 22:35（+09）
- 环境：生产 `https://aaccx.pw`
- 操作方式：浏览器既有管理员会话调管理 API（未输入任何凭证），未直连数据库
- 管理员指令：「把 GLM 分组和渠道下架，和之前的 grok 分组一样」

## 背景

同一轮先做了非 GPT 模型与上游 `api.ai-genesis.app` 广场的定价对比（详见
`20260905-220000` 系列对比结论）：DeepSeek / Kimi 的真实扣费单价与上游逐位一致；
GLM 我方按智谱官方 ¥价 ÷7 计（glm-5.1 = ¥6/¥24/¥1.3 → $0.857/$3.43/$0.186），
上游是拍平的 $1.4/$4.4，比官方还高。管理员据此决定把 GLM 整体下架。

## 变更

| 对象 | id | 变更前 | 变更后 | 接口 |
| --- | --- | --- | --- | --- |
| 分组 GLM0.6倍率 | 6 | status `active` | status **`inactive`** | `PUT /api/v1/admin/groups/6` |
| 渠道（上游账号）GLM模型官方0.2折价格 | 4 | schedulable `true` | schedulable **`false`** | `POST /api/v1/admin/accounts/4/schedulable` |

净改动精确为 `{分组.status, 账号.schedulable}` 两项，其余全部未动。

## 与 Grok 下架的异同

- **相同**：分组 `status → inactive`（Grok 分组 `#3` 同款，倍率 `0.6` 保持不变）。
- **多做一步**：本次按管理员明确的「分组**和渠道**」把上游账号 `#4` 也 `schedulable → false`。
  Grok 那次**只停了分组、账号 `#1` 至今仍是 active/schedulable**（见
  `20260905-120500-group-rate-and-shelf-adjustments_CN.md`）。本次渠道也下架是额外的一层。

## 安全性（改动前已核实的坑）

- **分组 PUT 是整体替换里混着无条件覆盖字段**：`admin_group.go:656` 的
  `daily/weekly/monthly_limit_usd` 无条件走 `normalizeLimit(input.X)`。但请求 DTO 用
  `optionalLimitField`（带 `set` 标志，`group_handler.go:39`）：**JSON 省略该字段 → `set=false` →
  `ToServiceInput()` 返回 nil**，不会误写成 0。其余字段要么 nil 指针守卫，要么
  `messages_dispatch_model_config` / `models_list_config` / `model_routing` / `rpm_limit` /
  `reasoning_effort_*` 等**在 `UpdateGroup` 里根本没被读取**（走独立端点），裸 PUT 不会动它们。
- 逐字段 diff 确认：分组除 status 外**零改动**，`messages_dispatch_model_config`（3 条映射）、
  `models_list_config`（13 个模型）、`mcp_xml_inject=true`、`rate_multiplier=0.6` 全部保留。
- **限额 `0` 与 `null` 完全等价**：`group.go:129` `HasDailyLimit()` 是 `!= nil && > 0`，
  所以 `0` 和 `null` 都判「无限制」。生产所有在售 GPT/Kimi/DeepSeek/Claude 分组的限额都是 `0`
  且正常服务海量流量，**服务注释里「0 表示不允许用量」是错的**。首次 PUT 把 GLM 限额从 `0`
  变成了 `null`（行为无差异），随后补一次 PUT 恢复为 `0`，使其与兄弟分组一致、净改动最小。
- **渠道下架用 `POST /accounts/{id}/schedulable`**（账号列表行的开关同款），只改
  `schedulable`，**不碰** `status`/凭证/`base_url`/`model_mapping`/分组绑定。回读确认
  `has_api_key=true`、`base_url` 与 `model_mapping`（2 键）、分组绑定（仅 `6:GLM0.6倍率`）均未变。

## 验证

- 模型广场 `/api/v1/model-plaza`（公开端点）：GLM 分组**已消失**（inactive 分组不下发）。
- 用户建 API Key 的可选分组 `/groups/available`：**已无 GLM**。
- 账号 `#4` 仅绑定分组 `6`，分组已 inactive，账号又 `schedulable=false`，两道闸都关。

## 回滚

- 分组：`PUT /api/v1/admin/groups/6 {"status":"active", "daily_limit_usd":0, "weekly_limit_usd":0, "monthly_limit_usd":0}`。
- 渠道：`POST /api/v1/admin/accounts/4/schedulable {"schedulable":true}`。
- 存量绑定 GLM 分组的 API Key 在下架期间不可用，回滚后恢复。

## 未改动

上游账号凭证、`base_url`、`model_mapping`、账号与分组的绑定、隐藏最终倍率 `18x`、
用户余额、订单、历史用量、以及 GLM 的定价（`fallbackPrices["glm-5.1/5.2"]`，仍是智谱官方 ¥÷7）。
其它分组与账号（含刚开启长上下文计费的 6 个 GPT 账号）均未受影响。
