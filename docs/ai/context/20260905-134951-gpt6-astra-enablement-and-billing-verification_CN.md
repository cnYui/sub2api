# GPT-6 Astra 开放与真实计费验证

- 时间：2026-09-05 12:45 ~ 13:50（+09）
- 环境：生产 `https://aaccx.pw`
- 操作方式：浏览器既有管理员会话调管理 API（未输入任何凭证），未直连数据库
- 结果：`gpt-6-astra` 已开放，实测扣费与官方定价逐位一致

---

## 1. 更正两条此前的错误结论

**① 「上游没有 GPT-6」是错的。**
`20260905-131000` 记录的实测是裸 `gpt-6` 返回 404，据此判定"上游尚无此模型"。
实际上上游只认精确串 **`gpt-6-astra`**：

| 模型名 | 上游响应 |
| --- | --- |
| `gpt-6-astra` | **HTTP 200**，正常返回 |
| `gpt-6` / `gpt6` / `gpt-6-astra-high` | 404 `not supported by any configured account` |

测错字符串比没测更误导——它产生了一个看起来有依据的错误结论。

**② 「PR #2 修好了 GPT-6 计费」也是错的。**
取价有三级：渠道/分组定价 → `PricingService` 远端目录 → 硬编码 `fallbackPrices`。
PR #2 只改了第三级，而生产命中第二级。真正的缺陷是 `matchOpenAIModel` 末尾的
`DefaultTestModel`(`gpt-5.4`) 兜底会让别名少收 4 倍，已由 PR #5 修复。

**但本次实测证明：在这条链路上该缺陷不会触发**——别名在上游就被 404 拦掉了，
产生不了可计费用量。exact 串走远端目录，价格本来就正确。

---

## 2. 生产是否加载了远端价格目录：不用 SSH 的判定法

此前认为该问题只能 SSH 确认。实际有一条纯只读的外部通道：

模型广场 `/api/v1/model-plaza` 是**公开端点且暴露价格**。挑选满足两个条件的模型——
**只存在于远端目录（231 键）而不在镜像内嵌目录（198 键）**，且**源码里没有硬编码兜底价**——
若生产能报出它的精确价格，则价格只可能来自远端目录。

| 模型 | 远端目录基准价 | 生产广场展示价 | 比值 |
| --- | --- | --- | --- |
| `claude-fable-5` | `1e-05` | `1e-05` | 1.0 |
| `claude-sonnet-5` | `2e-06` | `2e-06` | 1.0 |
| `gemini-3.5-flash-lite` | `3e-07` | `3e-07` | 1.0 |

**结论：远端目录已成功同步**，`gpt-6-astra` 的基础价在生产上本来就是对的，开放无需先部署。

> 注意排除干扰项：`deepseek-v4-pro`、`glm-5.1`、`kimi-k2.5/2.6`、`gpt-5.6` 虽然也只在远端目录，
> 但源码里有硬编码兜底价，不能作为证据。

---

## 3. `model_mapping` 的双重语义

存储位置：`accounts.credentials["model_mapping"]`，类型 `map[string]string`。

- **名称翻译**：key = 用户请求的模型名，value = 转发给上游的模型名。本项目全部使用恒等映射。
- **准入白名单**：调度阶段 `gateway_scheduling.go:296` 检查请求模型是否为 map 的 key，
  不是则该账号被踢出候选（`filteredModelMapping++`）。全部账号被踢出 → 404。

这是本次唯一的阻塞项。

---

## 4. 修改 credentials 的两个陷阱（已验证）

**① 只传 `model_mapping` 会删掉 `base_url`。**
`MergePreservingSensitiveCreds` 以 incoming 为基底，**非敏感键完全由 incoming 决定**，
没传就是删除。三个账号的 credentials 恰好只有 `base_url` + `model_mapping` 两个键，
只传后者会直接废掉上游地址。

**② 敏感键在 GET 时被整个删除，不是返回掩码。**
`dto.RedactCredentials` 剥离 `SensitiveCredentialKeys`（`api_key` 等 14 个）并另出
`has_<key>` 状态位。因此 GET → 改 → 整体 PUT 是安全的：incoming 不含敏感键，
后端自动保留原加密值。若 GET 返回的是掩码字符串，PUT 回去反而会覆盖真凭证。

**安全流程**：`GET` 完整 credentials → 在原对象上只加一个键 → 整体 `PUT`。

另确认 `UpdateAccountRequest` 中 `Name`/`Status` 为非指针字符串，但服务层有
`if input.X != ""` 判空；`GroupIDs`/`Concurrency`/`Extra` 为指针判 nil。
**只传 `credentials` 不会影响其它字段。**

---

## 5. 执行结果

对三个 上游B 上游账号各加一个键 `"gpt-6-astra": "gpt-6-astra"`：

| 账号 | 名称 | PUT | `base_url` | 映射键数 | 状态/调度/分组 |
| --- | --- | --- | --- | --- | --- |
| `#1132` | Codex新0.15倍（生产路径1） | 200 | 保留 | 8 → 9 | active / true / 1 |
| `#1164` | Codex新0.15倍（生产路径2） | 200 | 保留 | 8 → 9 | active / true / 1 |
| `#1168` | GPT0.28倍率 | 200 | 保留 | 8 → 9 | active / true / 1 |

网关 `/v1/models` 随即从 8 个模型变为 9 个，含 `gpt-6-astra`。

---

## 6. 真实计费验证

经 `https://api.aaccx.pw/v1/chat/completions` 发一次真实请求，HTTP 200。

`usage_logs` id `404425`：

| 字段 | 值 | 核对 |
| --- | --- | --- |
| 模型 / 上游模型 | `gpt-6-astra` | |
| 用户 | `xiaobianfuai@gmail.com`（id `448`） | |
| 分组 / 倍率 | GPT0.16倍率(日常2) / `0.16` | |
| 账号 | `#1132` | |
| `input_cost` | `0.00864` | `864 × $10/M` ✓ |
| `cache_read_cost` | `0.007488` | `7488 × $1/M` ✓ |
| `output_cost` | `0.0011` | `22 × $50/M` ✓ |
| **`total_cost`** | **`0.017228`** | 三项相加 ✓ |
| **`actual_cost`** | **`0.04961664`** | `0.017228 × 0.16 × 18` ✓ |

余额 `99733.6975054` → `99733.64788876`，差额 `0.04961664`，与 `actual_cost` 一致。

**三个价格分量全部按 Astra 官方价结算。** 若踩了 `gpt-5.4` 兜底，`total_cost` 会是
`0.004362`、`actual_cost` 会是 `0.01256256`——实测排除了这种情况。

---

## 7. 未处理事项

- **`>272K` 长上下文档位仍不生效**。远端目录用 `*_above_272k_tokens` 表达，解析器只认
  `long_context_*`（目录中命中 0）。该档位只能由 PR #5 的静态价提供，**而 PR #5 尚未部署**；
  且还需账号 `extra.openai_long_context_billing_enabled = true`（默认 false）。
  当前所有 GPT-6 请求按标准价计费，超过 272K 输入的请求会少收。
- **PR #5 未部署**。它的价值是：目录同步失败时的兜底 + 长上下文档位。当前不阻塞。
- **自动部署未打通**。`build.yml` 的 deploy job 是手动 opt-in（`workflow_dispatch` + 显式勾选），
  且 `VPS_*` secrets `total_count: 0`。**push 到 main 不会自动部署到 VPS**，
  需要配置 5 个 secrets 并在 VPS 上放 `deploy.sh`。
- **只开放了 上游B 的三个账号**。`api.ai-genesis.app` 的账号（`#1128`/`#1129`/`#1130`）
  未加入 `gpt-6-astra`，其对应分组仍不可用该模型；这些上游是否支持未测。
- 验证用请求已产生真实扣费 `0.04961664 USD`，未回滚（管理员自有账户）。
