# Sub2API 与 CLIProxyAPI 本地共享网络实施计划

## 目标

- 仅调整 Windows/Docker Desktop 本地开发链路，不修改公网、候选环境、Nginx 或 Cloudflare Tunnel。
- 保留 PostgreSQL、Redis 现有 `sub2api-network`，让 Sub2API 额外加入 `sub2api-cliproxy-local` 外部 bridge。
- CLIProxyAPI 通过服务名 `cliproxyapi` 提供 TLS 上游，usage event 通过服务名 `sub2api` 回调。
- 保留 `127.0.0.1:8317` 作为本机调试和回滚入口。

## 修改边界

- `CLIProxyAPI-private` 新增 Sub2API 本地集成专用 Compose 和证书生成脚本。
- 旧自签名证书停止跟踪；新 CA 私钥和叶子私钥只存放在 Git 忽略目录，不进入 Docker 构建上下文。
- Sub2API 增加可选 CA 注入能力和本地集成 Compose 覆盖文件；未设置环境变量时现有生产镜像行为不变。
- PostgreSQL、Redis 不加入共享网络，不修改数据结构，不清空缓存。
- 上游 `base_url` 通过管理 API 修改，利用现有 repository 路径同步刷新 Redis 调度快照。

## 实施顺序

1. 备份 PostgreSQL、Redis、CLIProxyAPI 本地配置和旧证书，并验证备份可读。
2. 保存当前 Sub2API 镜像回滚标签和 CLIProxyAPI 容器状态。
3. 完成两仓库文件修改和静态验证。
4. 创建共享网络、生成新 CA/叶子证书，先重建 Sub2API 并保持旧上游 URL。
5. 替换 CLIProxyAPI 容器，验证旧宿主机路径与新服务名路径。
6. 通过管理 API 更新 `cliproxy-local-openai` 上游 URL，执行业务和 usage event 验证。
7. 重建验证、敏感信息检查并记录结果。

## 回滚

- 先通过管理 API 恢复 `https://host.docker.internal:8317/v1`。
- 恢复 CLIProxyAPI 备份的 `.env`、`config.yaml` 和旧证书，再启动保留的旧容器。
- 如 Sub2API 新入口脚本异常，使用预留镜像标签恢复原镜像并按原 Compose 文件重建应用容器。
- PostgreSQL、Redis 备份只在确认数据损坏时人工恢复，不纳入普通回滚。
