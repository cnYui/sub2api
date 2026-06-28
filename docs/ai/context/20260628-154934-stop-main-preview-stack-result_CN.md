# 2026-06-28 关闭第三套 main-preview 栈结果

## 执行内容

按用户要求关闭第三套本地残留栈，执行停止：

- `sub2api-main-preview`
- `sub2api-main-preview-postgres`
- `sub2api-main-preview-redis`

本次只执行 `docker stop`，没有删除容器，没有删除 volume，没有修改 nginx，也没有影响 18084 和 18085。

## 停止后状态

`sub2api-main-preview*` 当前均为：

```text
Exited (0)
```

当前仍在运行的 Sub2API 栈：

- `sub2api-candidate`：`127.0.0.1:18084->8080`
- `sub2api-candidate-postgres`
- `sub2api-candidate-redis`
- `sub2api-smtp-test`：`127.0.0.1:18085->8080`
- `sub2api-smtp-test-postgres`
- `sub2api-smtp-test-redis`

## 验证

以下健康检查均返回 `{"status":"ok"}`：

- `http://127.0.0.1:18084/health`
- `http://127.0.0.1:18085/health`
- `http://127.0.0.1:8080/health`

## 结论

第三套 `main-preview` 应用和数据层已关闭；公网候选 18084、SMTP 测试 18085 和 nginx 8080 均保持正常。
