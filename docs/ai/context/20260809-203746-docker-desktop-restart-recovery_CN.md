# Docker Desktop 重启后的公网恢复核验

## 重启与恢复

- 按管理员要求执行 `docker desktop restart --timeout 300`，Docker Desktop 引擎完成重启。
- 引擎重启期间本机容器短暂中断；`sub2api-official-18082` 因 PostgreSQL 尚在启动阶段自动重试，数据库恢复 healthy 后应用恢复为 `running/healthy`。
- 应用仍运行此前构建的镜像 `sha256:607eccf01f8d88483b540ea3eb014ebaa7d36ec4e3164c09782a62d336e5a0c0`，未重建数据卷或修改配置。

## 最终核验

| 目标 | 结果 |
| --- | --- |
| `sub2api-official-18082` | `running/healthy` |
| `sub2api-official-18082-postgres` | `running/healthy` |
| `sub2api-official-18082-redis` | `running/healthy` |
| `http://127.0.0.1:18082/health` | HTTP 200 |
| `http://127.0.0.1:8080/health` | HTTP 200 |
| `https://aaccx.pw/health` | HTTP 200 |
| `https://www.aaccx.pw/health` | HTTP 200 |
| `https://api.aaccx.pw/health` | HTTP 200 |
| `sub2api-public-nginx-local nginx -t` | 通过 |

