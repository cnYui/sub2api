# AI 协作入口

> 最新压缩记忆：`docs/ai/context/20260728-151255-agents-memory-condensed_CN.md`。
> 上一版压缩记忆：`docs/ai/context/20260717-093308-agents-memory-condensed_CN.md`。
> 长期上下文统一新建到 `docs/ai/context/YYYYMMDD-HHMMSS-*.md`，不要覆盖、重命名或删除历史文档。

## 协作规则

- 默认使用中文；文档、说明、总结、计划、回复和代码注释均使用中文，除非用户明确要求英文。
- 代码注释写原因，不写过程；表达简洁直接。
- 函数式优先，组合优于继承；TS/JS 避免 OOP。
- 新功能优先复用或重构现有代码，遵循 KISS、DRY 和 ai-coding-discipline。
- 从第一性原理确认真正问题，警惕 XY 问题，解决根因，不做 workaround。
- 小设计问题直接重构；大设计问题原地加 TODO 并说明原因。
- 修改、架构设计、技术选型前后，在 `docs/ai/context/` 新建 plan/design/result 文档。

## 当前架构与运行态

- Sub2API 是唯一公网 API 入口、用户 API Key、计费和用量事实源。
- 当前主链路：`Cloudflare Tunnel -> sub2api-public-nginx-local:8080 -> 外层 sub2api-dev:18080 -> 内层 sub2api-upstream-latest:18086 -> OpenAI`。
- 2026-07-28 已核验 Nginx upstream 为 `host.docker.internal:18080`，`18080/18086/8080` health 均为 200。
- 外层 `18080` 负责用户、套餐、流量卡、价格和计费；内层 `18086` 负责 OpenAI OAuth 账号池和上游调度。
- `cliproxyapi-local-dev:8317` 即使运行也不代表参与主链路；必须核对外层账号调度和 usage 日志。
- 正式模型 API 只支持 `/v1/*`；裸 `/responses`、`/models`、`/chat/completions`、`/embeddings`、`/images/*` 不做静默兼容。
- 旧 `18084/18082/18085` 和 CPA 链路均为历史状态；判断运行态必须检查 Nginx、容器、端口和 health。
- `xiaobianfuai@gmail.com` 是管理员和本机 Codex Local Key 所属账号，不按普通用户删除。

## 最高优先级定论

- 2026-07-28 计费审计确认：外层 reservation 已启用且非 shadow，但仍有 74 条过期 `dispatched` 无 usage fact，冻结 5 个用户共 `31.49608750 USD`；另有 2902 条历史 debt，合计 `531.91499889 USD`。当前未修复、未批量释放、未处理历史 debt。
- 根因是请求已派发后的 HTTP/SSE/流式失败没有统一终结 reservation，以及 Embeddings、OpenAI Messages、WebSocket turn 等入口未统一请求前授权。
- 目标固定为：套餐原子 hold -> 流量卡 reservation -> 无来源请求前 402。账户余额不参与模型请求；一次请求只能有一个 authorization，结算禁止重新选源。
- 所有 OpenAI 请求单请求预算硬上限为 2 USD；按最终文本输入、输出上限、附件处理和图片数量精确预算，不固定冻结 2 USD，不用金额阈值判断生图。
- 2 USD 不是最低授权额；资金不足时在原子事务内使用 `min(2 USD, 单一资金来源真实可用额度)` 作为动态上限，精确收紧未显式指定的输出或多图数量，不采用阶梯重试，也不保留固定 0.5 USD 死区。
- GPT-5.5 为 2x，GPT-5.6 为 2.5x，GPT Image 2 为 2x；倍率只应用一次。文字与生图混合仍使用同一个 authorization。
- 最低输出预算为 256 Token；取消固定 10% 安全系数，按最终变换后的文本精确 Token 和附件保守上界预算。允许多图，超出同一 authorization 的剩余 2 USD 预算时只生成可覆盖部分并停止新增图片。
- 当前内层 ChatGPT OAuth 上游不支持官方 `POST /v1/responses/input_tokens`，不能作为生产计费预授权的唯一依赖；内部预授权统一使用本地预算器，公开端点明确返回 501。
- 本次 P0/P1 开发采用隔离双层候选环境：外层 `18081` 使用外层数据库克隆，内层 `18087` 使用内层数据库克隆，两套 Redis 均为空且独立；真实请求只走 `18081 -> 18087`，公网 `18080/18086` 不迁移、不重启、不写入。最终设计见 `docs/ai/context/20260728-162444-openai-billing-atomic-hold-final-design_CN.md`。
- 截至 2026-07-28 08:18，内层 OpenAI OAuth 共 399 个，`active/schedulable` 320 个；`active/schedulable` 不等于已验证支持 `gpt-5.4`。
- 详细事实、状态机、预算公式和账号池规范见最新压缩记忆，不在本文件重复流水。

## 业务红线

- 订阅到期不能联动停用 API Key，因为有效流量卡必须继续可用。
- 已有有效订阅时只允许购买相同 `group_id` 续费；不同 `group_id` 必须先退款，不创建订单、不扣余额、不自动切换。
- `29 元订阅池` 对应 `group_id=2 / codex-pool-19-usd`；`79 元订阅池` 对应 `codex-pool-69-usd`。
- CLIProxyAPI 是聚合上游，不是静态 OpenAI Key；若重新接入必须使用 pool mode，并正确处理 401/403/429 failover。
- 不在文档、提交或日志中记录完整 API Key、OAuth token、内部 token、HMAC secret、SMTP 密码、支付密钥。
- 修改运行态 DB、Redis、容器、Nginx 或公网链路前，必须写计划、备份、验证备份可读并明确回滚边界。
- 不删除或重建 PostgreSQL/Redis 数据容器和 volume，除非用户明确授权且已核对精确目标。

## 维护规则

- 长期上下文只在 `docs/ai/context/` 新建文件；不要把流水账再次堆回本文件。
- 合并、提交或收尾前运行 `git ls-files --others --exclude-standard docs/ai/context`，检查未跟踪上下文和敏感信息。
- 不把 `docs/ai/context/` 加回 `.gitignore`；暂不提交的上下文必须在回复中说明。
- 当前 Git `origin=https://github.com/cnYui/sub2api.git`，是用户个人 fork；同步 Wei-Shaw 上游前必须重新核对远端，禁止凭旧记忆直接推送或 merge。
