# yui.web 旧 Shop 公网路由下线结果

## 变更

已修改公网 nginx 配置：

- `/opt/homebrew/etc/nginx/servers/aaccx-root.conf`

新增规则：

```nginx
location ~ ^/shop/(login|register|reset-password|account|admin|redeem|key|query|order|pay|result|content)(/index\.html|/)?$ {
    add_header Cache-Control "no-store" always;
    return 410;
}
```

该规则位于 yui.web fallback 之前，只影响 `aaccx.pw` 公网入口下指定旧 Shop 页面。yui.web 本地代码、Sub2API 代码、CLIProxyAPI 配置均未修改。

## 已下线路由

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

同时覆盖无尾斜杠和 `/index.html` 形式，例如 `/shop/login`、`/shop/login/index.html`。

## 保留路由

- `/shop/`：仍返回 200，保留图片热区中转入口。
- `/shop/guide/`：仍返回 200，本次未下线。
- `/dashboard`：仍返回 Sub2API 前端 200。
- `/v1/models`：无 Key 时仍返回 Sub2API 风格 401，说明 API 主链路未受影响。

## 验证

- 改前红灯：目标路由返回 200 或 302，不是 410。
- `nginx -t`：通过。
- `nginx -s reload`：成功。
- 本机 `Host: aaccx.pw` 请求 `http://127.0.0.1:8080` 下目标路由：全部 410。
- 公网 `https://aaccx.pw` 下目标路由：全部 410。
- 公网保留路由：
  - `/shop/`：200
  - `/shop/guide/`：200
  - `/dashboard`：200
  - `/v1/models`：401 `API_KEY_REQUIRED`
