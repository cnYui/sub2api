# 自动套餐退款公网发布结果

时间：2026-07-09 10:51 JST

## 结论

已将本地 `main` 当前工作区的自动套餐退款代码发布到公网 18084 应用容器。

本次镜像同时包含：

- 用户侧自动套餐退款。
- `/v1/responses` 收到 `messages` 且无 `input` 时本地返回 400 的防御。
- Docker 前端构建固定 `pnpm@10.34.4`，避免 `pnpm@latest` 规则变化导致 frozen install 失败。

## 发布信息

- 发布前 HEAD：`d4fae0839`
- 新镜像：`sub2api-candidate:20260709-102735-d4fae0839-auto-refund`
- image id：`sha256:2636c36142c0eef03ef73d303a4feef7dd0385b52e54dcfdf5748b83fdeb7715`
- 旧应用容器：`sub2api-candidate-before-promote-20260709-103251`
- 新应用容器：`sub2api-candidate`
- 只替换应用容器，未重建 Postgres、Redis、nginx 或 Cloudflare Tunnel。

## 构建修复

首次 Docker 构建失败于前端依赖安装：

```text
ERR_PNPM_LOCKFILE_CONFIG_MISMATCH
```

根因是 Dockerfile 使用 `pnpm@latest`，新版 pnpm 不再读取 `package.json` 中的 `pnpm.overrides` 字段，而当前 lockfile 仍记录该 overrides 配置。已将 Dockerfile 固定为 `pnpm@10.34.4`，并在 `frontend/package.json` 增加 `packageManager`，随后 Docker 构建通过。

## 验证

- `go test -count=1 -tags=unit ./internal/service`
- `GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/handler`
- `go test -count=1 ./cmd/server`
- `pnpm vitest run src/views/user/__tests__/paymentRefund.spec.ts src/views/user/__tests__/paymentUx.spec.ts`
- `pnpm typecheck`
- `git diff --check`
- Docker production build：通过，产出新镜像。
- `deploy/promote-sub2api-candidate.sh --candidate-image ... --dry-run --yes`：确认只替换 18084 应用容器。
- 正式发布后 `http://127.0.0.1:18084/health`：通过。

## 未执行

- 未执行真实用户订单退款。
- 未推送远端。
- 未创建 PR。
