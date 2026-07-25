# CPA 本地容器启动计划

## 目标

- 在本机恢复 `cliproxyapi-local-dev`，浏览器可通过 `https://127.0.0.1:8317` 访问 CPA 控制台。
- 保持端口 `8317`，仅绑定 `127.0.0.1`。
- 不影响当前公网 Sub2API 服务，不切换 Nginx，不修改公网链路，不改 Sub2API DB/Redis。

## 当前事实

- `127.0.0.1:8317` 当前未监听。
- 当前运行中的 Sub2API 本地链路为：
  - `sub2api-dev`：`127.0.0.1:18080->8080`
  - `sub2api-upstream-latest`：`127.0.0.1:18086->8080`
  - `sub2api-public-nginx-local`：`127.0.0.1:8080->8080`
- 历史 CPA 容器 `cliproxyapi-local-dev` 处于停止状态，已绑定 `127.0.0.1:8317->8317`。
- CPA 容器仅加入本地 Docker bridge `sub2api-cliproxy-local`，别名为 `cliproxyapi`。

## 执行边界

- 只允许执行 `docker start cliproxyapi-local-dev` 或等价的本地 CPA compose 启动。
- 不执行 `docker compose up` 重建 Sub2API 项目。
- 不修改 Nginx 配置。
- 不修改 Sub2API 上游账号、数据库、Redis 或公网端口。
- 不修改 CPA 账号文件和配置文件。

## 备份与验证

- 启动前保存 CPA 容器的 Docker inspect 快照到 `backups/`，用于回滚时确认原端口、网络、挂载和环境形态。
- 备份后验证 JSON 可读。

## 验证

- `docker ps` 确认 `cliproxyapi-local-dev` 运行，端口仍是 `127.0.0.1:8317->8317`。
- 本机请求 `https://127.0.0.1:8317` 可返回 HTTP 响应。
- 用浏览器打开 `https://127.0.0.1:8317`，确认页面可访问。
- 抽查当前 Sub2API 本地健康接口仍可用，确认未影响公网承接链路。

## 回滚

- 如 CPA 启动异常或占用不符合预期，执行 `docker stop cliproxyapi-local-dev`。
- 因本次不改 DB/Redis/Nginx/账号配置，回滚边界限于停止 CPA 本地容器。
