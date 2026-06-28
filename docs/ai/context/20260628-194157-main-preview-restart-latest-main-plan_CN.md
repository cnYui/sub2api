# 2026-06-28 main-preview 18080 重启到最新 main 计划

## 目标

把本地 18080 main-preview 环境的 Sub2API 前后端更新到当前本机 `main` 最新代码，并保持 preview 数据层不变。

## 当前事实

- 工作树：`/Users/wujianxiang/CodeSpace/sub2api`
- 分支：`main`
- 当前 HEAD：`befa8f138 docs: 记录邀请返利合并结果`
- 当前 18080 应用镜像：`sub2api-main-preview:20260628-173235-eb95da02c`
- 当前 18080 数据层：
  - `sub2api-main-preview-postgres`
  - `sub2api-main-preview-redis`
- 当前公网候选：
  - `sub2api-candidate`
  - `sub2api-candidate-postgres`
  - `sub2api-candidate-redis`
  - 端口：`127.0.0.1:18084->8080`

## 约束

- 只替换 `sub2api-main-preview` 应用容器。
- 不停止、不删除、不重建 `sub2api-main-preview-postgres` 和 `sub2api-main-preview-redis`。
- 不停止、不删除、不重建、不写入 `sub2api-candidate*`。
- 不修改 nginx、Cloudflare Tunnel 或公网 `8080 -> 18084` 链路。
- 不打印、记录或提交 API Key、SMTP 密码、JWT secret、TOTP key、内部 token 或 dump 内容。

## 执行步骤

1. 备份当前 18080 preview DB 到 `deploy/backups/20260628-194157-18080-preview-before-main-restart.dump`，该文件包含敏感数据，权限设为 `600`，不要提交。
2. 构建镜像：
   - `sub2api-main-preview:20260628-194157-befa8f138`
   - `sub2api-main-preview:codex-main`
3. 从旧 `sub2api-main-preview` 提取环境变量到临时文件，不输出内容。
4. 停止并删除旧 `sub2api-main-preview` 应用容器。
5. 用新镜像重建 `sub2api-main-preview`：
   - 网络：`sub2api-main-preview-net`
   - 端口：`127.0.0.1:18080:8080`
   - 数据卷：`sub2api-main-preview-data:/app/data`
6. 等待 `http://127.0.0.1:18080/health` 返回 200，允许应用自动执行新迁移。
7. 验证：
   - 18080 health。
   - 18080 前端 HTML 资源指纹。
   - 18080 users/api_keys/schema_migrations 数量。
   - 18084 health 仍正常。
   - `sub2api-candidate` 镜像和启动时间未改变。

## 回滚

如果新 18080 应用启动失败：

1. 删除失败的 `sub2api-main-preview` 应用容器。
2. 用旧镜像 `sub2api-main-preview:20260628-173235-eb95da02c` 按原端口和网络重建应用容器。
3. 如新迁移导致 DB 不兼容，再用备份 dump 恢复 preview DB；该恢复只作用于 `sub2api-main-preview-postgres`。
