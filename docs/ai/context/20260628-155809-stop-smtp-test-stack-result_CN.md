# 2026-06-28 关闭 18085 SMTP 测试栈结果

## 执行内容

按用户要求关闭本地 18085 SMTP 测试栈，执行停止：

- `sub2api-smtp-test`
- `sub2api-smtp-test-postgres`
- `sub2api-smtp-test-redis`

本次只执行 `docker stop`，没有删除容器，没有删除 volume，没有修改 nginx，也没有影响 18084 公网候选栈。

## 停止后状态

`sub2api-smtp-test*` 当前均为：

```text
Exited (0)
```

当前仍在运行的 Sub2API 公网候选栈：

- `sub2api-candidate`：`127.0.0.1:18084->8080`
- `sub2api-candidate-postgres`
- `sub2api-candidate-redis`

## 验证

以下健康检查均返回 `{"status":"ok"}`：

- `http://127.0.0.1:18084/health`
- `http://127.0.0.1:8080/health`
- `https://api.aaccx.pw/health`

## 结论

18085 SMTP 测试应用和数据层已关闭；18084 公网候选链路保持正常。
