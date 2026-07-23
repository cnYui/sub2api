# 双 Sub2API 本地打通实施计划

## 目标

临时禁用 CPA 作为账号分发层的依赖路径，克隆 GitHub original 最新 Sub2API 到本地，尝试让：

- 外层：当前定制版 `D:\CodeWorkSpace\sub2api`，继续负责用户 Key、订阅、流量卡、`usage_facts` 和计费。
- 内层：GitHub original 最新 Sub2API，替代 CPA 负责真实上游账号凭证、账号调度、OAuth refresh、协议适配和 failover。

## 边界

- 不触碰公网运行态。
- 不停止当前 CPA 容器 `cliproxyapi-local-dev`，只在外层本地新增/切换一个测试 upstream account。
- 不修改生产 Nginx、Cloudflare、公网 DB/Redis。
- 内层 latest 使用独立目录、独立容器名、独立端口、独立 PostgreSQL/Redis volume。
- 如果需要写外层数据库，只限本地 `sub2api-dev` 对应本地 DB，并先记录现有 upstream account 状态。

## 当前本地事实

- 外层本地定制版容器：`sub2api-dev`，端口 `127.0.0.1:18080->8080`，健康。
- 本地公开 nginx：`sub2api-public-nginx-local`，端口 `127.0.0.1:8080->8080`。
- CPA 容器：`cliproxyapi-local-dev`，端口 `127.0.0.1:8317->8317`，健康。
- 计划克隆目录：`D:\CodeWorkSpace\sub2api-upstream-latest`。

## 实施步骤

1. 克隆 `https://github.com/Wei-Shaw/sub2api.git` 到 `D:\CodeWorkSpace\sub2api-upstream-latest`。
2. 检查 latest 仓库 Docker/配置入口，选择不会冲突的端口：
   - 内层 app：优先 `127.0.0.1:18086`。
   - 内层 PostgreSQL：不发布宿主端口或使用 `15436`。
   - 内层 Redis：不发布宿主端口或使用 `16380`。
3. 启动内层 latest Sub2API，先只验证 `/health` 或等价健康端点。
4. 在内层 latest 创建内部 service/admin 用户和内部 API Key，用于外层转发；该 Key 必须不作为真实用户计费依据。
5. 在外层本地定制版新增或更新一个测试 OpenAI upstream account：
   - `base_url=http://host.docker.internal:18086/v1` 或共享 Docker 网络服务名。
   - `api_key=<内层内部 Key>`。
   - `pool_mode=true`。
6. 用测试用户 Key 请求外层 `/v1/models`、`/v1/responses` 或 `/v1/chat/completions`。
7. 验证外层 `usage_facts` 是唯一计费事实，内层只作为请求上游记录。

## 成功标准

- 外层请求能到达内层 latest。
- 内层 latest 能返回至少 `/v1/models` 的可识别响应。
- 如果内层缺真实上游账号，外层应收到内层明确错误，而不是网络/TLS/认证错误。
- 不影响现有 CPA 容器和外层本地服务。
