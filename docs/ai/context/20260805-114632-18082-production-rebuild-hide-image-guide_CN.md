# 18082 生产重建与生图主题下架发布

## 发布目标

将 `main` 分支最新前端内容发布到 18082 生产应用，使使用方法页面隐藏“生图方法”主题。

## 执行

- 使用 `deploy/docker-compose.dev.yml` 与 `deploy/docker-compose.18082.yml` 合并配置执行：
  `docker compose -f docker-compose.dev.yml -f docker-compose.18082.yml up -d --build --force-recreate --no-deps sub2api`
- 仅重建 `sub2api-official-18082` 应用容器，未重建 PostgreSQL、Redis 或数据卷。
- 旧应用镜像：`sha256:ea441e1f043c8b8d518715918e6123656386912f05efd17c4eb0ea5ea282a7fc`。
- 新应用镜像：`sha256:d2715893266e6ac251f57ee27b8f3193c5176f2347ac39484ac874ca3f3791a6`。

## 验证

- `sub2api-official-18082`：`running`、`healthy`，端口 `127.0.0.1:18082 -> 8080`。
- PostgreSQL、Redis：容器持续运行，启动时间未因本次发布改变。
- `http://127.0.0.1:18082/health`：HTTP 200，响应 `{"status":"ok"}`。
- `http://127.0.0.1:8080/health`：HTTP 200，响应 `{"status":"ok"}`。
- `https://aaccx.pw/health`、`https://www.aaccx.pw/health`、`https://api.aaccx.pw/health`：均 HTTP 200，响应 `{"status":"ok"}`。

## 风险说明

- 构建输出仅有项目既有的 Browserslist、动态导入和大 chunk 警告，没有构建错误。
- 本次只调整使用方法主题可见性，不改变生图 API、计费或权限逻辑。
