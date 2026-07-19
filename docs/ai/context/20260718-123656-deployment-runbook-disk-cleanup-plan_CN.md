# 部署 Runbook 增加磁盘空间清理计划

## 背景

用户补充：重新部署前电脑空间不够，需要先把旧容器清理任务写进部署文档，再进行重新部署。

## 目标

- 在 `docs/SUB2API_CLIPROXYAPI_DEPLOYMENT_RUNBOOK_CN.md` 增加“旧容器/旧镜像清理释放空间”章节。
- 更新新人检查清单，明确清理步骤必须在重新部署前完成。
- 更新 `AGENTS.md` 长期记忆，避免后续部署时跳过空间清理。
- 新增 result 上下文记录本轮只改文档，未清理本机 Docker、未部署。

## 设计边界

- 只写文档，不执行 Docker 清理命令。
- 清理流程必须先识别当前生产容器，保护：
  - `sub2api-candidate`
  - `sub2api-candidate-postgres`
  - `sub2api-candidate-redis`
- 允许新人清理的优先级：
  1. 已停止的旧应用容器、preview/test 容器、历史 `before-promote` 容器。
  2. dangling images。
  3. Docker build cache。
  4. 仍未释放足够空间时，按清单删除未被任何容器引用的旧 `sub2api-candidate:*` 镜像。
- 明确禁止：
  - `docker volume prune`
  - `docker compose down -v`
  - 删除 `deploy/candidate/postgres_data`、`deploy/candidate/redis_data`、`deploy/backups`
  - 删除当前生产容器或当前镜像

## 待改文件

- `docs/SUB2API_CLIPROXYAPI_DEPLOYMENT_RUNBOOK_CN.md`
- `AGENTS.md`
- `docs/ai/context/20260718-123907-deployment-runbook-disk-cleanup-result_CN.md`

## 验证

- `git diff --check`
- 敏感信息粗扫：确认新增内容没有真实密钥。
