# 本机自用 Key 公网 Chat 验证结果

## 修复动作

1. 修复 CLIProxyAPI 监听地址，使 Sub2API 容器可访问 `host.docker.internal:8317`。
2. 清除 `accounts.id=1` 的 `temp_unschedulable_until` 和 `temp_unschedulable_reason`。
3. 重启 `sub2api` 容器，刷新进程内账号缓存。

## 当前状态

`cliproxy-local-openai` 当前状态：

- `status=active`
- `schedulable=true`
- `temp_unschedulable_until=null`
- `temp_unschedulable_reason=""`

`sub2api` 容器状态为 healthy。

## 公网验证

使用本机自用 API Key（掩码 `sk-LOCAL-4540...8804`）请求：

`POST https://api.aaccx.pw/v1/chat/completions`

模型：

`gpt-5.4`

返回：

- HTTP 状态码：`200`
- 响应内容：`pong`
- 请求 ID：`82581908-b110-40bc-992e-47e16d48b8cf`

结论：`Cloudflare -> nginx -> Sub2API -> CLIProxyAPI -> Codex 上游` 主链路已恢复。
