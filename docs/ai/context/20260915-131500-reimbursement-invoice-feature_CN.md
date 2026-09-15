# 报销/开票信息功能：设计、实现与验证记录（2026-09-15）

<!-- prune:keep -->

## 需求（管理员原话要点）

- 侧边栏新增「报销/开票」页面，风格与现有页面一致。用户**只能粘贴文字**、不能传文件。
- 文本由 LLM（DeepSeek 官方 API）解析成 6 个字段：公司名称、公司税号、银行账户、开户行、地址、金额。
- 解析后若有字段缺失，用表单展示「已解析 / 未解析」项并提示用户用**自然语言补充**，再解析，直到六项齐全才允许提交入库；不齐全只存用户本地。
- 管理员后台新增汇总页：表格列出所有用户提交、按序号、带提交日期、**从早到晚**排列。
- 流程：用户提交 → 状态「待处理/审核中」→ 管理员上传 PDF → 状态自动「已完成」并通知用户 → 用户看到「下载」按钮。
- 三个真实案例（福州斯摩尔、上海熠视、北京和讯）必须解析成功。

## 关键决策

| 决策 | 结论 | 原因 |
| --- | --- | --- |
| 模型 ID | `deepseek-flash` | `GET https://api.deepseek.com/models` 实测只返回 `deepseek-flash` 与 `deepseek-v4-pro`；「flash 4」是口头叫法 |
| key 存放 | settings 表 `reimbursement_llm_config`（后台弹窗可改）> env `REIMBURSEMENT_LLM_*` > 默认 | 仓库公开，key 绝不进被跟踪文件；生产 Mac 的 compose 不在仓库里，走后台设置最省事、免重启 |
| key 加密 | 明文存 settings，读回尾 4 位掩码 | 与仓库现有 `content_moderation_config` / SMTP 密码同口径；`SecretEncryptor` 依赖 `TOTP_ENCRYPTION_KEY` 未配置时重启即解不开，风险更大 |
| LLM 调用路径 | 裸 `net/http`（照 `content_moderation.go` 骨架） | 不能走 `OpenAIGatewayService`，那是计费调度路径，会扣余额、写 `usage_logs` |
| ent 建模 | 带 `User` 边（`WithUser()`） | 管理端表格要用户邮箱、要按邮箱搜索；ent 生成在临时 worktree 里做（Windows mmap 锁） |
| PDF 上传 | multipart 字段 `file`（仓库首个 multipart 先例） | base64 JSON 有 33% 体积开销且受 30s 超时限制；前端必须显式覆盖 `Content-Type` |
| PDF 存放 | `/app/data/reimbursement/<id>.pdf`，文件名由服务端生成 | 唯一持久卷；绝不用上传原名拼路径 |
| 通知 | 邮件 best-effort（新事件 `reimbursement.completed`，四处登记 + 内置 HTML 兜底） | 仓库只有邮件一种用户推送；SMTP 未配置时静默跳过，不回滚状态 |
| 字段可编辑性 | 用户**不能**手改六字段，只能文字补充再解析 | 严格按需求；服务端 `previous + 新文本` 喂 LLM 并防御性合并 |
| 状态 | 只有 `pending` / `completed` | 需求只有两态；用户侧文案「审核中」、管理侧「待处理」 |

## 接口（契约全文在会话 scratchpad，此处摘要）

用户（JWT）：
- `POST /api/v1/reimbursement/parse` `{text, previous?}` → `{fields{6 键恒存在,缺失 null}, missing[], complete, notes}`；挂 `Heavy` 限流；错误 `REIMBURSEMENT_TEXT_REQUIRED / TEXT_TOO_LONG / LLM_NOT_CONFIGURED(503) / LLM_FAILED(503)`。
- `POST /api/v1/reimbursement/requests`（六项齐全才成功，否则 `400 REIMBURSEMENT_INCOMPLETE`，message 列出缺项）
- `GET /api/v1/reimbursement/requests`（desc）、`GET .../:id`、`GET .../:id/pdf`（仅本人且 completed；越权一律 404）

管理（admin 组，接受 x-api-key，不挂 step-up）：
- `GET /api/v1/admin/reimbursement/requests?status&search&sort_by&sort_order`（默认 `created_at asc`，预加载用户）
- `GET .../:id`、`POST .../:id/pdf`（multipart `file`，≤20MB，`%PDF-` 魔数）、`GET .../:id/pdf`
- `GET/PUT /api/v1/admin/reimbursement/config`（掩码、空 key 保留、`clear_api_key`）、`POST .../config/test`（真实调一次，返回六字段 + 耗时）

## 落地文件

后端：`migrations/214_reimbursement_requests.sql`、`ent/schema/reimbursement_request.go`（+ `user.go` 反向边，生成物已回写）、`internal/config/config.go`（`ReimbursementConfig` + 5 条 `SetDefault`）、`internal/service/reimbursement.go / reimbursement_llm.go / reimbursement_service.go`（+ `notification_email_service.go` 事件登记、`domain_constants.go`）、`internal/repository/reimbursement_repo.go`、`internal/handler/dto/reimbursement.go`、`internal/handler/reimbursement_handler.go`、`internal/handler/admin/reimbursement_handler.go`、`routes/user.go`、`routes/admin.go`、三处 `wire.go` + 重生成 `cmd/server/wire_gen.go`、`deploy/docker-compose.yml`（5 个 env 占位）、`deploy/config.example.yaml`、`deploy/ops.env.example`。

前端：`api/reimbursement.ts`、`api/admin/reimbursements.ts`、`views/user/ReimbursementView.vue`、`views/admin/ReimbursementsView.vue`、`router/index.ts`（两条路由 + 简易模式 restrictedPaths）、`components/layout/AppSidebar.vue`（用户项 + 管理项 + `DocumentIcon`）、i18n zh/en（`nav.reimbursement(s)`、`reimbursement.*`、`admin.reimbursements.*`）、三个 spec。

## LLM system prompt

见 `backend/internal/service/reimbursement_llm.go` 的 `reimbursementSystemPrompt` 常量。要点：只输出固定六键 + `notes` 的 JSON；找不到填 null 不编造；税号去分隔符转大写；账号只留数字；金额去货币符号/千分位输出 number；`previous_fields` 存在时以新文本为准更新、其余沿用；无关文本全 null + notes「未识别到开票信息」。请求参数 `temperature=0`、`response_format=json_object`、`max_tokens=1024`。

## 验证结果（全部在本机实测）

1. 后端：`go build ./...` ✓；`go test -tags=unit -run '^$' ./...` 全量编译 ✓；定向单测（httptest 伪造 DeepSeek 的三案例/补充轮/围栏/字符串金额/5xx 重试/4xx 不重试/超时/未配置/settings 优先级/掩码/PDF 魔数与大小/路径越界/越权/重新上传）✓；`TestConfigKeysAreEnvReachable` ✓；`wire check` ✓；`golangci-lint`（四个包）0 issues。
2. **Live**：`REIMBURSEMENT_LIVE_TEST_API_KEY=… go test -tags=unit -run TestReimbursementParser_Live ./internal/service/` → 案例 1/2/3、案例 2 补充轮、无关文本全部 PASS（每次 0.8~1.4s）。
3. **API 级端到端**（本机 Docker Postgres 18 + Redis 8，`AUTO_SETUP` 起真实服务，迁移 214 自动应用）：24 项全 PASS——登录、合规确认、未配置 key 时 503、配置写入/掩码/来源、测试解析、案例 2 解析缺地址、不完整提交 400、补充后齐全、提交 pending、用户列表、未完成时下载 404、管理列表含邮箱与原文、非 PDF 上传 400、真实 PDF 上传变 completed、管理/用户下载字节一致且 `Content-Disposition` 带 RFC 5987 中文名、越权 404、无关文本六项缺失、空文本 400。
4. **浏览器 UI 走查**（vite dev + 本地后端）：用户页粘贴案例 2 → 解析 → 「还缺少：地址」红框、提交按钮 disabled → 文字补充 → 「信息已完整，可以提交」、提交可用 → 提交成功 toast、草稿清空、列表出现「审核中」行；管理页 11 列（序号/用户/提交日期/六字段/状态/操作）按提交时间升序、「解析设置」弹窗显示掩码 `********dde9`、来源「后台设置」、「测试解析」返回六字段 + 耗时 2548 ms。上传弹窗的文件选择无法在内置浏览器里操作，靠 vitest spec + API 级上传覆盖。
5. 前端：`pnpm run lint:check` ✓、`pnpm run typecheck` ✓、新增 3 个 spec + i18n/router/sidebar 既有 spec 共 97 用例 ✓；全量 `vitest run` 204 文件 / 1413 用例 ✓。

## 部署步骤

1. 合并到 `main` → CI 出镜像并自动部署到生产 Mac，容器启动时自动应用迁移 214。
2. 管理员登录后台 → 「报销开票管理」→ 右上「解析设置」→ 粘贴 DeepSeek key（模型保持 `deepseek-flash`）→ 「测试解析」看到六字段 → 「保存」。不需要改 compose、不需要重启。
3. 可选：如希望用 env 方式，把 `REIMBURSEMENT_LLM_API_KEY` 加进生产 Mac 的 `docker-compose.mac.yml`（仓库外），settings 为空时才会用到它。

## 已知限制 / 后续

- 发票 PDF 只在本机数据卷，R2 异地备份已失效，需要备份策略。
- 上传/解析请求已单独放宽超时（130s / 180s），但 Cloudflare Tunnel 侧仍有 100s 左右的代理超时上限，极端慢网络下 20MB 上传仍可能被边缘掐断。
- 邮件通知依赖 SMTP 已配置；未配置时用户只能靠页面状态。
- `ValidateResolvedIP: true` 会拒绝内网/回环 `base_url`；若要接内网中转需放开 `AllowPrivateHosts`。
- 用户侧列表未做状态筛选；管理侧未做删除（需求未要求）。

## 代码评审与修复（同日）

评审工作流：6 个维度（安全 / 后端正确性 / 前端正确性 / LLM 解析链路 / 部署运维 / 需求与 UX）并行找问题，每条发现再由 3 个独立反驳者（正确性 / 影响 / 复现）核实，≥2 票不可驳才确认。24 条原始发现 → 确认 16 条（含 3 组重复）→ 实际修 12 处：

| # | 问题 | 修法 |
| --- | --- | --- |
| 1 | 管理端 `search` 按字节截断切坏中文 → PG 22021 → 500 | handler 不再截断，只用服务层按 rune 截断 |
| 2 | 前端 `apiClient` 30s 超时短于后端 LLM 超时（60~120s）与 20MB 上传 | `parse`/`testConfig` 传 130s，`uploadPdf` 传 180s |
| 3 | `EmptyState` 传字符串 `icon="document"` 渲染成 `<document>` | 改用 `#icon` 插槽放 `<Icon name="document">` |
| 4 | PDF 下载 404 时用户看到 axios 原文 | `client.ts` 错误拦截器把 JSON 类型 Blob 解析成结构化错误；视图兜底 i18n |
| 5 | 超过 21MB 上传被误报「不是合法 PDF」 | `errors.As(*http.MaxBytesError)` → `REIMBURSEMENT_PDF_TOO_LARGE` |
| 6 | `AttachPDF` 非原子（先覆盖文件后写库、两条 UPDATE 无事务） | 仓储两条 UPDATE 同一事务；服务层「写临时 → 写库 → 改名」，失败删临时文件 |
| 7 | `resolvePDFPath` 绝对路径分支的目录检查恒真 | 两分支都要求 basename 等于 `<id>.pdf`（不绑定当前 pdf_dir，保历史记录可读） |
| 8 | `parse` 原文被审计中间件整体写进 `audit_logs` | `POST /api/v1/reimbursement/parse` 加入 `auditBodyOmittedRoutes`（`/requests` 保留审计） |
| 9 | `parse` 只有 60 rpm 通用限流，可被刷 DeepSeek 额度 | 进程内每用户每日 200 次上限，超限 `429 REIMBURSEMENT_PARSE_QUOTA_EXCEEDED`（管理员测试解析不计数；单实例内存计数） |
| 10 | 上传成功 toast 宣称「已通知用户」 | 文案改为「正在通知用户」 |
| 11 | 用户页解析/补充/重新开始/提交互不感知 loading | 统一 `busy` 计算属性门控五处 |
| 12 | 解析设置弹窗加载中可编辑、返回后被覆盖 | 输入框/复选框 `:disabled="loadingConfig"` + 「加载中…」提示 |

未修（有意）：发票 PDF 无异地备份（已写进 AGENTS.md 待办）；邮件模板在未配 `frontend_url` 时链接为空（与仓库既有模板一致，配了就有）；六项齐全后补充框消失（契约明文如此，重新解析入口仍在卡片一）。

修复后复验：后端 gofmt/build/全量 unit 编译/定向单测/wire check/golangci-lint 全绿；前端 lint/typecheck/全量 vitest 全绿；重建二进制后 API 端到端 26 项再次全 PASS（脚本已幂等，连跑两次一致）（含清 key → 未配置 503 → 配置 → 解析 → 补充 → 提交 → 上传 → 下载）。
