# 管理员撤销订阅后二次购买拦截修复结果

## 背景

用户反馈管理员在 `Subscriptions` 页面撤销用户套餐后，用户仍然无法二次购买。前置个案是 `xinlise@gmail.com` 的退款失败与管理员撤销：旧订阅只写入 `deleted_at`，但 `status` 仍为 `active`，同时用户购买页存在短时活跃订阅缓存，导致撤销后仍可能被“一用户当前只能存在一个套餐”的保护拦截。

## 根因

- 管理员撤销入口 `DELETE /api/v1/admin/subscriptions/:id` 调用 `SubscriptionService.RevokeSubscription()`。
- 原逻辑只调用 `userSubRepo.Delete()`；`UserSubscription` 使用 Ent `SoftDeleteMixin`，因此删除会写 `deleted_at`，不会改 `status`。
- 这会留下 `deleted_at IS NOT NULL AND status='active' AND expires_at > now()` 的脏状态。
- 用户购买页的 active subscriptions 缓存可能在管理员撤销后仍保留旧 active 结果，导致前端本地先弹 `ACTIVE_SUBSCRIPTION_EXISTS`。

## 代码改动

- `backend/internal/service/subscription_service.go`
  - `RevokeSubscription()` 在软删除前先调用 `UpdateStatus(subscriptionID, expired)`。
  - 管理员撤销现在会真实写库：先把订阅状态置为 `expired`，再设置 `deleted_at`。
- `backend/internal/service/subscription_revoke_test.go`
  - 新增 `TestRevokeSubscriptionMarksExpiredBeforeSoftDelete`，覆盖撤销写入顺序。
- `backend/internal/service/subscription_assign_idempotency_test.go`
  - 测试桩补齐 `UpdateStatus`。
- `frontend/src/views/user/PaymentView.vue`
  - 订阅套餐选择、弹窗套餐选择、确认订阅前强制 `fetchActiveSubscriptions(true)`。
  - 刷新失败时保留当前缓存判断，并继续让后端购买保护兜底。
- `frontend/src/views/user/__tests__/PaymentView.spec.ts`
  - 新增 stale active subscription cache 回归测试。

## 运行态处理

- 发布前备份 Postgres：
  - `deploy/backups/20260710-213756-sub2api-candidate-before-admin-subscription-revoke-repurchase-deploy.dump`
  - 已用容器内 `pg_restore -l` 验证可读。
- 发布前备份 Redis：
  - `deploy/backups/20260710-213756-sub2api-candidate-redis-before-admin-subscription-revoke-repurchase-deploy.rdb`
  - `redis-cli SAVE` 返回 `OK`，复制后文件存在。
- 构建镜像：
  - `sub2api-candidate:20260710-214000-a575f28c1-admin-sub-revoke-repurchase`
  - image id `sha256:70d71315ceeebf7447e70ee3fae87047e013b9529feee5113ea513e5240888f5`
- 已发布到 `public_candidate_18084`：
  - 旧容器重命名为 `sub2api-candidate-before-promote-20260710-214730`
  - 新容器 `sub2api-candidate` 使用新镜像，状态 `healthy`
  - 未重建 Postgres、Redis、nginx、Cloudflare Tunnel
  - `/app/data` 仍挂载 `/Users/wujianxiang/CodeSpace/sub2api/.worktrees/codex-sub2api-candidate-rehearsal-20260626/deploy/candidate/data`

## 历史脏数据清理

发布后只读扫描发现全库还有 8 条历史残留：

```sql
deleted_at IS NOT NULL
AND status = 'active'
AND expires_at > NOW()
```

这些行均已是软删除订阅，业务语义上不应再算 active。已在发布前备份保护下统一更新为 `status='expired'`，返回行：

- `id=1/user_id=2/group_id=2`
- `id=3/user_id=5/group_id=2`
- `id=26/user_id=7/group_id=2`
- `id=54/user_id=24/group_id=3`
- `id=46/user_id=26/group_id=4`
- `id=49/user_id=31/group_id=2`
- `id=61/user_id=44/group_id=2`
- `id=68/user_id=48/group_id=3`

对应 Redis `billing:sub:{user_id}:{group_id}` 缓存键执行 `DEL`，返回 `0`，说明当时没有残留缓存。复核脏状态计数为 `0`。

## `xinlise@gmail.com` 当前状态

- `user_subscriptions.id=88/group_id=8/codex-pool-89-usd`：`expired`，保留 `deleted_at=2026-07-10 16:27:50.204049+08`
- `user_subscriptions.id=98/group_id=12/codex-pool-179-usd`：`active`，这是用户后续新买的 199 元套餐，本次未撤销

## 验证

- `GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service`：通过，`90.042s`
- `pnpm vitest run src/views/user/__tests__/PaymentView.spec.ts`：通过，`33 tests`
- `pnpm typecheck`：通过
- `git diff --check`：通过
- `docker build -t sub2api-candidate:20260710-214000-a575f28c1-admin-sub-revoke-repurchase --build-arg COMMIT=a575f28c1 -f Dockerfile .`：通过
- `DEPLOY_TARGET=public_candidate_18084 deploy/promote-sub2api-candidate.sh --candidate-image sub2api-candidate:20260710-214000-a575f28c1-admin-sub-revoke-repurchase --dry-run --yes`：目标正确
- `DEPLOY_TARGET=public_candidate_18084 deploy/promote-sub2api-candidate.sh --candidate-image sub2api-candidate:20260710-214000-a575f28c1-admin-sub-revoke-repurchase --yes`：通过
- `curl http://127.0.0.1:18084/health`：`{"status":"ok"}`
- `curl https://api.aaccx.pw/health`：`{"status":"ok"}`

## 后续注意

- 管理员撤销真实写库已由代码保证：`status=expired` 与 `deleted_at` 都会落库。
- 现有历史“软删但 active”脏订阅已清零。
- 前端购买页不再盲信 60 秒缓存，购买前会强制刷新一次 active subscriptions。
