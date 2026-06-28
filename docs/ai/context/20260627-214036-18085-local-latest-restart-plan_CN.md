# 18085 本地蓝绿测试实例重启计划

## 目标

把当前本地 `main` 的 Sub2API 最新前后端版本重新构建并运行到 `127.0.0.1:18085`，用于蓝绿测试，不影响当前公网链路 `8080 -> 18084 -> sub2api-candidate`。

## 当前事实

- 当前 `main` HEAD：`e4704061d fix: 统一认证中间件计费准入`。
- 18085 当前运行的是隔离栈：
  - 应用容器：`sub2api-smtp-test`
  - DB：`sub2api-smtp-test-postgres`
  - Redis：`sub2api-smtp-test-redis`
  - 端口：`127.0.0.1:18085 -> container:8080`
- 公网当前运行态：
  - `sub2api-candidate`
  - `sub2api-candidate-postgres`
  - `sub2api-candidate-redis`
  - `127.0.0.1:18084`

## 约束

- 不停止、不重建、不删除 `sub2api-candidate*` 任何容器。
- 不修改 Nginx、Cloudflare Tunnel 或 `8080 -> 18084` 链路。
- 不替换 `weishaw/sub2api:latest`，只构建新的 `sub2api-smtp-test:*` 镜像。
- 不重建 18085 的 Postgres/Redis，保留 18085 测试库里的 SMTP 运行态配置。
- 不输出或记录 SMTP 密码、API Key、内部 token、HMAC secret。

## 操作步骤

1. 检查 Docker 当前容器，确认 18084 和 18085 是不同容器栈。
2. 读取 `sub2api-smtp-test` 的非敏感启动信息：网络、端口映射、容器名、镜像名。
3. 用当前 main 工作树构建新镜像：`sub2api-smtp-test:20260627-214036`。
4. 停止并删除 `sub2api-smtp-test` 应用容器；不动 DB/Redis。
5. 用原 18085 配置重新创建应用容器，连接 `sub2api-smtp-test-net`、`sub2api-smtp-test-postgres`、`sub2api-smtp-test-redis`。
6. 等待 `http://127.0.0.1:18085/health` 返回 200。
7. 验证：
   - 18085 health。
   - 18085 前端 HTML 资源指纹。
   - 18085 `/api/v1/settings/public` 中注册、邮箱验证、密码重置状态。
   - 18084 health 仍正常，公网候选容器未重建。
8. 写结果文档并更新必要长期记忆。
