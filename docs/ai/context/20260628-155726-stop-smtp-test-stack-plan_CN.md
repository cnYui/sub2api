# 2026-06-28 关闭 18085 SMTP 测试栈计划

## 背景

用户要求关闭本地 18085 SMTP 测试栈：

- `sub2api-smtp-test`
- `sub2api-smtp-test-postgres`
- `sub2api-smtp-test-redis`

该栈此前用于验证 SMTP 配置，端口为 `127.0.0.1:18085->8080`。

## 执行边界

- 只停止 `sub2api-smtp-test*` 相关容器。
- 不删除容器。
- 不删除 Docker volume。
- 不修改 nginx。
- 不影响 18084 公网候选栈：`sub2api-candidate*`。

## 验证

- `sub2api-smtp-test*` 应处于 exited/stopped。
- `http://127.0.0.1:18084/health` 正常。
- `http://127.0.0.1:8080/health` 正常。
- `https://api.aaccx.pw/health` 正常。
