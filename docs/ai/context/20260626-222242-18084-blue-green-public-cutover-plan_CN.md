# 18084 候选环境切换公网执行计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use `sub2api-local-redeploy` for公网相关切换约束；如果开始执行，按本文逐项验证，不要跳步。

**Goal:** 将已验证通过的 `127.0.0.1:18084` 候选 Sub2API 环境切到公网入口，恢复公网使用最新代码和最新正确数据。

**Architecture:** 当前公网入口是 `Cloudflare Tunnel -> nginx 127.0.0.1:8080 -> Sub2API 127.0.0.1:18080`。候选环境是完整独立栈 `sub2api-candidate:18084 + sub2api-candidate-postgres + sub2api-candidate-redis`，已通过页面、支付、健康检查验证。优先采用 Nginx 边缘切流，把公网反代从 `18080` 改到 `18084`，减少镜像回滚和公网容器重建风险；稳定后再做标准化固化。

**Tech Stack:** Docker Compose、PostgreSQL 18、Redis 8、Nginx、Sub2API Go 后端、Vue 前端。

---

## 当前事实

- 公网 app：`sub2api | weishaw/sub2api:latest | 127.0.0.1:18080->8080 | healthy`
- 候选 app：`sub2api-candidate | sub2api-candidate:20260626-220602-payment-template-30e66c82580f | 127.0.0.1:18084->8080 | healthy`
- 公网 DB：`sub2api-postgres`，当前挂载 `/Users/wujianxiang/CodeSpace/sub2api/deploy/postgres_data`
- 候选 DB：`sub2api-candidate-postgres`，当前挂载候选 worktree 下的克隆数据目录
- 公网只读计数：`users=32`、`api_keys=22`、`migrations=188`
- 候选只读计数：`users=47`、`api_keys=40`、`migrations=191`
- 健康检查：`18080 / 18084 / https://api.aaccx.pw/health` 当前均返回 `{"status":"ok"}`

结论：这次不能只替换前端或只替换 app 镜像。候选环境的数据明显比当前公网 DB 更新，公网切换必须明确“候选 DB 从切换点开始成为事实源”。

## 风险边界

- Nginx reload 通常是无中断或极短中断，但正在进行的流式 `/v1/*` 请求仍可能断开。
- 从 `18080` 切到 `18084` 后，切换前在旧公网 DB 上新增的写入不会自动出现在候选 DB 中；执行前需要接受这一点，或先做差异核对和补录。
- 候选环境当前是 rehearsal 命名，不适合长期作为最终生产形态；边缘切流可作为低风险上线方式，但稳定后要固化到标准公网栈。
- 不打印、提交或记录任何 `.env`、JWT secret、支付密钥、API key、商户密钥。

## Task 1: 切换前冻结和备份

**Files:**
- Read: `/opt/homebrew/etc/nginx/servers/cliproxy.conf`
- Read: `/opt/homebrew/etc/nginx/servers/aaccx-root.conf`
- Create: `/Users/wujianxiang/CodeSpace/sub2api/deploy/backups/YYYYMMDD-HHMMSS-public-before-18084-cutover.dump`
- Create: `/Users/wujianxiang/CodeSpace/sub2api/deploy/backups/YYYYMMDD-HHMMSS-candidate-before-18084-cutover.dump`

- [ ] **Step 1: 确认维护窗口**

说明切换会影响公网 `/v1/*` 的长连接请求，窗口建议 1-3 分钟。

- [ ] **Step 2: 创建备份目录**

Run:

```bash
mkdir -p /Users/wujianxiang/CodeSpace/sub2api/deploy/backups
```

Expected: 命令成功退出。

- [ ] **Step 3: 备份旧公网 DB**

Run:

```bash
TS="$(date +%Y%m%d-%H%M%S)"
PATH="/Applications/Docker.app/Contents/Resources/bin:$PATH" \
  docker exec sub2api-postgres sh -lc \
  'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc' \
  > "/Users/wujianxiang/CodeSpace/sub2api/deploy/backups/${TS}-public-before-18084-cutover.dump"
```

Expected: 生成非空 dump 文件。

- [ ] **Step 4: 备份候选 DB**

Run:

```bash
TS="$(date +%Y%m%d-%H%M%S)"
PATH="/Applications/Docker.app/Contents/Resources/bin:$PATH" \
  docker exec sub2api-candidate-postgres sh -lc \
  'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc' \
  > "/Users/wujianxiang/CodeSpace/sub2api/deploy/backups/${TS}-candidate-before-18084-cutover.dump"
```

Expected: 生成非空 dump 文件。

- [ ] **Step 5: 备份 Nginx 配置**

Run:

```bash
TS="$(date +%Y%m%d-%H%M%S)"
cp /opt/homebrew/etc/nginx/servers/cliproxy.conf "/opt/homebrew/etc/nginx/backups/cliproxy.conf.${TS}.before-18084-cutover.bak"
cp /opt/homebrew/etc/nginx/servers/aaccx-root.conf "/opt/homebrew/etc/nginx/backups/aaccx-root.conf.${TS}.before-18084-cutover.bak"
```

Expected: 两个备份文件创建成功。

## Task 2: 边缘切流到 18084

**Files:**
- Modify: `/opt/homebrew/etc/nginx/servers/cliproxy.conf`
- Modify: `/opt/homebrew/etc/nginx/servers/aaccx-root.conf`

- [ ] **Step 1: 修改 Sub2API 反代端口**

将两个 Nginx 配置中的 Sub2API `proxy_pass http://127.0.0.1:18080` 改为 `proxy_pass http://127.0.0.1:18084`。不要修改 yui.web/shop 指向 `4173` 的路由。

- [ ] **Step 2: 校验 Nginx 配置**

Run:

```bash
nginx -t
```

Expected: 输出包含 `syntax is ok` 和 `test is successful`。

- [ ] **Step 3: 热重载 Nginx**

Run:

```bash
nginx -s reload
```

Expected: 命令成功退出。

## Task 3: 切换后验证

**Files:**
- Read: `/opt/homebrew/etc/nginx/servers/cliproxy.conf`
- Read: `/opt/homebrew/etc/nginx/servers/aaccx-root.conf`

- [ ] **Step 1: 验证本地候选健康**

Run:

```bash
curl -fsS http://127.0.0.1:18084/health
```

Expected:

```json
{"status":"ok"}
```

- [ ] **Step 2: 验证公网健康**

Run:

```bash
curl -fsS https://api.aaccx.pw/health
```

Expected:

```json
{"status":"ok"}
```

- [ ] **Step 3: 验证公网 HTML 资源来自候选构建**

Run:

```bash
curl -sS https://aaccx.pw/purchase | rg 'assets/.*\\.(js|css)'
```

Expected: 返回当前候选构建的前端资源引用。

- [ ] **Step 4: 用普通用户验证购买页**

在浏览器打开 `https://aaccx.pw/purchase`，使用普通用户登录，确认订阅续费入口、套餐、流量包、支付宝确认支付入口均存在。

- [ ] **Step 5: 用 API 侧验证 Key 行为**

使用一条脱敏可用的用户 Key 发起 `/v1/models` 或 `/v1/responses` 的最小请求，确认公网 API 不返回异常 `401` 或 `502`。

## Task 4: 回滚方案

**Files:**
- Modify: `/opt/homebrew/etc/nginx/servers/cliproxy.conf`
- Modify: `/opt/homebrew/etc/nginx/servers/aaccx-root.conf`

- [ ] **Step 1: 回滚端口**

将两个 Nginx 配置中刚改过的 `proxy_pass http://127.0.0.1:18084` 改回 `proxy_pass http://127.0.0.1:18080`。

- [ ] **Step 2: 校验并重载**

Run:

```bash
nginx -t && nginx -s reload
```

Expected: Nginx 配置通过并重载成功。

- [ ] **Step 3: 验证回滚入口**

Run:

```bash
curl -fsS http://127.0.0.1:18080/health
curl -fsS https://api.aaccx.pw/health
```

Expected: 两个健康检查都返回 `{"status":"ok"}`。

## Task 5: 稳定后固化

**Files:**
- Modify: `/Users/wujianxiang/CodeSpace/sub2api/deploy/docker-compose.local.yml`
- Modify: `/Users/wujianxiang/CodeSpace/sub2api/deploy/redeploy-sub2api-image.sh`
- Create: `/Users/wujianxiang/CodeSpace/sub2api/docs/ai/context/YYYYMMDD-HHMMSS-18084-blue-green-public-cutover-result_CN.md`

- [ ] **Step 1: 观察公网 15-30 分钟**

观察 API 健康、登录、购买页、支付下单、关键错误日志。若异常率正常，再进入固化。

- [ ] **Step 2: 选择固化方式**

推荐方式：在维护窗口内把候选 DB dump 恢复到标准公网 DB 目标，并用当前已验证代码重新构建 `weishaw/sub2api:latest`，最终让 Nginx 回到标准 `18080`。不推荐长期让公网依赖 rehearsal 命名的容器和 worktree 路径。

- [ ] **Step 3: 写结果文档**

记录切换时间、入口、镜像、DB 备份文件名、验证结果、回滚点。不记录任何密钥或完整 API Key。

## 推荐执行判断

当前推荐先执行 Task 1-3，也就是“备份 + Nginx 切到 18084 + 验证”。这会最快把公网切到已验证的候选 app 和候选 DB，避免再次重建公网镜像或误碰旧公网 DB。稳定后再执行 Task 5，把候选状态固化回标准公网栈。
