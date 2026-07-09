# Docker 匿名小 Volume 清理结果

时间：2026-07-09 09:39 JST

## 操作范围

已删除 4 个未被任何容器引用的匿名小 volume：

- `1d4dce6ca64cb7a635c702baa2cd9a2080dd879bfbb56db7f9ef621e6fefea1f`
- `875d1fb39ffb0c2e94414ba4445f60f53972afa45a459bf839ca570a141891c3`
- `29792943d15eb51af1a5f1b1e69f1d52e16c9c464898db67ac6b948ad6b8b604`
- `ee97006e00370dc7477de36cc2cf6ffe6c53dae2beaf754fee04d1c5b9f12b54`

未删除当前 Postgres 容器引用的匿名 volume：

- `97d3544bd4fb3e047dfc96fd3e104b9a3def77435a546b220c8ef02ec23eecb7`

未删除任何 named data volume：

- `deploy_postgres_data`
- `deploy_redis_data`
- `deploy_sub2api_data`
- `sub2api-main-existingdb-preview-data`
- `sub2api-main-preview-data`
- `sub2api-main-preview-pgdata`
- `sub2api-main-preview-redisdata`

## 验证

- `docker ps -a` 剩余 3 个容器，均为运行态 healthy：
  - `sub2api-candidate`
  - `sub2api-candidate-redis`
  - `sub2api-candidate-postgres`
- `docker volume ls` 剩余 8 个 volume。
- `docker system df -v` 显示 Local Volumes 仍约 `831MB`，主要来自旧 named data volumes。
- `http://127.0.0.1:18084/health` 返回 `{"status":"ok"}`。

## 后续判断

剩余 `LINKS=0` 的 named data volumes 当前运行链路不使用，但包含旧 Postgres、Redis、应用配置和日志数据。若要继续清理，建议先按需备份或明确不再需要旧 18080/preview 回滚数据，再删除这些 named volumes。
