# 18082 最终计费倍率调整为 20 倍实施计划

> **执行说明：** 当前工作区已有未提交改动，本次不创建 Git 提交，只修改本次倍率相关配置和新增审计记录。

**目标：** 将 18082 后续模型请求的服务端最终扣费倍率从 `18x` 持久化调整为 `20x`。

**架构：** Compose 覆盖文件向应用容器注入 `BILLING_FINAL_MULTIPLIER`。仅使用 `--force-recreate --no-deps sub2api` 替换应用容器，PostgreSQL、Redis 和数据卷保持不变。

**技术栈：** Docker Compose、PowerShell、Go 配置测试。

---

## 任务 1：更新并校验配置

**文件：** 修改 `deploy/docker-compose.18082.yml:9`。

- [ ] 将 `BILLING_FINAL_MULTIPLIER=18` 改为 `BILLING_FINAL_MULTIPLIER=20`。
- [ ] 运行以下命令并确认输出包含 `BILLING_FINAL_MULTIPLIER: "20"`：

```powershell
docker compose -f deploy/docker-compose.dev.yml -f deploy/docker-compose.18082.yml config
```

## 任务 2：重建并验证应用容器

**文件：** 无文件改动，仅更新运行中的应用容器。

- [ ] 仅重建应用服务：

```powershell
docker compose -f deploy/docker-compose.dev.yml -f deploy/docker-compose.18082.yml up -d --force-recreate --no-deps sub2api
```

- [ ] 确认 `sub2api-official-18082` 为 `running healthy`，环境变量为 `BILLING_FINAL_MULTIPLIER=20`。
- [ ] 确认 `http://127.0.0.1:18082/health`、`https://aaccx.pw/health`、`https://www.aaccx.pw/health`、`https://api.aaccx.pw/health` 均返回 HTTP 200。
- [ ] 运行倍率解析回归测试：

```powershell
go test -run '^TestLoadBillingFinalMultiplierFromEnvironment$' -count=1 ./internal/config
```

## 任务 3：记录发布结果

**文件：** 新增 `docs/ai/context/20260808-100204-final-billing-multiplier-20-deploy-result_CN.md`，并在 `AGENTS.md` 追加本次决策。

- [ ] 记录变更前后倍率、应用容器替换范围、依赖容器是否保持不变、环境变量和所有健康检查结果。
- [ ] 明确历史账单不追溯重算，模型广场不叠加最终倍率。
- [ ] 运行 `git diff --check`，确认无空白错误。
