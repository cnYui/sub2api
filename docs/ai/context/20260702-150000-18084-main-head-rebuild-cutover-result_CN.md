# 18080 切回 18084 main HEAD 重建结果

> 承接 `20260626-224349-18080-18084-cutover-and-old-18080-retired-result_CN.md`：用户要求"用本地 main HEAD 真实代码"跑 18084，不依赖 hub latest。

## 时间线

- 2026-07-02 13:30 JST：Docker Desktop GUI 重启（之前 docker build 因 ENOSPC 死锁导致 daemon 排队）
- 2026-07-02 13:42 JST：清理 root 盘 22GB+（docker build cache 20.74GB + 17 张历史 sub2api 镜像 + 13 个 stopped 容器 + 20 个 unused volumes）
- 2026-07-02 14:43 JST：磁盘从 7.8Gi → 34Gi 可用
- 2026-07-02 14:50 JST：发现 `weishaw/sub2api:latest` 是 hub 上别人 build 的 `0.1.138`（commit 占位 `docker`），不是 main HEAD `6ef887a8d`
- 2026-07-02 14:55 JST：本地源码 `docker build` 出 `sub2api-main-preview:20260702-local-main-6ef887a8d` (147MB)
- 2026-07-02 14:58 JST：候选 compose 改用新镜像，18084 容器起来，commit 校验通过
- 2026-07-02 15:00 JST：nginx 切流 18080 → 18084，公网入口走 main HEAD 镜像

## 当前公网拓扑（2026-07-02 15:00 JST）

```
Cloudflare Tunnel → nginx 127.0.0.1:8080 → sub2api-candidate 127.0.0.1:18084
                                                ↓
                                      CLIProxyAPI 127.0.0.1:8317
                                                ↓
                                            OpenAI 上游
```

| 容器 | 状态 | 镜像 | 端口 |
| --- | --- | --- | --- |
| `sub2api-candidate` | Up 4min (healthy) | `sub2api-main-preview:20260702-local-main-6ef887a8d` (commit: 6ef887a8d) | 127.0.0.1:18084 |
| `sub2api-candidate-postgres` | Up 4min (healthy) | `postgres:18-alpine` | 5432 |
| `sub2api-candidate-redis` | Up 4min (healthy) | `redis:8-alpine` | 6379 |
| `sub2api` (18080) | Up 3h+ (healthy) | `weishaw/sub2api:latest` (回滚用) | 127.0.0.1:18080 |
| `sub2api-postgres` (18080) | Up 3h+ (healthy) | `postgres:18-alpine` | 5432 |
| `sub2api-redis` (18080) | Up 3h+ (healthy) | `redis:8-alpine` | 6379 |

## 验证结果

| 探测 | 结果 |
| --- | --- |
| `127.0.0.1:18084/health` | `200 {"status":"ok"}` 5.5ms |
| `127.0.0.1:18080/health` | `200` (应急回滚入口) |
| `https://aaccx.pw/purchase` | `200` 181ms |
| `https://api.aaccx.pw/health` | `200` 149ms |
| `127.0.0.1:18084/v1/chat/completions` (xiaobianfuai key, gpt-5.5) | `200` 8.6s，真实 LLM 回答 |
| `https://aaccx.pw/v1/chat/completions` (公网入口，已切到 18084) | `200` 9.2s，真实 LLM 回答 |
| `sub2api-candidate --version` | `Sub2API local-main-6ef887a8d (commit: 6ef887a8d, built: 2026-07-02)` |

## 关键变更

### 1. 候选 worktree 镜像指向
`/Users/wujianxiang/CodeSpace/sub2api/.worktrees/codex-sub2api-candidate-rehearsal-20260626/deploy/.env.candidate.local`:
- `CANDIDATE_IMAGE=sub2api-main-preview:20260702-local-main-6ef887a8d`（旧值 `sub2api-candidate:20260626-220602-payment-template-30e66c82580f` 镜像已被清理时删掉）

### 2. 18084 候选 compose
- `sub2api-candidate` 容器用新镜像跑，commit `6ef887a8d`
- DB 还是候选 worktree `deploy/candidate/postgres_data`（47 users / 40 api_keys / 191 migrations）
- env `SECURITY_URL_ALLOWLIST_ALLOW_INSECURE_HTTP=true / ALLOW_PRIVATE_HOSTS=true`（之前 22:43 修正过）

### 3. Nginx 切流
- `cliproxy.conf` / `aaccx-root.conf`：6+9 个 `proxy_pass 18080` → `18084`，0 个 18080 残留
- `nginx -t` ok + `nginx -s reload` 完成
- 备份：`/tmp/cliproxy.conf.bak.1782964135`（18084→18080 中间状态）和 `/tmp/aaccx-root.conf.bak.1782964136`

## 旧 18080 角色

- 仍跑 `weishaw/sub2api:latest` (0.1.138, commit: docker, hub latest)
- DB 仍跑（`sub2api-postgres`），里面是 55 users / 45 api_keys
- 不接公网，但容器还活着 → nginx 切回 18080 即可 1 秒回滚

## 镜像信息

- 新建：`sub2api-main-preview:20260702-local-main-6ef887a8d` (147MB, sha256:d0473a1a9988)
- build-arg：`VERSION=local-main-6ef887a8d COMMIT=6ef887a8d DATE=2026-07-02 GOPROXY=https://goproxy.cn,direct GOSUMDB=sum.golang.google.cn`
- Dockerfile：`/Users/wujianxiang/CodeSpace/sub2api/Dockerfile`（含 frontend pnpm build + go build + 静态资源嵌入）

## 风险与提示

- 候选 DB 仍用 6/26 那次预演的 clone（`candidate/postgres_data`），跟 18080 的 18080 DB（候选库 dump 灌入 deploy_postgres_data）理论上同源，但 migrate 数已不是同代（191 vs 188）；两边用 admin 账号同邮箱 admin@sub2api.local / xiaobianfuai@gmail.com，密码哈希可能不一致。如果 18084 登录不上 admin 端，可改用 xiaobianfuai 的 LOCAL Key 直调 API（完整 Key 已脱敏，已验证可走 `/v1/chat`）。
- `weishaw/sub2api:latest` 是 7/2 04:44 UTC 别人 build 的 hub 镜像，不是 main HEAD；后续如长期保留 18080 跑 hub latest，发布到 latest 之前需要打 main HEAD 的 tag 并 push 覆盖。
- `sub2api-candidate` compose `restart: no` 是预演约定，宕机不会自启；如要长期以候选形态对外，建议改 `restart: unless-stopped`。
