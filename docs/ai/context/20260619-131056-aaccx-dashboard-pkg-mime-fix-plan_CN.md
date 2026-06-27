# aaccx.pw dashboard 白屏修复计划：移除过期 pkg rewrite

## 时间

2026-06-19 13:10 JST

## 用户问题

`https://aaccx.pw/dashboard` 白屏。浏览器报错显示多个 `/assets/pkg-*` JS/CSS 被返回 `text/html`，模块脚本和样式表因 MIME 不匹配被拒绝加载。

## 本地基线

已先跑本地流程：

- `sub2api` 容器运行中，映射 `127.0.0.1:18080 -> 8080`，Docker health 为 healthy。
- `node scripts/verify-local-frontend-assets.mjs` 通过：
  - `htmlAssets=6`
  - `checkedAssets=125`
  - `vendorReferences=0`
- 本地 `http://127.0.0.1:18080/dashboard` HTML 引用真实 `pkg-*` 资源：
  - `/assets/pkg-vue-DdvVI69T.js`
  - `/assets/pkg-i18n-ECmPCSvH.js`
  - `/assets/pkg-misc-DRGW1HPS.js`
  - `/assets/pkg-misc-DB0Q8XAf.css`
- 本地 `http://127.0.0.1:18080/assets/pkg-vue-DdvVI69T.js` 返回 `200 text/javascript`。
- 使用本机 active Key 验证本地 API：
  - `/v1/models` 返回 200，首个模型为 `gpt-5.5`
  - `/v1/chat/completions` 使用 `gpt-5.4` 返回 `pong`

## 红灯证据

公网同一个资源当前返回：

```text
https://aaccx.pw/assets/pkg-vue-DdvVI69T.js
200 text/html
```

同类问题也存在于 `api.aaccx.pw`，因为两个 nginx server 都保留了旧规则：

```nginx
location ^~ /assets/pkg- {
    rewrite ^/assets/pkg-(.*)$ /assets/vendor-$1 break;
    proxy_pass http://127.0.0.1:18080;
}
```

这会把真实存在的 `pkg-*` 资源改成当前运行产物中不存在的 `vendor-*`，后端 SPA fallback 返回 `index.html`，浏览器因此看到 `text/html`。

## 修复方案

推荐最小修复：

1. 在 `/opt/homebrew/etc/nginx/servers/aaccx-root.conf` 删除 `location ^~ /assets/pkg-` 内的 `pkg-* -> vendor-*` rewrite。
2. 在 `/opt/homebrew/etc/nginx/servers/cliproxy.conf` 做同样删除，保持 `aaccx.pw` 与 `api.aaccx.pw` 一致。
3. 暂时保留：
   - `/assets/app-index-* -> /assets/index-*`，继续规避入口缓存。
   - `/assets/*` 的 `Cache-Control: no-store`。
   - `vendor- -> pkg-` 响应体改写，虽然当前新产物不再需要，但保留它不会影响 `pkg-*` 新资源，后续可单独清理。

不采用的方案：

- 回退前端 chunk 命名到 `vendor-*`：会重新触发 Cloudflare 对 `vendor-*` 的拦截。
- 扩大 SPA fallback 或把未知静态资源强行 200：会掩盖真正缺失的资源，并继续制造 MIME 错误。
- 只修 `aaccx.pw`：`api.aaccx.pw` 仍可能被同类缓存和资源规则打坏。

## 验收计划

修复后执行：

1. `nginx -t`
2. 重载 nginx
3. 验证本地 origin 仍正常：
   - `node scripts/verify-local-frontend-assets.mjs`
   - 本地 `/v1/models` 和最小 chat
4. 验证公网 MIME：
   - `https://aaccx.pw/assets/pkg-vue-DdvVI69T.js` 返回 `text/javascript`
   - `https://aaccx.pw/assets/pkg-misc-DB0Q8XAf.css` 返回 `text/css`
   - `https://api.aaccx.pw/assets/pkg-vue-DdvVI69T.js` 返回 `text/javascript`
5. 验证 `https://aaccx.pw/dashboard` HTML 仍返回 200，且引用 `app-index-*` 和 `pkg-*`。
