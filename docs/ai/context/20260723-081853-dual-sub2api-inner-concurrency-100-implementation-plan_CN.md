# 内层上游 100 并发实施计划

> **供智能代理执行：** 按任务顺序执行；每个任务完成后记录验证结果，不得跳过备份、回读或回滚准备。

**目标：** 将 `18086` 固定转发身份的用户级并发从 5 调整为 100，使外层 `18080` 的 100 并发上限能够进入内层账号池。

**架构：** 外层继续以真实用户 Key 执行鉴权和计费，转发给内层的固定 Key。内层只更新该 Key 所属用户的 `concurrency` 字段；内层 OAuth 账号的 `concurrency`、优先级和调度器均保持不变。

**技术栈：** Sub2API 管理 API、Docker PostgreSQL 备份、PowerShell、现有 Redis 并发计数与 usage fact 验证。

---

## 文件范围

- 新增：`docs/ai/context/20260723-081853-dual-sub2api-inner-concurrency-100-implementation-plan_CN.md`
- 新增（不纳入 Git）：`backups/20260723-081853-upstream-18086-before-concurrency-100.dump`
- 修改：内层运行态 PostgreSQL 中固定转发身份（`users.id=1`）的 `concurrency` 值。
- 不修改：外层和内层任何 Go 源码、Docker Compose、账号凭证、API Key、账号池字段或计费表。

## 任务 1：确认目标身份并备份内层数据库

**文件：**

- 新增：`backups/20260723-081853-upstream-18086-before-concurrency-100.dump`

- [ ] **步骤 1：读取目标用户和转发 Key 的非敏感字段**

运行：

```powershell
docker exec sub2api-upstream-postgres sh -lc 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -At -F "|" -c "SELECT u.id, u.email, u.concurrency, k.id, k.name FROM users u JOIN api_keys k ON k.user_id = u.id;"'
```

预期：仅返回一条记录，用户 ID 为 `1`，当前并发为 `5`；命令不得查询或输出 Key 原文、密码或 OAuth 凭证。

- [ ] **步骤 2：在数据库容器中生成 custom 格式备份**

运行：

```powershell
New-Item -ItemType Directory -Force backups | Out-Null
docker exec sub2api-upstream-postgres sh -lc 'pg_dump -U "$POSTGRES_USER" -Fc "$POSTGRES_DB" > /tmp/upstream-18086-before-concurrency-100.dump'
docker cp sub2api-upstream-postgres:/tmp/upstream-18086-before-concurrency-100.dump backups/20260723-081853-upstream-18086-before-concurrency-100.dump
```

预期：本地 `backups/` 下存在非空 dump 文件。该文件只作本机回滚用途，不得加入 Git、日志或对外传输。

- [ ] **步骤 3：验证备份可读**

运行：

```powershell
docker cp backups/20260723-081853-upstream-18086-before-concurrency-100.dump sub2api-upstream-postgres:/tmp/upstream-18086-before-concurrency-100-verify.dump
docker exec sub2api-upstream-postgres sh -lc 'pg_restore -l /tmp/upstream-18086-before-concurrency-100-verify.dump | sed -n "1,5p"'
```

预期：输出 PostgreSQL archive 的目录条目，命令返回 `0`。

## 任务 2：通过内层管理 API 更新固定转发身份

**文件：**

- 修改：内层 PostgreSQL `users.id=1` 的 `concurrency`。
- 参考：[user_handler.go](D:/CodeWorkSpace/sub2api-upstream-latest/backend/internal/handler/admin/user_handler.go) 的 `Update`；[admin.go](D:/CodeWorkSpace/sub2api-upstream-latest/backend/internal/server/routes/admin.go) 的 `PUT /api/v1/admin/users/:id` 路由。

- [ ] **步骤 1：从受保护来源取得内层管理员 Bearer Token**

运行：

```powershell
$adminToken = $env:SUB2API_UPSTREAM_ADMIN_TOKEN
if ([string]::IsNullOrWhiteSpace($adminToken)) {
  $containerEnv = @{}
  (docker inspect sub2api-upstream-latest | ConvertFrom-Json)[0].Config.Env | ForEach-Object {
    $name, $value = $_ -split '=', 2
    $containerEnv[$name] = $value
  }
  $loginBody = @{ email = $containerEnv.ADMIN_EMAIL; password = $containerEnv.ADMIN_PASSWORD } | ConvertTo-Json -Compress
  $login = Invoke-RestMethod -Method Post -Uri 'http://127.0.0.1:18086/api/v1/auth/login' -ContentType 'application/json' -Body $loginBody
  $adminToken = [string]$login.data.access_token
}
if ([string]::IsNullOrWhiteSpace($adminToken)) { throw '未获取到内层管理员访问令牌' }
$headers = @{ Authorization = "Bearer $adminToken"; 'Content-Type' = 'application/json' }
```

预期：令牌和本地管理员密码只保存在当前 PowerShell 进程变量中，不写入脚本、文档、命令历史或响应输出。

- [ ] **步骤 2：优先通过管理 API 提交仅包含并发字段的更新请求**

运行：

```powershell
$response = Invoke-RestMethod -Method Put -Uri 'http://127.0.0.1:18086/api/v1/admin/users/1' -Headers $headers -Body '{"concurrency":100}'
$response.data | Select-Object id,email,concurrency,rpm_limit,status
```

预期：HTTP 200，返回对象的 `id` 为 `1`、`concurrency` 为 `100`；RPM、状态和其他用户字段保持原值。

- [ ] **步骤 3：管理 API 无法认证时，执行受控单字段数据库更新**

仅当步骤 1 因本地初始管理员密码已过期或未配置而无法获取 Token 时执行；不得尝试猜测密码、重置管理员密码或修改 JWT 密钥。

运行：

```powershell
docker exec sub2api-upstream-postgres sh -lc 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -v ON_ERROR_STOP=1 -At -F "|" -c "UPDATE users SET concurrency = 100 WHERE id = 1 AND concurrency = 5 RETURNING id, concurrency;"'
```

预期：仅返回 `1|100`。返回零行时立即停止，说明目标用户或前置并发状态已变化；不得执行第二次无条件 UPDATE。

- [ ] **步骤 4：回读数据库，并在可用时回读内层管理 API**

运行：

```powershell
docker exec sub2api-upstream-postgres sh -lc 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -At -c "SELECT id, concurrency FROM users WHERE id = 1;"'
```

预期：数据库显示 `1|100`。若已获取管理员 Token，再执行下列可选回读：

```powershell
$readback = Invoke-RestMethod -Method Get -Uri 'http://127.0.0.1:18086/api/v1/admin/users/1' -Headers $headers
$readback.data | Select-Object id,concurrency,current_concurrency
```

可选回读也应显示并发 `100`；数据库回读是本次受控更新的必要验证。

## 任务 3：验证并发边界、计费归属和调度不变性

**文件：**

- 不新增代码文件；验证结果在后续 result 文档记录。

- [ ] **步骤 1：确认内层账号池字段未被改变**

运行：

```powershell
docker exec sub2api-upstream-postgres sh -lc 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -At -F "|" -c "SELECT id, platform, concurrency, priority, schedulable FROM accounts ORDER BY id;"'
```

预期：22 个 OpenAI OAuth 账号的并发、优先级、`schedulable` 值与变更前一致；不能出现将单账号并发批量改为 100 的记录。

- [ ] **步骤 2：以外层真实用户 Key 进行受控并发冒烟**

运行：

```powershell
$smokeAPIKey = $env:SUB2API_SMOKE_API_KEY
if ([string]::IsNullOrWhiteSpace($smokeAPIKey)) { throw '缺少 SUB2API_SMOKE_API_KEY' }
1..10 | ForEach-Object -Parallel {
  curl.exe --silent --show-error --fail-with-body -X POST 'http://127.0.0.1:18080/v1/responses' `
    -H "Authorization: Bearer $using:smokeAPIKey" `
    -H 'Content-Type: application/json' `
    -d '{"model":"gpt-5.4-mini","input":"reply with ok"}'
} -ThrottleLimit 10
```

预期：请求不再因内层用户并发 `5` 而在第 6 个请求起被拒绝；上游临时限流或模型错误应按既有错误契约呈现，不得伪造成功。

- [ ] **步骤 3：缺少真实用户 Key 时，以内部转发 Key 直接验证内层 10 路并发**

此步骤只验证 `18086` 的入口并发，不替代外层真实用户计费验证；内部 Key 只能在当前 PowerShell 进程内存中使用，禁止输出或写入磁盘。

运行：

```powershell
$credentialsJson = docker exec sub2api-postgres-dev sh -lc 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -At -c "SELECT credentials FROM accounts WHERE id = 2;"'
$forwarderKey = [string](($credentialsJson | ConvertFrom-Json).api_key)
if ([string]::IsNullOrWhiteSpace($forwarderKey)) { throw '外层账号未配置内层转发 Key' }
$requestBody = '{"model":"gpt-5.4-mini","input":"reply with ok"}'
$results = 1..10 | ForEach-Object -Parallel {
  $response = Invoke-WebRequest -Method Post -Uri 'http://127.0.0.1:18086/v1/responses' -Headers @{ Authorization = "Bearer $using:forwarderKey" } -ContentType 'application/json' -Body $using:requestBody -SkipHttpErrorCheck -TimeoutSec 120
  [PSCustomObject]@{ status_code = [int]$response.StatusCode }
} -ThrottleLimit 10
$results | Group-Object status_code | Select-Object Name,Count
```

预期：10 个请求均能越过原先用户级并发 5 的入口限制；若出现上游 429/5xx，应按账号或上游错误处理，不得出现“用户并发限制为 5”类错误。

- [ ] **步骤 4：确认计费和账号调度归属**

运行：

```powershell
docker exec sub2api-postgres-dev sh -lc 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -At -F "|" -c "SELECT user_id, api_key_id, account_id, billing_status FROM usage_facts ORDER BY id DESC LIMIT 10;"'
docker exec sub2api-upstream-postgres sh -lc 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -At -F "|" -c "SELECT user_id, api_key_id, account_id FROM usage_logs ORDER BY id DESC LIMIT 10;"'
```

预期：外层记录保留真实用户/API Key 与外层账号；内层记录归属固定转发身份，且 `account_id` 可分散到多个 OAuth 账号。

## 任务 4：准备可逆回滚并归档结果

**文件：**

- 新增：`docs/ai/context/20260723-082000-dual-sub2api-inner-concurrency-100-result_CN.md`

- [ ] **步骤 1：验证回滚命令的范围**

运行：

```powershell
$rollbackBody = '{"concurrency":5}'
Write-Output '回滚仅在验证失败或明确需要降载时执行：PUT /api/v1/admin/users/1 with concurrency=5'
```

预期：回滚只改同一用户的一个字段；不得通过整库恢复覆盖正常运行期间新增的 usage 事实。

- [ ] **步骤 2：归档实施结果**

记录：变更前后并发值、备份验证结果、并发冒烟结果、外层计费归属、内层账号分散情况、回滚命令和未执行的范围外操作。

- [ ] **步骤 3：提交文档变更**

运行：

```powershell
git add docs/ai/context/20260723-081853-dual-sub2api-inner-concurrency-100-implementation-plan_CN.md docs/ai/context/20260723-082000-dual-sub2api-inner-concurrency-100-result_CN.md
git commit -m "docs: record inner upstream concurrency change"
```

预期：仅提交本次新增设计、计划和结果文档；备份文件与现有工作区其他改动保持不纳入暂存区。
