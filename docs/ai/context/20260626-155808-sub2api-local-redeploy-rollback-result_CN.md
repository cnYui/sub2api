# Sub2API 本地重部署回滚结果

## 时间

- 触发：2026-06-26 15:50 部署计划
- 失败发现：15:54 起新容器 `Restarting (1)`，本地 18080 不可用，公网 `/health` 502
- 回滚完成：15:58 左右

## 背景

按 `20260626-155017-sub2api-local-redeploy-plan_CN.md` 执行 `sub2api-local-redeploy`：
重建 `weishaw/sub2api:latest` 并替换运行态 `sub2api` 容器。

## 部署失败现象

新镜像 `weishaw/sub2api:latest`（id `f668024ff119`，15:54:24 构建）替换容器后，进程每次启动都
在 `Failed to initialize application` 处返回 exit 1，Docker 自动进入 backoff 重启循环：

```
migration 155_seed_codex_subscription_plans_baseline.sql checksum mismatch
db   = 0e2d20c620783bf91cbf5ffb524edb46730e998a10799f66dd03e988a32b0b8f
file = 64c22df919959e4afef129f8aacd2785254215cee2888f2ec162f65c36d1921f
This means the migration file was modified after being applied to the database.
```

`127.0.0.1:18080/health` 无监听，`https://api.aaccx.pw/health` 在 nginx 后返回 502。

## 根因

`backend/internal/repository/migrations_runner.go:155-195` 对已应用的 SQL migration 计算 SHA256
并比对 `schema_migrations.checksum`。白名单 `migrationChecksumCompatibilityRules`（同文件
第 68-80 行）覆盖 054/061/109/110/112/115/116/118/119/120/123，但 `155_seed_codex_subscription_plans_baseline.sql` 不在其中。
当前 main 上 fadfe9c7f 的 155 文件 hash 是 `64c22df9…`，与 DB 历史记录 `0e2d20c6…` 不匹配，
且本次部署属于运行时污点（不在白名单）→ 启动失败。

## 临时回滚动作

1. `docker compose ... stop sub2api`（停止死循环容器）。
2. `docker tag weishaw/sub2api:rollback-20260621-182042 weishaw/sub2api:latest`。
3. `docker compose ... up -d --no-deps --force-recreate sub2api`。
4. 25 秒后健康检查通过；本地 18080 与公网 api.aaccx.pw `/health` 均 200。
5. 当前运行容器镜像 id：`423e0593979c` (= `rollback-20260621-182042`)。

未修改 `deploy/docker-compose.local.yml`、`deploy/.env.scheme-a.local`、任何迁移文件或 git 状态。
下次 `sub2api-local-redeploy` 会再次把 `:latest` 指向新构建的 `f668024ff119`（仍会失败）。

## 验证

- `docker ps --filter name=sub2api`：`Up 32 seconds (healthy)`，端口 `127.0.0.1:18080->8080`。
- `curl http://127.0.0.1:18080/health` → 200。
- `curl https://api.aaccx.pw/health` → 200。
- 容器日志里 `/v1/responses`、`/api/v1/auth/me`、`/subscriptions` 路径返回 200，公网流量已恢复。
- 前端资源 hash 变更为 `pkg-vue-BqGtxt06.js` / `index-DKtOnsbF.js` / `index-BMta9z_W.css`（回滚版本）。

## 待办（用户决定后再做）

1. 把 `0e2d20c6…`（旧）或 `64c22df9…`（新）加入
   `migrationChecksumCompatibilityRules["155_seed_codex_subscription_plans_baseline.sql"]`，
   或者将 DB `schema_migrations` 的 checksum 更新为 `64c22df9…`，再重新部署。
2. 也可以选择删除 fadfe9c7f 中 155 文件的修改，回到 DB 记录的旧版内容（但这与当前已合并的
   "99 元 + baseline" 套餐基线冲突）。
3. 在决定之前不要再跑 `sub2api-local-redeploy`，否则容器会再次回到 `Restarting (1)` 循环。
