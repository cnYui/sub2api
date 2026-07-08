# personal/main 同步结果

## 结果

- 当前本地分支：`main`。
- 已提交 149/199 元订阅套餐改动：
  - commit：`32725ff31 feat: add 149 and 199 subscription plans`
- 已常规推送到远端 `personal/main`：
  - 推送范围：`5c9a6253f..32725ff31`
  - 命令：`git push personal main`
  - 未使用 force push。

## 推送前检查

- 已执行 `git fetch personal main`。
- 推送前 `git rev-list --left-right --count personal/main...main` 为 `0 1`，说明远端没有本地缺失提交，本地只领先 1 个提交。

## 提交内容

- 新增 `backend/migrations/161_seed_codex_149_199_subscription_plans.sql`。
- 新增 149/199 元套餐迁移回归测试。
- 新增购买页 149/199 档位回归测试。
- 更新 `AGENTS.md` 长期记忆。
- 新增套餐设计、计划、结果文档与本次同步计划文档。

## 验证

提交前已通过：

- `cd backend && go test -count=1 ./migrations`
- `cd frontend && pnpm vitest run src/views/user/__tests__/PaymentView.spec.ts`
- `git diff --check`

## 未做事项

- 未构建镜像。
- 未部署公网 18084。
- 未修改公网 DB、Redis、nginx 或 Cloudflare Tunnel。
- 未给公网运行态 `codex-pool-135-usd` 与 `codex-pool-179-usd` 绑定 `cliproxy-local-openai` 上游账号。

## 后续提醒

公网发布后仍需单独验收：

- 公网 DB 已应用 `161_seed_codex_149_199_subscription_plans.sql`。
- `149 元订阅池` 与 `199 元订阅池` 为 `for_sale=true`。
- `cliproxy-local-openai` 已绑定到 `codex-pool-135-usd` 与 `codex-pool-179-usd`。
- 新套餐用户真实请求 `/v1/responses` 能成功落库到对应 group。
