# 2026-09-23 在售分组模型可用性实测 + 白名单收紧 + 备注与监控同步

## 指令

管理员要求：当前所有分组逐个测试模型可用性；每个分组都设白名单、剔除调不通的模型，让用户只能用白名单里的模型；
同步更新分组备注；渠道监控页只展示白名单模型，而不是所有模型。

## 范围

只处理**在售（`status=active`）**的 12 个分组：4、7、10、12、71、74、75、76、80、81、82、83。
`inactive` 的 8、9、13、69、70、77、78、79 未测、未改（下架中，广场和监控都不显示；5、73 已软删）。

## 方法

- 通道：诊断密钥 SSH 进生产 Mac，主机内 `curl/urllib 127.0.0.1:8080` 带 `admin_api_key`（审计 actor 为 `admin:448`）。
- 每个账号先 `POST /admin/accounts/{id}/models/sync-upstream`（只读）拿上游 `/v1/models`，
  再对白名单里的**每个模型**调一次 `POST /admin/accounts/{id}/test`（`{"model_id": ...}`，直连上游、不写 `usage_logs`、不扣余额），
  并对照近 7 天 `usage_logs` 的成功请求。
- 测试脚本在每个账号测完后回读 `status/schedulable/rate_limit_reset_at`，若被测试打成 error / 限流就立即 `clear-error` / 恢复调度 / `clear-rate-limit`。
  本次**没有触发**任何一种（Claude 账号 403 会 `SetError`+停调度，OpenAI 账号 401 同理、429 会写限流）。
- 暂时性错误（502 / 503 / overloaded / 429）隔 15～20 分钟重测，只有**确定性的 404 model_not_found** 或**反复失败且多日无成功**才剔除。

## 实测结果

| 分组 | 账号 | 通过 | 剔除（原因） |
| --- | --- | --- | --- |
| 4 Claude1.5倍率（优质） | #2 | claude-sonnet-5、opus-5、opus-4-8、opus-4-7、opus-4-6、sonnet-4-6、haiku-4-5-20251001 | claude-opus-4-5-20251101（404，上游只有不带日期的 `claude-opus-4-5`） |
| 7 Kimi1倍率 | #5 | kimi-k3 | kimi-k2.5、kimi-k2.6（404） |
| 10 GPT0.35倍率（优质） | #1129 | gpt-6-astra、gpt-5.6-sol、gpt-5.5、gpt-5.6-terra（首测 502，重测通过，今天 11:02 仍有成功） | gpt-5.6（三次分别 overloaded / CF 502 / 429 all rate-limited，09-17 15:12 后无成功请求；上游清单虽列出） |
| 12 GPT生图1倍率 | #1131 | gpt-image-2（真实出图一次） | — |
| 71 Claude0.78倍率 | #1166（火神） | —（见下） | 11 个 404：claude-haiku-4-5、haiku-4-5-20251001、opus-4-5-20251101、sonnet-4-5-20250929、opus-4-1-20250805、opus-4-20250514、sonnet-4-20250514、3-7-sonnet、3-5-sonnet ×2、3-5-haiku |
| 74 GPT0.28倍率 | #1168 | gpt-5.6-sol、gpt-6-astra | — |
| 75 Grok0.15倍率 | #1169 | grok-4.7、4.6、4.5 | — |
| 76 GLM0.6倍率 | #1173 | glm-5.3、glm-5.3-flash | — |
| 80 GPT1倍率 | #1174 | gpt-6-astra | — |
| 81 / 82 / 83（日常二） | #1175 / #1176 / #1177 | deepseek-v4.1-flash / glm-5.3-flash / kimi-k2.6 | — |

**分组 71（火神 Claude）两轮实测全挂**：每轮第一个请求挂约 183 秒后 502，之后所有模型秒回
`503 No available accounts`（火神自己的 Claude 号池没有可用账号）。但剩下 6 个模型都在火神 `/v1/models` 里，
今天 08:44 还有一笔 `claude-fable-5` 成功，近两周是「偶尔能用」。所以这 6 个**保留**在白名单里，
备注第三行写明「上游不稳定，调不通请改选 Claude1.5倍率（优质）」。**要不要把 71 改 `inactive` 由管理员决定**
（有效 key 9 个 / 7 人，停用后这些 key 会 403）。

注意：白名单**不能清空**——`model_mapping` 为空等于放行全部模型（`Account.IsModelSupported`）。

## 变更（回滚用）

白名单用 `POST /admin/accounts/bulk-update` `{account_ids:[id], credentials:{model_mapping:{...}}}` 改：
JSONB 键级合并、事务内 `FOR UPDATE`、不碰 `api_key`/`base_url`，**也不会像 `PUT /accounts/{id}` 那样异步重跑 Responses 能力探测**（坑 20）。
#5 的 `openai_responses_supported=false` 且**没有** mode 锁，用 PUT 改它有可能被探测翻成 true。

| 账号 | 改前 `model_mapping` 键 | 改后 |
| --- | --- | --- |
| #2 | claude-haiku-4-5-20251001、claude-opus-4-5-20251101、claude-opus-4-6、claude-opus-4-7、claude-opus-4-8、claude-opus-5、claude-sonnet-4-6、claude-sonnet-5 | 去掉 claude-opus-4-5-20251101 |
| #5 | kimi-k2.5、kimi-k2.6、kimi-k3 | kimi-k3 |
| #1129 | gpt-5.5、gpt-5.6、gpt-5.6-sol、gpt-5.6-terra、gpt-6-astra | 去掉 gpt-5.6 |
| #1166 | 上表 11 个 404 + claude-fable-5、opus-4-6、opus-4-7、opus-4-8、sonnet-4-6、sonnet-5 | 只留后 6 个 |

全是恒等映射。回滚就是用同一接口把旧键集合写回。改后回读：四个账号 `api_key` 仍在、`base_url` 不变、
`status=active`、`schedulable=true`，#5 仍 `openai_responses_supported=false`，#1129 仍 `force_chat_completions`。

分组备注（`PUT /admin/groups/{id}`，三个限额字段按 GET 值 `0` 原样回传，坑 27；名称、状态、倍率回读不变）：

- 4：`可用 claude-opus-5、claude-sonnet-5、claude-opus-4-8、` / `claude-opus-4-7、claude-opus-4-6、claude-sonnet-4-6、` / `claude-haiku-4-5-20251001 模型`
  （原来写成 `claude-opus-4-8/4-7/4-6` 缩写，下拉搜索搜不到完整 ID，顺手按规范展开）
- 10：`可用 gpt-6-astra、gpt-5.6-sol、gpt-5.6-terra、` / `gpt-5.5 模型`（去掉 gpt-5.6）
- 71：`可用 claude-fable-5、claude-sonnet-5、claude-opus-4-8、` / `claude-opus-4-7、claude-opus-4-6、claude-sonnet-4-6 模型` / `上游不稳定，调不通请改选 Claude1.5倍率（优质）`
- 7、12、74、75、76、80～83 的备注本来就和实测一致，没动。

## 监控

`POST /admin/channel-monitors/sync-groups` → `created 0 / updated 4 / disabled 0 / unchanged 8`。
更新的是 15（分组 4）、16（7）、23（10）、26（71），主模型都还在白名单里所以保留。随后对 12 个启用监控逐个 `POST /{id}/run`，
每个监控只列白名单模型，全部 `operational`。以后改白名单不用手动改监控，调度器 10 分钟内自动同步。

**监控绿灯 ≠ 能用**：监控只拉 `/v1/models` 看模型在不在目录里（规则禁止推理探测），71 的 6 个模型全绿，但同一时刻真实请求全是 502/503。

## 验证

- 公开 `GET https://aaccx.pw/api/v1/model-plaza`：12 个在售分组的模型列表与新白名单逐一一致。
- 被剔除的模型：账号选不上时 `classifyNoAccountError` 按持久配置诊断，返回 `404 model_not_found`
  （`Model "x" is not supported by any configured account in this group`），不会打到上游、不扣费。

## 追加：分组 71 已停用（同日，管理员指示「71 停用吧」）

第三轮重测（约 30 分钟后）仍是 183 秒 502 + 秒回 503，管理员决定停用。

- `PUT /admin/groups/71`：`status: active → inactive`，名称、倍率、平台、三个限额（`0`）回读不变；
  备注改为 `【暂时下架 2026-09-23】上游火神 Claude 号池不可用` / `请改选 Claude1.5倍率（优质）`（沿用 9/13/69 的写法）。
- 停用时有效 key 9 个 / 7 人，最近一次使用 09-23 08:44，近 24 小时成功 1 笔；这些 key 现在会 `403 GROUP_DISABLED`。
- `sync-groups` → `disabled 1`，监控 26 自动停用；在售分组剩 11 个：4、7、10、12、74、75、76、80、81、82、83，公开广场回读一致。
- `account_groups` 绑定与 #1166 的 6 个模型白名单都保留。**恢复**：先用账号「测试连接」确认火神 Claude 能调通，
  再把 `status` 改回 `active`，备注改回上面「变更」一节里的三行（第三行视当时稳定性决定要不要留），监控 10 分钟内自动恢复。

## 上游有、我方白名单没有的模型（本次只删不加）

加模型必须先确认定价生效（坑 12），本次没加：

- #2：claude-fable-5、claude-fable-5-1、claude-haiku-4-5、claude-opus-4-5
- #1166：claude-opus-5、claude-fable-5-1
- #1129：gpt-6、codex-auto-review、gpt-5.3-codex-spark，以及 4 个图像模型（生图单独隔离，坑 6）
- #1169：grok-build-0.1、grok-imagine-image-2.0、grok-imagine-video-1.5
- #1173：glm-5.3-flashx
- #1175～#1177（同一上游分组、同一钱包）：各自都能调另外 4 个模型和 qwen3.8-27b，隔离靠我方白名单
