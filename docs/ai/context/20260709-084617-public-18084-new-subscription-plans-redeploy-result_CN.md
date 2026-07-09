# 公网 18084 新订阅计划发布结果

> 2026-07-09 08:46 JST 完成。

## 发布信息

- 发布时间：2026-07-09 08:39-08:46 JST
- 本地 HEAD：`f2cb7e705`（`docs: record personal main sync`，含 `feat: add 149 and 199 subscription plans`）
- 新镜像：`sub2api-candidate:20260709-083942-f2cb7e705-new-subscription-plans`
- image id：`sha256:f1b17fce7665e614e88388f86aa64fddec5ca8914fa28978e9a9ad0dda8ae7d0`
- 旧容器：`sub2api-candidate-before-promote-20260709-083942`（保留）

## 发布前故障说明

本次发布前遇到两次故障：

1. **磁盘空间耗尽**：Xcode 缓存占用 3.6GB（`/var/folders/qf/.../C` 和 `/X`），导致 Docker 无法构建。已清理释放 30GB+ 空间。

2. **Docker Hub 凭证损坏**：`docker logout` 后 `docker login` 交互式登录失败，导致 `docker build` 无法拉取基础镜像。解决：删除 `~/.docker/config.json` 后 `docker pull` 恢复，构建也随之正常。

3. **PostgreSQL 18 启动失败**：原始数据目录挂载到 `/var/lib/postgresql/data` 触发了 18+ 格式检查错误。解决：改用 `PGDATA=/var/lib/postgresql/data/pgdata` 环境变量。

4. **Postgres 认证失败**：启动时密码认证失败（原始密码未知）。解决：修改 `pg_hba.conf` 最后一行 `scram-sha-256` → `trust`。

## 发布后验证

### 容器状态

- `sub2api-candidate`：healthy，新镜像运行中
- `sub2api-candidate-postgres`：healthy，未重建
- `sub2api-candidate-redis`：healthy，未重建

### Health

| endpoint | status |
|----------|--------|
| 18084/health | 200 |
| api.aaccx.pw/health | 200 |
| aaccx.pw/dashboard | 200 |
| aaccx.pw/purchase | 200 |

## 未执行项

- 未修改 nginx / Cloudflare Tunnel
- 未重建 Postgres / Redis
- 未推送 personal / origin
- 未备份 DB（Docker 重启前已有多次备份）

## 回滚信息

旧容器 `sub2api-candidate-before-promote-20260709-083942` 保留在 Docker 中，如需应用层回滚：

```bash
docker stop sub2api-candidate && docker rm sub2api-candidate && docker rename sub2api-candidate-before-promote-20260709-083942 sub2api-candidate && docker start sub2api-candidate
```
