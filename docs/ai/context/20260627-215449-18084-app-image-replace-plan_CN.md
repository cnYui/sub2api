# 18084 应用容器镜像替换执行计划

时间：2026-06-27 21:54 JST

## 目标

将当前 18085 已验证的新前后端代码替换到公网链路中的 `sub2api-candidate` 应用容器，保持公网拓扑不变：

```text
Cloudflare Tunnel -> nginx 127.0.0.1:8080 -> sub2api-candidate 127.0.0.1:18084 -> CLIProxyAPI 127.0.0.1:8317
```

## 不变项

- 不修改 nginx 配置。
- 不把 nginx 指向 18085。
- 不停止、不删除、不重建 `sub2api-candidate-postgres`。
- 不停止、不删除、不重建 `sub2api-candidate-redis`。
- 不执行 `docker compose down`。
- 不打印或记录 env 中的密码、JWT secret、TOTP key、内部 token。

## 执行步骤

1. 记录当前 `main` HEAD 和容器状态。
2. 对 `sub2api-candidate-postgres` 做一次逻辑备份，保存到 `deploy/backups/`。
3. 检查 18084 候选库当前 migration 版本数。
4. 将已验证镜像 `sub2api-smtp-test:20260627-214036` 打 tag 为 `sub2api-candidate:20260627-214036-e4704061d`。
5. 从旧 `sub2api-candidate` 提取 env 到临时文件，不输出内容。
6. 停止旧 `sub2api-candidate`，重命名为备份容器。
7. 用新镜像启动新的 `sub2api-candidate`：
   - `--network sub2api-candidate-network`
   - `-p 127.0.0.1:18084:8080`
   - `--env-file` 使用旧容器提取出的 env
8. 等新容器 healthy。
9. 如果新容器 unhealthy 或超时，删除新容器并恢复旧容器。
10. 验证：
    - `18084/health`
    - `8080/health`
    - `Host: api.aaccx.pw` 下的 `8080/health`
    - 18084 公开设置中的非敏感开关
    - 18084 前端资源 hash
    - `18085/health` 确认测试栈仍在
11. 写入结果文档并更新 `AGENTS.md`。

## 回滚方式

如果新容器未进入 healthy：

1. 删除新 `sub2api-candidate`。
2. 将备份容器重命名回 `sub2api-candidate`。
3. 启动备份容器。
4. 验证 `18084/health` 和 `8080/health`。

数据库和 Redis 全程保持运行。
