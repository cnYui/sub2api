# 运维敏感值外置、GHCR 构建流水线与 GPT-6 Astra 计费

- 时间：2026-09-05 12:05（+09）
- 分支：`feat/ghcr-ci-and-gpt6-pricing`
- 决策前提：仓库保持**公开**（管理员明确选择），因此敏感值必须外置而非依赖仓库私有性
- 关联：`docs/ai/context/20260905-114416-ghcr-cicd-migration-and-repo-exposure-plan_CN.md`（本次执行的是该计划的阶段 0、2 的一部分与 GPT-6 计费）

---

## 1. 运维敏感值外置

### 做法

新增 `deploy/ops.env.example`（模板，入库）与 `deploy/ops.env`（真实值，已 `.gitignore`）。
所有运维文档改用 `${变量名}` 占位。

| 变量 | 用途 |
| --- | --- |
| `OPS_VPS_HOST` | 生产 VPS 公网地址 |
| `OPS_VPS_USER` / `OPS_VPS_PORT` | 部署用 SSH 账号与端口 |
| `OPS_DEPLOY_DIR` | 应用部署根目录 |
| `OPS_TUNNEL_ID` | Cloudflare Tunnel ID |
| `OPS_R2_BUCKET` | R2 异地备份桶名 |
| `ACCOUNT_CREDENTIALS_ENCRYPTION_KEY_HOST_FILE` | 上游凭证加密密钥的宿主机路径 |

### 清理范围

| 文件 | 状态 |
| --- | --- |
| `AGENTS.md` | 部署拓扑节改为占位符，并加入「本仓库是公开仓库」的显式警告 |
| `docs/ai/context/20260905-100322-*`、`-110342-*`、`-112834-*` | 未跟踪，VPS IP / 桶名 / Tunnel ID 全部替换 |
| `docs/ai/context/20260905-114500-*`（并行会话产出） | 未跟踪，VPS IP 已替换 |
| `docs/CLOUD_LOGIN_GUIDE_CN.md` | 未跟踪，VPS IP 已替换 |
| `20260813-060905-*`、`20260830-083933-*`、`20260830-202216-*`、`20260831-130507-*`、`20260831-135830-*` | **已跟踪且已公开**，Tunnel ID 一并参数化 |

最后 5 个文件里的 Tunnel ID 早已随公开仓库泄露，替换不能回收，但统一之后
`grep -r '7f5fafd9'` 返回空才能作为可信的守卫规则——留着例外，守卫就没有意义。

### 已验证

全仓工作区（排除 `.git`、`.worktrees`、`node_modules`、`deploy/ops.env`）对
VPS IP 与 Tunnel ID 的 grep 均无残留。

### 未处理

- 管理后台备份引导页的 i18n 文案（`frontend/src/i18n/locales/*/admin/overview.ts`）里有一个
  桶名示例。它是 UI 占位文本、不是配置泄露，但**建站时若直接照抄该示例作为真实桶名，
  等于把桶名公开**。核对一次实际桶名是否与示例雷同；雷同则改桶名或改示例。本次未动。
- **1.2 节所述的历史暴露（85 个客户邮箱、余额/退款/审计记录、隐藏最终倍率）仍然公开**，
  本次外置只防住新增值，不回收既有泄露。处置方案见 `20260905-114416` 计划的阶段 1。

---

## 2. GHCR 构建流水线

新增 `.github/workflows/build.yml`，与既有 `release.yml` 分工：

| workflow | 触发 | 产出 tag |
| --- | --- | --- |
| `build.yml`（新增） | push 到 main（忽略纯文档改动）、手动 | `sha-<完整 commit>`、`main` |
| `release.yml`（未改动） | push tag `v*`、手动 | 语义版本、`latest` |

**刻意不产出 `:latest`**——那个 tag 归 `release.yml` 所有，两条流水线抢同一个可变 tag 会导致
「服务器拉到的 latest 到底是哪次构建」无法判定。

### 几个不显然的点

- **`GOPROXY` / `GOSUMDB` 必须显式覆盖**。`Dockerfile` 的默认值是国内镜像
  （`goproxy.cn` / `sum.golang.google.cn`），在 GitHub 美国 runner 上不合适，
  workflow 中覆盖为 `proxy.golang.org` / `sum.golang.org`。原始改造方案未察觉这一点。
- **镜像名大小写**。`github.repository` 是 `cnYui/sub2api`，含大写；GHCR 不接受大写路径。
  用 `docker/metadata-action` 自动转小写，手写镜像名极易踩这个坑。
- **`concurrency` + `cancel-in-progress`**：同分支新 push 取消旧构建，避免 tag 竞态。
- **`deploy` job 默认不触发**：只有手动触发并显式勾选 `deploy` 输入时才执行 SSH 部署，
  push 到 main 永远不会自动上线。所需 secrets 与「部署密钥必须用 `command=` 限制」的
  要求写在 job 的注释里。

### 未验证

**本次没有实际触发过这条 workflow。** 计划阶段 2 要求先手动跑一次确认能出镜像，
该步骤需要在 GitHub 页面操作，尚未执行。前端构建在低配 VPS 上必 OOM，
GitHub runner 内存是否足够也还没有实测数据。

---

## 3. GPT-6 Astra 计费

### 问题

> **前提更正（2026-09-05 13:10 补）**：本节最初按「上游已上线 GPT-6、正在漏计费」书写。
> 同日并行会话在收紧 GPT 白名单时实测确认，**两个上游对 `gpt-6` 均返回 404，该模型尚未真正可用**
> （见 `20260905-131000-gpt-whitelist-restrict-54-55-56_CN.md`）。
> 因此这份定价是**预防性登记**，不是在修复正在发生的资损。白名单机制本身的缺口（下述根因）
> 与定价数据都仍然成立，上游真正上线时不必再改代码，但**需重新确认实际暴露的模型 ID**。

上游 GPT 分组若上线 GPT-6，本地对该模型将**不计费**——用户能调用，平台不扣款。

根因不是遗漏，而是设计：`getFallbackPricing` 对 OpenAI 族走**白名单**匹配
（源码注释：「仅匹配已知型号，避免未知 OpenAI 型号误计价」），
未登记的型号返回 `nil`，不会按任何价格结算。

### 官方定价

来源：<https://developers.openai.com/api/docs/models/gpt-6-astra>（2026-09-04 发布），
经第三方聚合站交叉验证一致。

| 项 | 每百万 token |
| --- | --- |
| 输入 | $10 |
| 缓存读取 | $1 |
| 缓存写入 | $12.50 |
| 输出 | $50 |
| 超过 272K 输入 | 整次请求按 2x 输入与缓存、1.5x 输出 |

缓存写入正好是输入价的 1.25 倍，与 OpenAI 全系惯例一致；
priority tier 取标准价 2 倍，与 GPT-5.6 全系一致。

**272K 阈值与 2x / 1.5x 倍率与代码中既有的 `openAIGPT54LongContext*` 常量完全相同**，
因此直接复用而非另立常量。

### 改动

1. `billing_service.go`：新增 `fallbackPrices["gpt-6-astra"]`，各字段显式写全。
2. `billing_service.go`：`getFallbackPricing` 的 OpenAI switch 增加 `gpt-6-astra` 分支。
3. `openai_model_alias.go`：`canonicalizeOpenAIModelAliasSpelling` 补 `gpt6` → `gpt-6` 归一化。
   原先只有 `gpt5` → `gpt-5`，而紧随其后的 `gpt-` 前缀校验会把裸 `gpt6` 直接判为未知模型丢弃。
4. `openai_model_alias.go`：`normalizeKnownOpenAICodexModel` 增加 GPT-6 分支。
5. `openai_model_alias.go`：新增 `isOpenAIGPT6Model`。
6. `billing_service.go`：`applyModelSpecificPricingPolicy` 把 GPT-6 与 GPT-5.6 同等对待
   （局部变量 `isGPT56` 改名为 `usesGPT56LikePolicy`）。该函数负责给**渠道/数据库配置**
   的价格补全长上下文与缓存写入参数；不改的话，「管理员在渠道里配 GPT-6 价格」
   这条路径会缺这些字段。本地兜底价已写全，走不到补全逻辑。

### 刻意的白名单取舍

`gpt-6-astra`（含后缀变体）与裸 `gpt-6` 命中 Astra 价格；
**未知的 `gpt-6-*` 变体（如将来的 mini / nano）返回空，不套用 Astra 价格。**

理由：若将来 OpenAI 上了更便宜的 GPT-6 变体，按 Astra 的 $10/$50 计价就是对用户**超收**——
这比漏计费更糟。宁可再漏一次、补一次登记，也不误收。

### 测试

新增 `billing_service_gpt6_test.go`，7 个用例全部通过：

- 各字段价格与官方一致，缓存写入 = 输入 × 1.25，长上下文常量正确
- 8 种别名写法（`gpt6-astra`、`GPT-6-Astra`、`gpt-6-astra-high`、`openai/gpt-6-astra`、
  裸 `gpt-6`、`gpt6` 等）全部解析到同一份价格
- 未知 `gpt-6-mini` / `-nano` / `-turbo` 不被归一化
- 渠道配置缺字段时补全生效，且不就地改写入参
- 端到端产生非零扣费；分组倍率线性作用于 `ActualCost` 而不影响 `TotalCost`
- 长上下文：1M 输入 + 1M 输出 = $95（`1M×$10×2 + 1M×$50×1.5`），
  恰好 272K 时仍按标准价 $52.72

> 写测试时先把长上下文那条的期望值写成了 $60（漏算阈值），是**测试错、代码对**。
> 修正后额外补了一条阈值边界用例。

---

## 4. 测试环境的既有问题（非本次引入）

`internal/service` 的 `unit` 标签测试套件**在 base commit 上就无法编译**，
涉及多个文件的未定义符号：`ptrFloat`、`ptrInt64`、`resolveRedeemAction`、
`redeemActionCreate`、`redeemActionSkipCompleted`、`paymentFulfillmentTestProvider`、
`testPtrFloat64`，以及 `payment_order_provider_snapshot_test.go` 中
`createOrderInTx` 的实参类型与签名不符。

已用 `git stash` 在改动前复现确认与本次无关。为验证新增测试，临时把其他测试文件
移出后运行，**验证完毕已全部还原（544 个测试文件，`git status` 无删除）**。

**这批损坏的测试未修复**——不在本次范围内，但它意味着当前无法对该包跑全量单测，
`backend-ci.yml` 若开启该标签会直接失败。建议单独处理。

---

## 5. 顺带处理

`backend/ip.go` 是一个 0 字节的空文件（历史误建产物，`git status` 中同类还有
`backend/0)`、`deploy/0`、`deploy/1`、`0`、`60`、`HTTP`、`({` 等十余个 0 字节文件），
空 `.go` 文件会直接让 `go build ./...` 失败。已把 `backend/ip.go` 移出仓库到临时目录，
**其余同类垃圾文件未动**，均为未跟踪状态，不影响构建也不会进入提交。

---

## 6. 当前状态与待办

已完成并可提交：敏感值外置、`build.yml`、GPT-6 Astra 计费与测试。

待办：

- [ ] **手动触发一次 `build.yml`**，确认能产出镜像且前端构建不 OOM（计划阶段 2）
- [ ] 确认上游实际暴露的模型 ID 就是 `gpt-6-astra`；若为其他名称需补登记
- [ ] 发布后用一次真实 GPT-6 请求验证扣费，核对 `标准成本 × 分组倍率 × 18`
- [ ] 服务器切换为镜像拉取（计划阶段 4，含 7 项验证清单）
- [ ] 历史暴露内容的处置（计划阶段 1）
- [ ] `internal/service` 的 `unit` 测试套件修复
