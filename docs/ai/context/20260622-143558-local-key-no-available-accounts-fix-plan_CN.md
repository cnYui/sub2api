# 本机自用 Key 503 修复计划

## 背景

用户使用本机自用 API Key 访问 `https://api.aaccx.pw/v1/models` 成功，但访问 `/v1/chat/completions` 返回 503。

## 诊断结论

Sub2API 日志显示请求进入 `codex-pool-local-unlimited` 分组后账号选择失败：

- `openai_chat_completions.account_select_failed`
- `error: no available accounts`

数据库中该分组只有一个上游账号 `cliproxy-local-openai`。该账号处于临时不可调度状态，原因为：

`Post "http://host.docker.internal:8317/v1/responses": dial tcp 192.168.65.254:8317: connect: connection refused`

CLIProxyAPI 当时只监听 `127.0.0.1:8317`，Docker 容器从 `host.docker.internal` 访问宿主机服务时无法命中该 loopback 监听。

## 修复设计

不改 Sub2API 调度逻辑，不改账号池模型映射。只做运行链路修复：

1. 将 CLIProxyAPI 的监听地址从 `127.0.0.1` 改为默认空 host，使 Docker 容器可通过 `host.docker.internal:8317` 访问。
2. 保持 CLIProxyAPI `remote-management.allow-remote=false`，避免远程管理接口开放。
3. 重启 LaunchAgent 托管的 CLIProxyAPI。
4. 从 Sub2API 容器内验证 `host.docker.internal:8317/v1/models` 可达。
5. 清除账号 `cliproxy-local-openai` 的临时不可调度标记。
6. 重新从公网 `https://api.aaccx.pw/v1/chat/completions` 验证实际生成请求。

## 风险与回滚

风险：CLIProxyAPI 8317 会监听宿主机所有接口。当前公网入口没有直接映射该端口，且管理 API 远程访问仍关闭。

回滚：将 CLIProxyAPI `config.yaml` 的 `host` 改回 `127.0.0.1` 并重启 LaunchAgent。
