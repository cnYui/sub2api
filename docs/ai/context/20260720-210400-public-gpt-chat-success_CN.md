# 公网 GPT Chat 链路测试与修复结果

## 结论

公网 `https://api.aaccx.pw/v1/chat/completions` 已使用 `gpt-5.4-mini` 测试成功，返回 HTTP 200，模型回复 `pong`。

## 测试结果

- 本地 Sub2API：`http://127.0.0.1:18084/v1/chat/completions` 返回 200
- 公网 Sub2API：`https://api.aaccx.pw/v1/chat/completions` 返回 200
- 请求模型：`gpt-5.4-mini`
- 实际返回模型：`gpt-5.4-mini-2026-03-17`
- 返回内容：`pong`

## 关键修复项

1. 修正 CLIProxyAPI `auth-dir`
   - 原配置：`auth-dir: "auths"`
   - 修正后：`auth-dir: "/root/.cli-proxy-api"`
   - 原因：Docker 容器内实际挂载目录是 `/root/.cli-proxy-api`，旧配置导致 GPT/Codex 文件凭据没有加载。
   - 修复后 CLIProxyAPI 加载了 20 个文件凭据与 1 个 Claude API key。

2. 修正 CLIProxyAPI usage 回调地址
   - 从容器内不可达的 `127.0.0.1:4173` 改为 Docker 网络内 `sub2api:8080`。
   - 现在 `/api/internal/usage-events` 回调返回 200。

3. 同步 Sub2API usage 回调验签配置
   - 在候选 compose 中增加 `YUI_USAGE_EVENT_TOKEN` 与 `YUI_USAGE_EVENT_HMAC_SECRET`。
   - 与 CLIProxyAPI `.env` 保持一致。

4. 修正 Sub2API 上游账号配置
   - 上游 base URL 指向 Docker 网络内的 `https://cliproxyapi:8317/v1`。
   - 上游 API key 更新为 CLIProxyAPI 当前有效本地 key。

5. 修复 Sub2API 信任 CLIProxyAPI 本地 TLS CA
   - 将 CLIProxyAPI runtime CA 加入 Sub2API 容器信任链。
   - 重启 Sub2API 后 Go 进程重新加载系统 CA，公网请求恢复成功。

## 备注

- 之前误判为 Kiro Gateway/Claude-compatible 上游问题；实际当前使用的是 CLIProxyAPI 里的 GPT/Codex 文件凭据池。
- GPT 凭据池里部分 Codex 凭据存在 401/429，但仍有可用凭据，整体请求已可成功。
- 未提交代码。
