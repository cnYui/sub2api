# 周额度降档实施结果

## 范围

- 本轮只处理本地开发环境：公共 Codex 订阅周额度统一降到 `58 / 78 / 118 / 158 / 198 / 299 / 400 USD/周`。
- 未操作公网、生产数据库、Nginx、Cloudflare 或 CLIProxyAPI。
- 保持 28 天有效期、每 7 天按订阅锚点刷新；订单 `subscription_snapshot` 不随本次额度下调改写。

## 代码结果

- 前端购买页、订阅页、顶部进度、Dashboard、Key 用量和管理端相关文案/断言同步为新周额度。
- 后端公共 Codex 分组固定额度映射同步为新周额度。
- 新增前向迁移 `backend/migrations/175_reduce_public_codex_weekly_quota_amounts.sql`，只更新：
  - `groups.weekly_limit_usd/default_validity_days/description`
  - `subscription_plans` 展示文案
  - active 且未过期的 `subscription_entitlement_periods.weekly_limit_usd/period_total_quota_usd`
- `weekly-quota-cutover.sh` 的 dry-run/apply 固定映射同步为新额度；历史超额审计基准从 `72/7` 改为 `58/7`。

## 本地数据库结果

- 已在执行前创建并校验可读备份：
  - `backups/sub2api-dev-before-weekly-quota-amount-reduction-20260722-093231.dump`
- 已在本地 `sub2api-postgres-dev` 执行迁移 `175_reduce_public_codex_weekly_quota_amounts.sql`。
- 迁移结果：
  - `groups` 更新 7 行
  - `subscription_plans` 更新 7 行
  - active entitlement 更新 102 行
- 本地 Redis 已清理 `billing:sub:*` 订阅额度缓存。
- 校验结果：102 条 active 公共 Codex entitlement 全部处于新额度集合。

## 两名历史超额用户

本地当前周窗口均从 `2026-07-22 00:00:00+08` 开始，额度已按新最低档 `58 USD/周` 生效：

| 用户 | 订阅 | 当前周用量 | 今日/本周剩余 |
| --- | --- | ---: | ---: |
| `xunskyler@gmail.com` | `21` | `7.914169 USD` | `50.085831 USD` |
| `luzhiyuan2026@163.com` | `53` | `4.806830 USD` | `53.193170 USD` |

## 本地重启

- 首次 compose 未指定原 project 名，Docker 试图创建同名数据容器并失败，未替换数据容器。
- 已改用原 project `sub2api-localdev` 执行：
  - `docker compose --env-file .env.local-dev -p sub2api-localdev -f docker-compose.dev.yml up -d --build sub2api`
- 当前本地容器：
  - `sub2api-dev` healthy，`127.0.0.1:8080`
  - `sub2api-postgres-dev` healthy
  - `sub2api-redis-dev` healthy
- `http://127.0.0.1:8080/health` 返回 `{"status":"ok"}`。

## 验证

- `cd backend && go test ./...` 通过。
- `cd frontend && pnpm typecheck` 通过。
- `cd frontend && pnpm lint:check` 通过。
- `cd frontend && pnpm exec vitest run --reporter=dot` 通过。
- `cd frontend && pnpm build` 通过。

Vitest/Build 输出中仍有既有的测试期故意错误日志、组件 stub 警告、Browserslist 过期提示和 chunk size 提示，未导致失败。
