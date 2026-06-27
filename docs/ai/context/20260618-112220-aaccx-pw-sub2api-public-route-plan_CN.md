# aaccx.pw 接入 Sub2API 公网路由计划

## 目标

让用户从 `https://aaccx.pw/shop` 页面点击进入 Sub2API 控制台，并让 `https://aaccx.pw/v1/models` 使用 Sub2API 用户 Key 返回模型列表。

## 当前状态

- `https://aaccx.pw/` 当前服务 yui.web 静态站。
- `https://aaccx.pw/shop` 当前服务 yui.web shop 页面。
- `https://aaccx.pw/v1/models` 当前返回 yui.web 静态站 404，不是 Sub2API。
- Sub2API 本地容器运行在 `127.0.0.1:18080`。
- CLIProxyAPI 本地上游运行在 `127.0.0.1:8317`。
- 本机 nginx 监听 `*:8080`，Cloudflare Tunnel 当前应指向该端口。

## 路由边界

保留 yui.web 作为主站和 shop 展示页：

```text
https://aaccx.pw/
https://aaccx.pw/shop
```

将以下路径代理到 Sub2API：

```text
https://aaccx.pw/v1/*
https://aaccx.pw/api/*
https://aaccx.pw/dashboard
https://aaccx.pw/login
https://aaccx.pw/register
https://aaccx.pw/keys
https://aaccx.pw/subscriptions
https://aaccx.pw/settings
https://aaccx.pw/assets/pkg-*
https://aaccx.pw/assets/app-index-*
```

说明：

- `/v1/*` 是 OpenAI-compatible API。
- `/api/*` 是 Sub2API 前端调用的管理 API。
- `/dashboard` 等是 Sub2API 前端路由。
- `/assets/pkg-*` 和 `/assets/app-index-*` 是 Sub2API 前端构建产物；只代理这些前缀，避免抢走 yui.web 的普通 `/assets/*`。

## shop 跳转

在 yui.web `/shop` 页面增加或调整进入控制台链接，目标为：

```text
https://aaccx.pw/dashboard
```

如果页面已有“进入控制台 / API 控制台 / 登录”等按钮，优先复用现有按钮。

## 验证

1. `curl https://aaccx.pw/v1/models` 无 Key 应返回 Sub2API 风格 401，而不是 yui.web 404。
2. 使用 `sk-LOCAL-454...e28804` 请求 `https://aaccx.pw/v1/models` 应返回 HTTP 200。
3. 使用同一 Key 请求 `https://aaccx.pw/v1/responses` 应返回 HTTP 200 并新增 `usage_logs`。
4. 浏览器打开 `https://aaccx.pw/shop`，点击控制台入口后应进入 `https://aaccx.pw/dashboard`。
5. `https://aaccx.pw/dashboard` 页面资源加载正常，不白屏。

## 安全边界

- 不在日志、文档、提交中记录完整 API Key。
- 不改公共 59 元套餐的限额。
- 不改 CLIProxyAPI 内部 Key 发放规则。
- 如果公网路由失败，回滚 nginx 变更即可恢复 yui.web 原状态。
