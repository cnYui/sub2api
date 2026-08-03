# /purchase 页面 18082 重启结果

时间：2026-08-03 16:22:00（Asia/Tokyo）

## 已部署版本

- 分支：`main`
- 应用提交：`2d8a47bd2 feat: 迁移购买页面为商品商店`
- 应用镜像：`sub2api-official-18082-sub2api`

## 执行范围

使用 `deploy/docker-compose.dev.yml` 与 `deploy/docker-compose.18082.yml` 强制重建 `sub2api` 容器。PostgreSQL 与 Redis 保持运行，未重建数据服务，也未改动其数据卷。

部署命令从旧应用容器的已有运行环境中读取数据库和代理配置，仅映射给本次 Compose 进程；未将配置写入仓库或本记录。

## 验证结果

- `sub2api-official-18082`：`running`、`healthy`
- `GET http://127.0.0.1:18082/health`：HTTP 200，响应 `{"status":"ok"}`
- `GET http://127.0.0.1:18082/purchase`：HTTP 200，页面入口已返回
