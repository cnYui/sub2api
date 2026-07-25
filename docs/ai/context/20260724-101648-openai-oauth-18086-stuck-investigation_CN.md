# 18086 OpenAI OAuth 授权页卡住只读排查

## 背景

用户在 `http://127.0.0.1:18086/admin/accounts` 添加 OpenAI/GPT OAuth 账号时，打开授权链接并登录后，OpenAI 页面停在 `auth.openai.com/sign-in-with-chatgpt/codex/...` 转圈，未跳到带 `code` 的本地回调链接。

## 运行态定位

- `18086` 对应容器 `sub2api-upstream-latest`，镜像 `weishaw/sub2api:latest`，端口映射 `127.0.0.1:18086->8080/tcp`，健康状态正常。
- 当前外层本地定制版是 `sub2api-dev:18080`，本问题发生在内层 upstream latest 控制台，不是外层 18080 的前端。

## 代码与页面证据

- OpenAI OAuth URL 由 `backend/internal/pkg/openai/oauth.go` 生成：
  - `AuthorizeURL=https://auth.openai.com/oauth/authorize`
  - `ClientID=app_EMoamEEZ73f0CkXaXp7hrann`
  - `DefaultRedirectURI=http://localhost:1455/auth/callback`
  - `scope=openid profile email offline_access`
  - `code_challenge_method=S256`
  - `codex_cli_simplified_flow=true`
  - `id_token_add_organizations=true`
- 18086 当前前端弹窗里已经生成授权链接，页面只读检查确认上述公开参数都存在，`code_challenge` 长度为 43，`state` 长度为 64。
- 前端未显示错误，Chrome 可读的 18086 管理页控制台错误为空。

## 日志证据

`docker logs --since 2h sub2api-upstream-latest` 显示：

- `2026-07-24 07:35:41 +08` `/api/v1/admin/openai/generate-auth-url` 返回 200。
- `2026-07-24 07:37:05 +08` `/api/v1/admin/openai/exchange-code` 返回 200。
- `2026-07-24 07:39:17 +08` `/api/v1/admin/openai/generate-auth-url` 返回 200。
- `2026-07-24 07:53:30 +08` `/api/v1/admin/openai/exchange-code` 返回 200。
- `2026-07-24 08:47:05 +08`、`08:56:14 +08`、`09:07:57 +08`、`09:08:00 +08` 后续仅看到 `/generate-auth-url` 返回 200，没有新的 `/exchange-code`。

这说明当前卡住的几次没有把授权结果带回 Sub2API；Sub2API 没有收到 code，也没有进入 token 兑换阶段。

## 判断

当前证据不支持“Sub2API 后端兑换或回调处理出错”。更可能是 OpenAI 授权页本身、浏览器环境或当前 ChatGPT 登录态在 `auth.openai.com` 内部卡住：

- 如果 OpenAI 成功跳到 `http://localhost:1455/auth/callback?code=...&state=...`，即使本机 1455 没有服务，浏览器地址栏也会出现 code，用户可复制回 18086。
- 当前实际停在 OpenAI 域名内，且 18086 日志没有新的 `/exchange-code`，失败点在 Sub2API 控制范围之外。
- 同一容器在当天早些时候已有 `/exchange-code` 200，说明这套生成/兑换链路本身不是全局坏掉。

## 建议

- 首选绕过浏览器授权页：使用 `Codex OAuth auth.json / AT 导入`、`Agent Identity auth.json` 或已有 RT 导入。
- 若继续手动 OAuth：
  - 重新生成授权链接后，用无扩展/无翻译插件的隐身窗口打开。
  - 清理或允许 `auth.openai.com`、`chatgpt.com`、`openai.com` 的站点 Cookie，尤其检查第三方 Cookie / 跟踪保护。
  - 换一个 Chrome Profile 或 Edge 测试，排除当前浏览器扩展/登录态干扰。
  - 若页面最终跳到 `localhost:1455/auth/callback?...code=...`，直接复制完整地址回 18086 第 3 步。
