# /monitor 渠道模型补全实施计划

> **执行说明：** 本任务是已确认的生产监控配置修正，不修改前端或后端代码；验证步骤以数据库与认证详情接口为准。

**目标：** 将 GPT 0.35 与 GPT 0.1 监控的模型集合补齐，使 `/monitor` 详情弹窗展示每个渠道真实可用的模型。

**架构：** 继续使用 `channel_monitors.primary_model + extra_models` 作为监控集合。通过 PostgreSQL 事务更新 8、9 号监控配置，10 号图像渠道保持单模型；运行中的调度器下次 `RunCheck` 会重新读取监控配置，无需重建容器。

**技术栈：** PostgreSQL 18、运行中的 Sub2API 18082 实例、Gin 用户详情接口。

---

### 任务 1：保存变更前快照

**文件/资源：**
- 读取：`channel_monitors` 表中的 8、9、10 号记录
- 新增：`docs/ai/context/` 本次执行记录（包含脱敏后的配置快照）

- [ ] **步骤 1：执行只读快照查询**

```powershell
docker exec sub2api-official-18082-postgres psql -U sub2api -d sub2api -c "SELECT id,name,primary_model,extra_models,enabled,updated_at FROM channel_monitors WHERE id IN (8,9,10) ORDER BY id;"
```

预期：8、9 号 `extra_models` 为空数组，10 号仍为空数组且主模型为 `gpt-image-2`；不输出 API Key。

### 任务 2：事务内补齐模型配置

**文件/资源：**
- 修改：PostgreSQL `channel_monitors` 的 8、9 号 `extra_models`
- 不修改：`frontend/src/components/user/MonitorDetailDialog.vue`、后端监控聚合接口、10 号图像监控

- [ ] **步骤 1：执行事务更新并在事务内回读**

```powershell
docker exec -i sub2api-official-18082-postgres psql -v ON_ERROR_STOP=1 -U sub2api -d sub2api @'
BEGIN;

UPDATE channel_monitors
SET extra_models = '["gpt-5.5","codex-auto-review","gpt-5.6-luna","gpt-5.6-terra","gpt-image-1.5","gpt-image-2"]'::jsonb,
    updated_at = NOW()
WHERE id = 8 AND enabled = TRUE;

UPDATE channel_monitors
SET extra_models = '["codex-auto-review","gpt-5.6-luna","gpt-5.6-sol","gpt-5.6-terra"]'::jsonb,
    updated_at = NOW()
WHERE id = 9 AND enabled = TRUE;

SELECT id, primary_model, extra_models
FROM channel_monitors
WHERE id IN (8,9,10)
ORDER BY id;

COMMIT;
'@
```

预期：8 号总模型数为 7，9 号总模型数为 5，10 号总模型数为 1；SQL 返回正常后才提交事务。

### 任务 3：验证运行状态与用户详情

**文件/资源：**
- 读取：`GET /health`
- 读取：认证用户 `GET /api/v1/channel-monitors/{id}/status`
- 记录：本次变更结果追加到新的 `docs/ai/context/` 执行记录，不覆盖历史文档

- [ ] **步骤 1：确认服务健康且未因配置更新重启**

```powershell
Invoke-WebRequest -UseBasicParsing -Uri 'http://127.0.0.1:18082/health' | Select-Object StatusCode,Content
docker ps --format '{{.Names}}\t{{.Status}}' | Select-String 'sub2api-official-18082($|-)'
```

预期：健康检查返回 HTTP 200，应用、PostgreSQL、Redis 容器均保持运行。

- [ ] **步骤 2：验证详情模型行数**

使用现有认证会话请求 8、9、10 号详情接口，确认响应 `models` 长度依次为 7、5、1，且模型名与任务 2 的 SQL 快照一致。

- [ ] **步骤 3：等待一次调度周期后核验新增历史**

查询 `channel_monitor_histories`，确认 8、9 号新增模型有检测记录；10 号不新增文本模型。若图像模型因当前文本探测协议返回失败，保留该真实状态，不将其误标为可用。

### 任务 4：完成记录

- [ ] **步骤 1：写入执行结果文档**

记录变更前后模型集合、接口返回模型行数、健康检查与历史记录查询结果；不得写入 API Key、密码或完整凭证。
