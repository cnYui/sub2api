# 2026-06-28 main-preview 18080 蓝绿测试启动计划

## 目标

使用本机当前 `main` 工作树的 Sub2API 最新代码重新构建前后端镜像，并启动隔离的 `sub2api-main-preview` 栈：

- 应用：`sub2api-main-preview`
- PostgreSQL：`sub2api-main-preview-postgres`
- Redis：`sub2api-main-preview-redis`
- 本机入口：`http://127.0.0.1:18080`

## 当前事实

- 当前工作树：`/Users/wujianxiang/CodeSpace/sub2api`
- 当前分支：`main`
- 当前 HEAD：`eb95da02c fix: 统一订阅缺失时的计费兜底路径`
- `main` 与 `origin/main` 处于分叉状态：本地 ahead 60、behind 23。本次按用户“本地最新主分支代码”理解为使用本机当前 `main` HEAD，不执行拉取、合并或重置。
- 公网运行栈：
  - `sub2api-candidate`
  - `sub2api-candidate-postgres`
  - `sub2api-candidate-redis`
  - 端口：`127.0.0.1:18084->8080`
- `sub2api-main-preview*` 当前均为停止状态。
- 旧 `sub2api-main-preview` 应用容器映射端口是 `18082`；本次要改为 `18080`，需要删除并重建停止中的应用容器，但保留数据卷。

## 约束

- 禁止停止、删除、重建或重命名 `sub2api-candidate*`。
- 禁止修改 Nginx、Cloudflare Tunnel 或公网 `8080 -> 18084` 链路。
- 禁止使用会按 Compose project 批量 `down --remove-orphans` 的流程，避免重演候选预演误伤公网事故。
- 不打印、记录或提交 API Key、SMTP 密码、JWT secret、TOTP key、内部 token 或 env 文件内容。
- 不修改公网数据库、Redis 或 18084 运行态 settings。

## 执行步骤

1. 构建镜像 `sub2api-main-preview:20260628-173235-eb95da02c`，并打辅助 tag `sub2api-main-preview:codex-main`。
2. 启动既有 `sub2api-main-preview-postgres` 和 `sub2api-main-preview-redis`。
3. 删除停止中的旧 `sub2api-main-preview` 应用容器，仅为了把端口从 `18082` 改为 `18080`。
4. 使用新镜像创建 `sub2api-main-preview`，连接既有 `sub2api-main-preview-net`，端口绑定为 `127.0.0.1:18080:8080`，数据卷仍为 `sub2api-main-preview-data:/app/data`。
5. 等待 `http://127.0.0.1:18080/health` 返回 200。
6. 验证：
   - `sub2api-main-preview*` 三个容器运行状态。
   - `http://127.0.0.1:18080/health`。
   - `http://127.0.0.1:18080` 前端 HTML 资源指纹。
   - `http://127.0.0.1:18080/api/v1/settings/public` 的非敏感开关。
   - `http://127.0.0.1:18084/health` 仍正常，且 `sub2api-candidate*` 未被重建或停止。
7. 写结果文档并更新 `AGENTS.md` 长期记忆。

## 回滚

- 如果 18080 应用启动失败，只停止并删除 `sub2api-main-preview` 应用容器；保留 `sub2api-main-preview-postgres`、`sub2api-main-preview-redis` 和数据卷，方便继续排查。
- 不对 18084 执行任何回滚动作，因为本计划不触碰公网运行栈。
