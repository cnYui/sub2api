# yui.web 旧 Shop 公网路由下线计划

## 目标

在公网 `aaccx.pw` 下线以下 yui.web 旧 Shop 页面入口：

- `/shop/login/`
- `/shop/register/`
- `/shop/reset-password/`
- `/shop/account/`
- `/shop/admin/`
- `/shop/redeem/`
- `/shop/key/`
- `/shop/query/`
- `/shop/order/`
- `/shop/pay/`
- `/shop/result/`
- `/shop/content/`

保留：

- `/shop/`：当前图片热区中转入口。
- `/shop/guide/`：本次用户未要求下线，暂时保留。

## 设计

只修改公网 nginx 分流配置 `/opt/homebrew/etc/nginx/servers/aaccx-root.conf`，不删除 yui.web 本地代码。

原因：

- 用户要求的是“公网这里的路由下线”，不是删除历史代码。
- yui.web 本地仍可能需要保留历史数据、迁移脚本、只读查询和回滚参考。
- nginx 层下线风险最低，不影响 `api.aaccx.pw` 和 Sub2API 主链路。

下线响应使用 HTTP `410 Gone`。它比 404 更明确，表示旧入口已退役；同时页面不会再展示旧登录、注册、账户、后台或兑换界面。

## 验证计划

改前红灯：

- 对上述路径执行 curl，当前预期不是 410。

改后绿灯：

- 本机 `Host: aaccx.pw` 请求 `http://127.0.0.1:8080/<path>` 返回 410。
- 公网请求 `https://aaccx.pw/<path>` 返回 410。
- `https://aaccx.pw/shop/` 仍返回 200。
- `https://aaccx.pw/shop/guide/` 仍返回 200。
- `https://aaccx.pw/dashboard` 仍返回 Sub2API 200。

## 风险

- 已登录旧 yui.web 用户访问旧账户页会看到 410；这是本次目标。
- 搜索引擎或旧链接访问这些路径会收到 410；符合下线语义。
- 不影响 `api.aaccx.pw/*`，不影响 `/v1/*` 模型调用链路。
