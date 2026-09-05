# GHCR 构建迁移与仓库暴露面处置计划

- 时间：2026-09-05 11:44（+09）
- 状态：**计划，尚未执行任何一步**
- 触发：评估「把 Docker 构建从 VPS 挪到 GitHub Actions」的改造方案时，核查发现仓库为公开状态

> **本文档刻意不写入具体的 VPS IP、R2 桶名、Tunnel ID 和防火墙服务名。**
> 原因见第 1.3 节——这些值目前尚未进入 git 历史，而本文档位于被跟踪的 `docs/ai/context/` 下。
> 实际值见 `docs/ai/context/20260905-100322-*` 与 `20260905-110342-*`（当前均为未跟踪状态）。
> 下文以 `<VPS_IP>`、`<R2_BUCKET>`、`<TUNNEL_ID>`、`<FW_SERVICE>` 指代。

---

## 0. 结论先行

改造方向是对的，但**不能按原计划的顺序动手**，有两个原因：

1. **仓库是公开的**，而原计划的第一步（把部署用 compose 提交进仓库）会把机器 IP、端口绑定和防火墙配置一起发布出去。
2. **原计划要从零写的 workflow，仓库里已经有了**且更完善。真正的工作量比计划描述的小得多。

因此本计划把「仓库暴露面处置」作为阶段 0/1 前置，流水线改造从阶段 2 开始。

---

## 1. 核查到的事实

### 1.1 仓库可见性

```
cnYui/sub2api    visibility: PUBLIC    isFork: true    parent: Wei-Shaw/sub2api
```

`AGENTS.md` 与多个历史 context 文档反复表述为「私有 GitHub `fork/main`」，**该表述错误**，已导致后续判断连带出错（包括本轮最初对 Actions 计费的评估）。压缩后的 `AGENTS.md` 已不含该表述，但历史文档中的说法未逐条修正。

### 1.2 已经公开的内容

仓库跟踪了 **342 个** `docs/ai/context/` 文档，全部公开可读：

| 类别 | 规模 | 说明 |
| --- | --- | --- |
| 真实客户邮箱 | **85 个**（去重） | QQ / Gmail / 163 / foxmail / proton |
| 含「余额」的文档 | 201 个 | 含具体金额、用户 ID |
| 含「退款」的文档 | 67 个 | 含订单号、退款金额、失败原因 |
| 含 `payment_audit_logs` 的文档 | 39 个 | 含审计 ID |
| 含公网域名的文档 | 82 个 | |
| `AGENTS.md` 中 `BILLING_FINAL_MULTIPLIER` | 9 处 | **隐藏最终计费倍率** |
| Cloudflare `<TUNNEL_ID>` | AGENTS.md + 1 个文档 | |
| 上游中转站域名 | AGENTS.md 6 处 + 2 个文档 | 连同各分组倍率 |

**商业影响需单独评估**：最终计费倍率是刻意不对用户展示的（模型广场专门实现了不叠加逻辑），但它与上游来源、各分组倍率一并公开，客户可据此还原完整加价结构。

**合规影响需单独评估**：85 个真实客户邮箱与其余额、订单、退款记录关联公开。

### 1.3 尚未公开、但下一次提交就会公开的内容

以下三项**当前不在 git 历史中**，是本次唯一还能守住的部分：

| 项 | 已提交版本 | 工作区待提交版本 |
| --- | --- | --- |
| `<VPS_IP>` | 尚未公开 | 在 `AGENTS.md`（工作区已 `M`） |
| `<R2_BUCKET>` | 尚未公开 | 同上 |
| `<FW_SERVICE>` 及防火墙规则 | 尚未公开 | 同上 |

另有 3 个 `20260905-*` 文档为**未跟踪**状态，含完整防火墙规则、备份配置、容量实测数据与 VPS 切换细节。

> 该风险由本轮 `AGENTS.md` 压缩引入：压缩时按内部文档标准书写，写入了部署拓扑的具体值，当时未核查仓库可见性。

### 1.4 未发现真实凭证泄露

已提交内容扫描结果：

- `sk-` 命中均为测试桩与 UI 占位符（`backend/internal/service/audit_log_test.go`、i18n `apiKeyPlaceholder`）
- `.env`、`config.yaml` **未被跟踪**（`.dockerignore` 与 `.gitignore` 均已排除）
- 无 Tunnel credentials JSON、无 `.pem` / `.key`
- 上游账号凭证以 AES-256-GCM 存于数据库，加密密钥为宿主机文件，均不在仓库

`<TUNNEL_ID>` 本身不是凭证——建立连接需要 credentials 文件，该文件未泄露。属拓扑暴露，非可直接利用的入口。

### 1.5 现有 CI 资产（原计划未察觉）

`.github/workflows/` 下已有 4 个 workflow：

| 文件 | 触发 | 关键事实 |
| --- | --- | --- |
| `release.yml` | `push` tags `v*` + `workflow_dispatch` | **已完整实现 GHCR 推送** |
| `backend-ci.yml` | `on: push`（**无分支限制**） | `runs-on: macos-15` |
| `security-scan.yml` | `on: push` + `pull_request` + 周 cron | |
| `cla.yml` | `issue_comment` / `pull_request_target` | 上游 CLA 流程，与本项目无关 |

`release.yml` 已具备原计划要新建的全部能力，且更完善：

- `permissions: packages: write`（第 26 行）
- `docker/login-action` 登录 `ghcr.io`，用 `secrets.GITHUB_TOKEN`（第 137 行）
- 镜像名小写转换（第 223 行，用 bash 的 `,,` 展开）
- `workflow_dispatch` 带 `simple_release` 开关，描述为 only x86_64 GHCR image, skip other artifacts ——**正是所需场景**
- `.goreleaser.simple.yaml:41-48` 产出 `ghcr.io/<owner>/sub2api:<version>-amd64`、`:<version>`、`:latest`，`dockerfile: Dockerfile.goreleaser`

**构建路径与原计划设想不同**：前端在**独立 job** 用 pnpm 构建，产物作为 `frontend-dist` artifact 注入 `backend/internal/web/dist/`，再由 goreleaser 用薄壳 `Dockerfile.goreleaser` 打包预编译二进制。这条路比在 Docker 内构建前端更省内存，**恰好对症 VPS 上的前端构建 OOM**。

仓库有两个 Dockerfile，用途不同：

| 文件 | 用途 |
| --- | --- |
| `Dockerfile` | 自包含多阶段：`node:24-alpine` + `pnpm@9` 构建前端，`golang:1.26.5-alpine` 构建后端，alpine 运行时 |
| `Dockerfile.goreleaser` | 薄壳：只 `COPY` 预编译二进制 + `backend/resources` + pg-client |

### 1.6 compose 与部署现状

| 事实 | 位置 |
| --- | --- |
| 生产 compose **早已是 `image:` 而非 `build:`** | `deploy/docker-compose.yml:19`，指向上游 Docker Hub 镜像 |
| 端口绑定依赖环境变量兜底 | `deploy/docker-compose.yml:29`，`BIND_HOST` 缺省值为 `0.0.0.0` |
| 唯一带 `build:` 的是开发用 compose | `deploy/docker-compose.dev.yml:13-17`（`context: ..`，`args: NPM_CONFIG_REGISTRY`） |
| **`docker-compose.vps.yml` 不在仓库** | 仅存在于 VPS，是 `BIND_HOST` 防护的关键一层 |
| 必需的宿主机 secret | `deploy/docker-compose.18082.yml` 的 `account_credentials_encryption_key`，来源为 `ACCOUNT_CREDENTIALS_ENCRYPTION_KEY_HOST_FILE`，声明为必填，缺失则 compose 直接失败 |

### 1.7 数据库迁移随启动自动执行

`backend/internal/repository/migrations_runner.go` 在应用启动时执行迁移，并对每个迁移文件做 **SHA256 checksum 校验**（已应用的迁移改内容会导致启动失败，历史上 207 号踩过，用新增 208 解决）。当前最大迁移号 `212`。

**原计划第 9 节「数据库迁移需单独执行，不会因为镜像更新自动完成」的表述与实现不符。**

---

## 2. 对原计划的评估

### 2.1 判断正确、应予保留

- 把构建挪出低配 VPS 的整体方向
- 保留 swap 作为 CI 不可用时的应急后手（且比原计划所述更重要——当前服务为单点无冗余）
- `platforms: linux/amd64` 显式指定，避免 `exec format error`
- GHCR 镜像路径必须全小写
- 自动部署需把 SSH 私钥交给 GitHub，建议单独生成部署专用密钥并在 `authorized_keys` 中用 `command=` 限制
- 新旧配置并行验证一次后再删除旧的
- `.env` 与数据卷不进镜像

### 2.2 前提不成立

| 原计划表述 | 实际 |
| --- | --- |
| 需新建 `.github/workflows/build.yml` 推 GHCR | `release.yml` 已实现，且更完善（见 1.5） |
| compose 当前写法是 `build: .`，需改为 `image:` | 生产 compose 早已是 `image:`（见 1.6） |
| 数据库迁移不会随镜像更新自动完成 | 随应用启动自动执行，且带 checksum 校验（见 1.7） |
| 私有仓库 Actions 约 2000 分钟/月，需注意消耗 | 仓库为**公开**，标准 runner 不计额度 |

### 2.3 事实性错误

| 项 | 原计划 | 实际 |
| --- | --- | --- |
| 仓库名 | `cnYui/sub2api-official-main` | `cnYui/sub2api`（目录名不等于仓库名） |
| 镜像路径 | `ghcr.io/cnyui/sub2api-official-main` | `ghcr.io/cnyui/sub2api` |
| compose 位置 | 仓库根目录 | `deploy/` 下 |

### 2.4 会直接造成事故的缺口

1. **`docker-compose.vps.yml` 不在仓库**。原计划第 4 节建议「单独维护一份 `docker-compose.prod.yml`，与本地开发用的分开」；若照做而未并入 vps override 的内容，`deploy/docker-compose.yml:29` 的 `0.0.0.0` 兜底会生效，防护从三层退化为仅剩 `DOCKER-USER` 一层。
2. **secrets 挂载完全未提及**。缺少加密密钥的宿主机文件则容器起不来；该密钥丢失则全部上游账号凭证不可解密。
3. **回滚风险被低估**。迁移随镜像走且自动执行，回滚镜像等于回滚迁移文件，但 `schema_migrations` 表记录仍在——结果是**旧代码运行在新 schema 上**。

### 2.5 计费判断更正

上一轮基于 `AGENTS.md` 的「私有仓库」表述，曾提出 `backend-ci.yml` 的 `macos-15` job 在私有仓库按 10 倍计费、会快速耗尽额度。**仓库实为公开，该结论不成立**，公开仓库标准 runner（含 macOS）不计额度。

但该结论在**转为私有后会重新成立**，因此仍列入阶段 3。

---

## 3. 两件事的耦合关系

```
仓库可见性决策
   |
   |--> 决定 Actions 是否计额度 --------> 决定是否必须收窄 CI 触发
   |--> 决定 GHCR 包配额是否适用
   |--> 决定「把部署配置提交进仓库」是否安全 --> 阻塞流水线改造第一步
```

**结论：可见性必须先定，否则流水线改造的第一步就会扩大暴露面。**

---

## 4. 分阶段计划

### 阶段 0：止血（唯一有时效性的一步）

**目标**：阻止 1.3 节三项进入 git 历史。

| # | 动作 |
| --- | --- |
| 0.1 | 改写 `AGENTS.md` 部署拓扑节，移除 `<VPS_IP>` / `<R2_BUCKET>` / `<FW_SERVICE>` 字面值，改为指向仓库外的运维记录 |
| 0.2 | 将 3 个 `20260905-*` 未跟踪文档移出仓库，或加入 `.gitignore` |
| 0.3 | 复核 `git status`，确认无其他含敏感值的待提交内容 |

**验证**：`git diff --cached` 与 `git status` 中不出现上述任一字面值。

**回滚**：本阶段只删不加，无需回滚；原始内容保留在仓库外。

---

### 阶段 1：确定文档边界与仓库可见性

**目标**：决定 342 个已公开运维文档如何处置，以及仓库是否转私有。

**必须先认清的前提**：**转私有不等于撤回已泄露内容**。公开期间 GitHub 代码搜索、爬虫与第三方镜像站均可能已取走；且本仓库是 fork，commit 存在于与上游共享的网络中。1.2 节内容须按**已泄露**处理，后续动作是止损而非回收。

候选方案（需决策，本文档不代为选择）：

| 方案 | 做法 | 代价 |
| --- | --- | --- |
| A | 仓库转私有，`docs/ai/context/` 留在仓库 | Actions 转为配额制（触发阶段 3）；已泄露内容不可回收 |
| B | 仓库保持公开，`docs/ai/context/` 移出仓库并清理历史 | 需 `git filter-repo`；fork 网络可能仍可通过 SHA 访问旧 commit |
| C | A + B 同时做 | 工作量最大，暴露面最小 |

**关联动作（无论选哪个方案都要做）**：

- 评估是否轮换 `<TUNNEL_ID>`（凭证未泄露，成本近乎为零，建议做）
- 评估 85 个客户邮箱公开的合规义务
- 评估最终计费倍率公开的商业影响
- 修正历史文档中「私有 GitHub」的错误表述，或在 `AGENTS.md` 记录该更正

---

### 阶段 2：验证现有 GHCR 流水线（不写任何代码）

**目标**：先确认轮子是否可用，再决定造不造。

| # | 动作 |
| --- | --- |
| 2.1 | 在 GitHub Actions 页面手动触发 `release.yml` 的 `workflow_dispatch`，勾选 `simple_release`，`tag` 填一个测试版本 |
| 2.2 | 观察构建是否通过；重点看前端 job（产出 `frontend-dist` artifact）是否 OOM |
| 2.3 | 到仓库 Packages 页面确认是否产出 `ghcr.io/cnyui/sub2api:<version>` 与 `:latest` |

**验证**：Packages 页面出现预期 tag，且 manifest 架构为 `linux/amd64`。

**分支**：

- **通过** → 阶段 3 只需给 `release.yml` 增加 main 分支触发（或继续手动触发），**不新建 workflow**
- **未通过** → 记录失败原因后再评估是否新建 `build.yml`；若为前端内存问题，在 `Dockerfile` 或前端 job 设 `NODE_OPTIONS=--max-old-space-size=4096`

**回滚**：本阶段只读，产出的测试镜像可在 Packages 页面删除。

---

### 阶段 3：收窄 CI 触发（仅在阶段 1 选择转私有时必做）

| # | 动作 |
| --- | --- |
| 3.1 | `backend-ci.yml`：`on: push` 改为限定分支，或改为仅 `pull_request` |
| 3.2 | 评估 `macos-15` job 是否必需（它跑的是 `deploy/*.sh` 的语法检查与三个 shell 测试，Linux runner 应可胜任） |
| 3.3 | `security-scan.yml`：`on: push` 收窄，保留周 cron |
| 3.4 | `cla.yml`：上游 CLA 流程，评估是否在 fork 中禁用 |

**验证**：推一个无关提交，确认只触发预期的 workflow。

---

### 阶段 4：服务器切换为镜像拉取

**前置**：阶段 0、2 完成；阶段 1 已决策。

| # | 动作 |
| --- | --- |
| 4.1 | **先把 `docker-compose.vps.yml` 纳入版本管理**（仓库或仓库外的运维库，取决于阶段 1 决策）——它目前是单点文件，VPS 故障即丢失 |
| 4.2 | 新建生产 compose，`image:` 指向 `ghcr.io/cnyui/sub2api` 并支持用变量指定 tag |
| 4.3 | **确认新 compose 保留**：`ports` 绑定 `127.0.0.1`、`env_file`、`secrets` 挂载、`NO_PROXY` 列表、数据卷路径、`external` 网络名 |
| 4.4 | 服务器 `docker login ghcr.io`（镜像若为 public 可跳过） |
| 4.5 | 保留旧 compose 备份，新旧并行验证一次 |
| 4.6 | `docker compose pull` 后 `docker compose up -d` |

**验证清单**（前两项在 9 月 5 日 VPS 切换时均翻过车，必须逐项确认）：

- [ ] 容器内 `BILLING_FINAL_MULTIPLIER` 环境变量值正确（基础 compose 无 `env_file`，`.env` 不会自动进容器）
- [ ] secret 文件在容器内可读，上游账号凭证可解密
- [ ] JWT secret 与切换前一致（注意实际生效值来自 `/app/data/config.yaml` 而非 `.env`）
- [ ] `docker ps` 端口映射为 `127.0.0.1`，公网实测不可达
- [ ] 三域名 `/health` 返回 200
- [ ] 一次真实 API 调用返回 200，且扣费金额符合「标准成本 × 分组倍率 × 最终倍率」
- [ ] 迁移已执行到预期号且无 checksum 报错

**回滚**：保留旧 compose 与旧镜像，用旧 compose 文件重新 `up -d`。

---

### 阶段 5：自动部署（可选，建议延后）

稳定运行一段时间后再评估。若接入：

- 单独生成部署专用 SSH 密钥，不复用管理密钥
- 服务器 `authorized_keys` 用 `command=` 限制为只能执行部署脚本，不开放完整 shell
- 注意与 SSH 加固后的实际端口、UFW 放行规则保持一致
- 部署脚本应在 `up -d` 后做健康检查，失败时自动回滚到上一个 tag

---

## 5. 风险与未决项

| 项 | 状态 |
| --- | --- |
| 1.2 节内容已公开，不可回收 | 需按已泄露处理，止损方案待阶段 1 决策 |
| 回滚镜像会导致旧代码运行在新 schema 上 | 需在阶段 4 前定义「可回滚版本范围」的判定方法 |
| CI 链路引入对 GitHub 可用性的依赖 | 保留 VPS swap 作为应急构建后手 |
| 服务为单点无冗余 | 与本计划无关，但会放大任何部署事故的影响 |
| 外部拨测未配置 | 部署切换期间无法及时发现服务不可用 |
| `docs/ai/context/` 已积累 363 个文档 | 无论是否公开，均需一次归档策略决策 |

---

## 6. 与原计划的落地顺序差异

| 原计划顺序 | 本计划顺序 | 差异原因 |
| --- | --- | --- |
| 1. 确认假设值 | 0. 止血：阻止敏感值进入 git | 有时效性，一次提交即不可逆 |
| 2. VPS 加 swap | 1. 决定可见性与文档边界 | 阻塞后续所有步骤 |
| 3. 提交新建的 `build.yml` | 2. 先手动触发现有 `release.yml` 验证 | 不确认轮子是否存在就造轮子 |
| 4. Packages 页面确认镜像 | 3. 收窄 CI 触发（若转私有） | |
| 5. 服务器 `docker login` | 4. 服务器切换为镜像拉取 | 新增 vps override 纳管、secrets、7 项验证清单 |
| 6-8. 改 compose、验证、评估自动部署 | 5. 自动部署（延后） | |

原顺序会在完成第 3 步之后才发现 2.4 节的三个缺口，且第 1 步就已把敏感值提交。

---

## 7. 当前状态

**未执行任何一步。** 阶段 0 有时效性——只要工作区的 `M AGENTS.md` 与 3 个未跟踪的 `20260905-*` 文档尚未提交，1.3 节三项就仍可守住。
