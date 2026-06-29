# 2026-06-29 18080 main-preview 重启到合并后 main 计划

## 目标

- 使用本地 `main` 当前 HEAD `ddd4fb9a9 fix: remove duplicate usage guide locale keys` 重新构建 `sub2api-main-preview` 应用镜像。
- 只替换 `127.0.0.1:18080` 的 `sub2api-main-preview` 应用容器。
- 保留现有 `sub2api-main-preview-postgres`、`sub2api-main-preview-redis` 和 `sub2api-main-preview-data`，让新应用容器自行执行本地 main 新迁移。
- 严格禁止触碰公网候选链路：`sub2api-candidate`、`sub2api-candidate-postgres`、`sub2api-candidate-redis`、nginx `8080 -> 18084`。

## 当前状态

- 工作目录：`/Users/wujianxiang/CodeSpace/sub2api`
- 分支：`main`
- Git 状态：合并后本地 `main` 已补一个 locale 重复 key 编译修复提交，领先 `origin/main` 87 个提交；本轮计划文档尚未提交。
- 当前 18080 应用容器：`sub2api-main-preview`
- 当前 18080 镜像：`sub2api-main-preview:20260628-205543-399f278f3`
- 当前 18080 端口：`127.0.0.1:18080->8080`
- 当前 18080 数据层：`sub2api-main-preview-postgres`、`sub2api-main-preview-redis`
- 当前 18084 公网候选容器：`sub2api-candidate`，镜像 `sub2api-candidate:20260627-221441-traffic-card-fix`，端口 `127.0.0.1:18084->8080`

## 执行方案

1. 备份 18080 preview 数据库到 `deploy/backups/20260629-092226-18080-preview-before-merged-main-restart.dump`，文件权限设为 `600`。
2. 运行与本轮合并相关的本地验证：
   - 后端迁移/仓储定向测试。
   - 前端注册页、购买页、订阅页、使用教程定向测试。
3. 构建新镜像：
   - `sub2api-main-preview:20260629-092226-ddd4fb9a9`
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
   - `18084/health`、`8080/health` 仍返回 200。
   - 18084 候选容器镜像和启动时间未变化。
   - 18080 前端资源指纹更新到合并后 main 构建产物。
   - 18080 preview DB `schema_migrations` 可查询，确认新迁移状态。

## 风险控制

- 不执行面向公网本地标准容器的 `deploy/redeploy-sub2api-image.sh`。
- 不清空、不覆盖 preview 数据库，除非恢复需要。
- 不输出 API Key、SMTP 密码、JWT secret、TOTP key、内部 token、env 文件内容或 dump 内容。
- 如果新应用启动失败，使用旧镜像 `sub2api-main-preview:20260628-205543-399f278f3` 按原网络、端口和数据卷重建应用容器；如迁移导致 preview DB 不兼容，再用本轮备份 dump 恢复 preview DB。
