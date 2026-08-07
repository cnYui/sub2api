# 最终计费倍率调整为 18 倍实施计划

> **供执行代理使用：** 必须逐项完成并在执行时更新复选框。由于工作区已有用户未提交改动，本计划不创建 Git 提交。

**目标：** 将 18082 的服务端最终扣费倍率持久化为 18，使后续请求按 `标准成本 × 分组倍率 × 18` 结算。

**架构：** 最终倍率由 Compose 向应用容器注入 `BILLING_FINAL_MULTIPLIER`。仅替换应用容器即可载入新环境变量；数据库、Redis 与数据卷不参与发布。模型广场继续使用不含最终倍率的展示价格。

**技术栈：** Docker Compose、Go 应用配置、PowerShell。

---

## 文件结构

- 修改 `deploy/docker-compose.18082.yml`：18082 应用容器的最终扣费倍率。
- 新增 `docs/ai/context/20260807-114501-final-billing-multiplier-18-deploy-result_CN.md`：实际发布和验证证据。
- 修改 `AGENTS.md`：保存该次已完成的运行态配置上下文。

### 任务 1：更新并静态验证 Compose 配置

**文件：**

- 修改：`deploy/docker-compose.18082.yml:9`

- [ ] **步骤 1：将最终倍率设为 18**

将环境变量保持为唯一的最终倍率来源：

```yaml
environment:
  - BILLING_FINAL_MULTIPLIER=18
  - BILLING_MINIMUM_BALANCE_RESERVE=0.01
```

- [ ] **步骤 2：验证渲染后的应用服务环境变量**

运行：

```powershell
docker compose -f deploy/docker-compose.dev.yml -f deploy/docker-compose.18082.yml config
```

预期：`sub2api` 服务的 `environment` 包含 `BILLING_FINAL_MULTIPLIER=18`，且服务名、端口、数据卷和其他环境变量不变。

### 任务 2：只重建应用容器并核验运行态

**文件：**

- 不修改文件：此任务只改变 `sub2api-official-18082` 应用容器的运行状态。

- [ ] **步骤 1：仅替换应用容器**

运行：

```powershell
docker compose -f deploy/docker-compose.dev.yml -f deploy/docker-compose.18082.yml up -d --force-recreate --no-deps sub2api
```

预期：仅应用容器被替换；PostgreSQL、Redis 和数据卷不会被重建。

- [ ] **步骤 2：确认容器健康、环境变量和服务健康接口**

运行：

```powershell
docker inspect --format '{{.State.Status}} {{if .State.Health}}{{.State.Health.Status}}{{end}}' sub2api-official-18082
docker inspect --format '{{range .Config.Env}}{{println .}}{{end}}' sub2api-official-18082
Invoke-WebRequest -UseBasicParsing http://127.0.0.1:18082/health
```

预期：容器状态为 `running healthy`，环境变量包含 `BILLING_FINAL_MULTIPLIER=18`，健康接口返回 HTTP 200 和 `{"status":"ok"}`。

### 任务 3：记录审计结果

**文件：**

- 新增：`docs/ai/context/20260807-114501-final-billing-multiplier-18-deploy-result_CN.md`
- 修改：`AGENTS.md`

- [ ] **步骤 1：新增不可覆盖的执行记录**

记录修改前后的倍率、实际重建的容器范围、运行态环境变量、健康检查结果，以及“历史账单不重算、模型广场不叠加最终倍率”的不变项。

- [ ] **步骤 2：追加项目长期上下文**

在 `AGENTS.md` 的“模型最终计费倍率”段落追加本次结论，并引用执行记录文件名，确保后续操作可恢复配置决策。

- [ ] **步骤 3：复查变更范围**

运行：

```powershell
git diff --check
git diff -- deploy/docker-compose.18082.yml AGENTS.md
```

预期：无空白错误；仅有最终倍率配置与本次新增上下文记录的相关变更，其他用户已有改动保持不变。
