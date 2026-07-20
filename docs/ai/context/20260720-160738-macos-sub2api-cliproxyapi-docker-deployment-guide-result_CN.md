# macOS Docker 部署指南补充结果

## 结果

- 新增 `docs/sub2api-docker-deployment-macos_CN.md`，用于 macOS 服务器上通过 Docker bridge 联动部署 Sub2API 与 CLIProxyAPI-private。
- 在 `README_CN.md` 的部署章节增加入口。
- 在 `deploy/README.md` 顶部增加入口。

## 边界

- 本次只新增/更新文档，没有修改 Docker Compose 行为、应用代码、数据库结构或运行态。
- 指南使用占位符描述 `.env`、API Key、usage event token 和 HMAC secret，没有写入真实敏感信息。
- 指南明确 PostgreSQL/Redis 保持在 Sub2API 数据网络，CLIProxyAPI-private 只通过共享 bridge 与 Sub2API 通信。

