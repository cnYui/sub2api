# 本地改动整合与公网发布边界

## 盘点结论

- 当前工作区分支为 `main`，所有具名本地分支的 tip 都已是 `main` 祖先，没有待执行的分支合并。
- 两个附加工作树没有未合并提交；billing 工作树的结算差异已经存在于当前 `main`，不重复移植。
- stash 中的未跟踪上下文文档与工作区文件哈希一致，不重复应用。
- 本地 `main` 相对 `upstream/main` 为 `ahead 45 / behind 51`。本次按管理员要求发布本地 `main`，不拉取上游历史。

## 纳入范围

主工作区的监控目录探测实现、208 迁移、监控测试、购买页和 API 端点页面改动，以及现有 `docs/ai/context/` 未跟踪文档全部纳入提交。前端端点复制固定使用 `https://api.aaccx.pw/v1`；监控只执行带鉴权的 `GET /v1/models`。

## 排除范围

`backend/.restore-account-1128` 是约 78 MB 的本地 ELF 恢复二进制，恢复工具源码含本地凭证备用逻辑；两者均不提交、不进入 Docker 构建上下文，但保留在工作区。billing 工作树的 `deploy/docker-compose.production-billing-fix.yml` 固定 15 倍且与当前 16 倍生产配置冲突，仅作为临时文件排除，不删除原文件。

## 验证

- 前端端点、监控指标回归：14 项测试通过。
- billing 仓储测试通过。
- 监控服务、管理处理器、仓储的 `-tags unit` 定向测试通过。
- 默认 Go 包集合仍有两个与本次改动无关的既有失败：`TestContentModerationRuntimeSnapshotRefreshFailureKeepsStaleConfig`、`TestEstimateOpenAIInputTokens_CompareWithOpenAIAPI`；不能将整包结果表述为全绿。

## 发布原则

构建 `deploy-sub2api:latest` 后仅替换 `sub2api-official-18082` 应用容器；PostgreSQL、Redis、Nginx、Cloudflare Tunnel 和数据卷不重建。发布后核对运行时 `BILLING_FINAL_MULTIPLIER=16`、容器健康状态、本地入口、Nginx 和三个公网域名。
