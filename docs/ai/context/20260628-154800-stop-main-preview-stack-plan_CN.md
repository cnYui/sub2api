# 2026-06-28 关闭第三套 main-preview 栈计划

## 背景

当前本机除公网候选 18084 和 SMTP 测试 18085 外，还残留第三套 `sub2api-main-preview` 栈：

- `sub2api-main-preview`
- `sub2api-main-preview-postgres`
- `sub2api-main-preview-redis`

用户要求关闭第三套。

## 执行边界

- 只停止 `sub2api-main-preview*` 相关容器。
- 不删除容器。
- 不删除 Docker volume。
- 不修改 nginx。
- 不影响 18084 公网候选栈：`sub2api-candidate*`。
- 不影响 18085 测试栈：`sub2api-smtp-test*`。

## 验证

- `sub2api-main-preview*` 应处于 exited/stopped。
- `http://127.0.0.1:18084/health` 正常。
- `http://127.0.0.1:18085/health` 正常。
- `http://127.0.0.1:8080/health` 正常。
