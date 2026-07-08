# 微信登录绑定启用排查与设计

## 背景

用户在个人资料页“登录方式绑定”中看到微信为“未绑定”，但没有“绑定微信”按钮，希望打开微信登录绑定。

## 只读排查结论

- 当前本地/公网入口 `18084` 与 `https://api.aaccx.pw` 的公开设置均返回：
  - `wechat_oauth_enabled=false`
  - `wechat_oauth_open_enabled=false`
  - `wechat_oauth_mp_enabled=false`
  - `wechat_oauth_mobile_enabled=false`
- 当前 `sub2api-candidate-postgres.settings` 里只查到 `wechat_connect_open_enabled`、`wechat_connect_mp_enabled` 两个空值，没有微信 AppID / AppSecret。
- `sub2api-candidate` 容器环境变量里没有微信 OAuth 相关配置。
- 前端 `ProfileIdentityBindingsSection.vue` 已支持微信绑定按钮；后端已有 `/api/v1/auth/oauth/wechat/bind/start`、`/callback` 和 pending OAuth bind 流程。因此当前主要是运行态配置未启用，不是需要新增绑定功能。

## 推荐方案

优先启用微信开放平台 PC 网站扫码登录：

- 写入 `wechat_connect_enabled=true`
- 写入 `wechat_connect_open_enabled=true`
- 写入 `wechat_connect_open_app_id`
- 写入 `wechat_connect_open_app_secret`
- 写入 `wechat_connect_mode=open`
- 写入 `wechat_connect_scopes=snsapi_login`
- 保留或写入 `wechat_connect_frontend_redirect_url=/auth/wechat/callback`
- 可选写入 `wechat_connect_redirect_url=https://api.aaccx.pw/api/v1/auth/oauth/wechat/callback`

此方案适合桌面浏览器登录绑定页，用户点击后进入微信开放平台扫码登录。

## 备选方案

- 公众号网页授权：启用 `wechat_connect_mp_enabled=true`，只适合微信内置浏览器；桌面浏览器仍不会出现可用绑定按钮。
- 仅显示按钮但不配置凭据：不推荐。用户点击后会进入不可用或提供商错误状态，等于制造坏入口。

## 后续执行前提

需要用户提供或在后台设置页填入微信开放平台 PC 应用的 AppID / AppSecret。密钥不得写入源码、提交、长期文档或日志摘要；执行时只允许进入运行态 DB/settings。
