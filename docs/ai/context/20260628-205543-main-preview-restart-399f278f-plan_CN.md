# 18080 main-preview 重启计划

## 目标

- 使用本地 `main` 当前 HEAD `399f278f3` 的代码重新构建并启动 `127.0.0.1:18080` 的 `sub2api-main-preview` 应用容器。
- 保留现有 `sub2api-main-preview-postgres` 和 `sub2api-main-preview-redis`，让新应用容器自行执行迁移。
- 严格禁止触碰公网候选链路：`sub2api-candidate`、`sub2api-candidate-postgres`、`sub2api-candidate-redis`、nginx `8080 -> 18084`。

## 当前状态

- 工作目录：`/Users/wujianxiang/CodeSpace/sub2api`
- 分支：`main`
- 当前 HEAD：`399f278f3 merge: 合并注册重复邮箱预检拦截`
- Git 状态：存在用户上下文相关未提交改动，不能丢弃或回滚。
- 当前 18080 应用容器：`sub2api-main-preview`，镜像 `sub2api-main-preview:20260628-195911-befa8f138-migrationfix`，端口 `127.0.0.1:18080->8080`。
- 当前公网候选容器：`sub2api-candidate`，镜像 `sub2api-candidate:20260627-221441-traffic-card-fix`，端口 `127.0.0.1:18084->8080`。

## 执行方案

1. 备份 18080 preview 数据库到 `deploy/backups/20260628-205543-18080-preview-before-399f278f-restart.dump`，文件权限设为 `600`。
2. 运行与本次新代码和迁移风险相关的测试：
   - 迁移 checksum/156/157 相关测试。
   - 注册、邮箱验证码、重复邮箱预检相关后端 service 测试。
   - 前端注册页相关测试。
3. 构建新镜像：
   - `sub2api-main-preview:20260628-205543-399f278f3`
   - `sub2api-main-preview:codex-main`
4. 从旧 `sub2api-main-preview` 容器提取环境变量到临时 env 文件，不打印敏感内容。
5. 停止并删除旧 `sub2api-main-preview` 应用容器，只重建应用容器：
   - 网络：`sub2api-main-preview-net`
   - 端口：`127.0.0.1:18080:8080`
   - 数据卷：`sub2api-main-preview-data:/app/data`
   - 环境变量：沿用旧应用容器提取结果
6. 等待 `http://127.0.0.1:18080/health` 返回成功。
7. 验证：
   - `sub2api-main-preview` 运行新镜像并 healthy。
   - `18080/health` 返回 200。
   - `18084/health`、`8080/health` 仍返回 200，且公网候选容器镜像和启动时间未变化。
   - 18080 数据库 `schema_migrations` 可查询，确认新迁移状态。

## 风险控制

- 不执行 `deploy/redeploy-sub2api-image.sh`，因为该脚本面向标准 `sub2api` 公网本地容器，不适合本次 preview-only 替换。
- 不清空、不覆盖 preview 数据库，除非用户另行确认。
- 不输出 API Key、SMTP 密码、JWT secret、TOTP key、内部 token 或 dump 内容。
- 如果测试失败或迁移失败，停止替换并保留当前 18080 容器。
