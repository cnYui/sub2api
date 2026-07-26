# 指定容器与镜像清理结果

时间：2026-07-26 09:59:00

## 已删除

按用户指定，已删除下列独立 PKB/Supabase 服务容器和对应镜像：

- `supabase_kong_SW` / `public.ecr.aws/supabase/kong:2.8.1`
- `pkb-neo4j` / `neo4j:5.26.0`
- `supabase_rest_SW` / `public.ecr.aws/supabase/postgrest:v14.13`
- `supabase_vector_SW` / `public.ecr.aws/supabase/vector:0.53.0-alpine`
- `pkb-frontend-dev` / `node:20-alpine`
- `supabase_pg_meta_SW` / `public.ecr.aws/supabase/postgres-meta:v0.96.6`
- `supabase_inbucket_SW` / `public.ecr.aws/supabase/mailpit:v1.30.2`
- `supabase_realtime_SW` / `public.ecr.aws/supabase/realtime:v2.108.0`
- `supabase_studio_SW` / `public.ecr.aws/supabase/studio:2026.06.22-sha-2207d7f`
- `supabase_auth_SW` / `public.ecr.aws/supabase/gotrue:v2.191.0`
- `supabase_analytics_SW` / `public.ecr.aws/supabase/logflare:1.45.4`
- `supabase_storage_SW` / `public.ecr.aws/supabase/storage-api:v1.61.3`

`supabase_edge_runtime_SW` 与 `public.ecr.aws/supabase/edge-runtime:v1.74.1` 在执行清理前已不存在，因此无需重复删除。

## 明确保留

以下条目虽然出现在用户初始清单中，但实际是当前健康的 Sub2API 链路依赖；按“公网运行中的镜像、数据库等禁止删除”保留：

- `sub2api-public-nginx-local` / `nginx:1.27-alpine`
- `sub2api-redis-dev` 与 `sub2api-upstream-redis` / `redis:8-alpine`

未删除 PostgreSQL、Redis 挂载数据、Docker 卷、构建缓存或其余运行容器。

## 最终核验

- 剩余 9 个镜像均被现有运行容器引用。
- `sub2api-dev:18080` 与 `sub2api-upstream-latest:18086` 均为 `healthy`。
- 两个 `/health` 接口均返回 `{"status":"ok"}`。
