# 公网服务旧容器回滚结果

## 用户指令与范围

按用户指令回滚到上一版外层容器并优先恢复公网旧版本服务。

- 仅替换外层 `sub2api-dev` 容器镜像。
- 恢复 `sub2api-public-nginx-local` 公网 Nginx。
- 不回滚 PostgreSQL 数据库、不重建或清空 Redis、数据库、volume 或内层服务。

## 执行结果

- 原新镜像 `sha256:07fa2bbfd5dc007d1ae9371bc4b1a96fc27c931497ffa68468ba471e1c9e0b1d` 已保留为 `sub2api-localdev-sub2api:pre-rollback-20260729-102213`。
- 外层 `sub2api-dev` 已替换为回滚镜像 `sub2api-localdev-sub2api:rollback-20260729-093220`，镜像 ID 为 `sha256:043ec4470979c60aed771f6de43d7ab3087a2eb31a34f2ba61aa41aac8854bd0`。
- `sub2api-public-nginx-local` 已启动，状态为 `running`。

## 验证证据

- `127.0.0.1:18080/health`、`127.0.0.1:18086/health`、`127.0.0.1:8080/health` 均返回 200。
- `https://aaccx.pw/health` 与 `https://api.aaccx.pw/health` 均返回 200。
- 旧外层容器的无认证 `GET /v1/models` 返回 401。

## 重要边界

数据库仍保留迁移 180 对 `usage_facts` 和授权表的 schema 演进。本次仅通过健康检查和无认证边界验证了旧容器可启动与公网可达；未使用 API Key 发起真实模型请求，因此不将真实计费链路标记为已验证。修复工作树 `codex/fix-usage-fact-authorization-column` 未合并、未构建、未部署。
