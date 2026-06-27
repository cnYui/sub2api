# Sub2API 公网会话恢复与蓝绿切换判断

## 当前判断

- 公网 Sub2API 主进程、Postgres、Redis 当前健康。
- `http://127.0.0.1:18080/health` 与 `https://api.aaccx.pw/health` 当前均返回 200。
- 公网 Postgres 未见用户表/API key 表批量损坏：
  - `users_total=32`
  - `users_active=31`
  - `users_with_password=31`
  - `api_keys_active=21`
  - 事故窗口后 `settings` 无更新
- 管理员账号密码登录本地公网后端 smoke test 返回 200，说明账号密码登录能力未整体损坏。
- 近期 `/v1/responses` 仍有大量 200，说明 API key 鉴权也不是整体失效。

## 已确认公网问题

第一次候选预演误重建公网 Redis，导致旧 refresh token / 登录会话态丢失。表现为：

- 用户原有页面请求 `/api/v1/auth/me` 返回 401。
- 前端尝试 `/api/v1/auth/refresh` 返回 401。
- 后端日志出现 `Refresh token not found, possible reuse attack`。
- 用户被迫回到登录页。

这是会话态丢失，不是 Postgres 主数据整体损坏。

## 当前修复顺序

1. 不重启公网容器，不再跑真实候选预演。
2. 对用户侧先按会话失效处理：刷新页面后重新登录；若前端还卡在旧 token 状态，清理站点本地存储后重新登录。
3. 如果某个用户重新输入正确密码仍 401，按该用户邮箱做只读排查：
   - `users.status`
   - `users.deleted_at`
   - `users.password_hash` 是否为空
   - 该用户是否原本只通过 OAuth/第三方登录
4. 对确认为密码不可用的具体用户，再单独执行密码重置或恢复登录方式；不要全局改用户表。
5. 只在确认用户登录恢复后，再继续候选预演脚本修复。

## 不建议的修复

- 不建议恢复旧 Redis 快照：refresh token 是短期会话态，当前可用快照不一定完整且可能造成 token family/reuse 状态混乱。
- 不建议直接切换到新镜像：当前新镜像仍涉及 migration 155 checksum/运行态 DB 差异，未完成候选预演前直接上公网风险高。
- 不建议只切前端：登录会话丢失发生在 Redis/后端 refresh token 层，前端新代码不能恢复旧 refresh token。

## 关于“本地成功跑起来后是否一键切换前端即可修复”

不能只切前端。

如果本地候选环境使用生产 DB 克隆完整跑通，只能证明候选镜像与克隆 DB 可启动、迁移、健康检查和关键 API 可用。公网修复仍需要切换公网 Sub2API 后端镜像，必要时也包含前端静态资源，因为当前 Docker 镜像同时打包后端与前端。

正确蓝绿路径应是：

1. 本地候选：新镜像 + 生产 DB 克隆 + 独立 Redis + 独立端口跑通。
2. 候选验证：build、migration、启动、health、登录 smoke、关键页面、只读 API、必要的候选 `/v1` smoke。
3. 准备公网切换：确认公网 DB 不会触发 checksum mismatch；确认需要新增的运行态数据已通过安全迁移或脚本准备好。
4. 短窗口切换：retag 已验证候选镜像为公网镜像，重建 `sub2api` 应用容器；不重建公网 Postgres/Redis。
5. 切后验证：本地 health、公网 health、管理员登录、关键只读页面、少量受控 API key smoke。

## 防复发要求

- 候选 compose 必须固定 `name: sub2api-candidate-rehearsal`，脚本必须显式 `-p sub2api-candidate-rehearsal`。
- 候选脚本不得使用会按 project 清理公网容器的 `down --remove-orphans`。
- 候选脚本启动前必须检查公网容器 label，发现公网容器和候选 project 相同立即退出。
- 候选数据目录不得与公网 `deploy/postgres_data`、`deploy/redis_data`、`deploy/data` 共用。
