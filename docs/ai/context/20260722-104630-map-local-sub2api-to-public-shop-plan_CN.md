# 本地 18080/8317 链路映射到公网 `/shop` 计划

## 目标

- 将当前本地启动的 Sub2API 映射到 `127.0.0.1:18080`，CLIProxyAPI 映射到 `127.0.0.1:8317`。
- 目标公网链路为 `aaccx.pw -> Cloudflare -> nginx -> Sub2API 18080 -> CLIProxyAPI 8317 -> 上游账号`。
- 将公网 `https://aaccx.pw/shop` 指向该链路。
- 让用户可以通过公网 `/shop` 看到刚完成的周额度与 28 天订阅页面改动。

## 授权范围

- 用户已明确要求映射到公网 `aaccx.pw/shop`。
- 允许只在必要范围内修改公网入口配置，例如本机 Nginx/Cloudflare Tunnel 指向。
- 不操作生产数据库内容，不修改支付配置，不触碰 CLIProxyAPI 账号池。

## 风险

- `aaccx.pw/shop` 历史上归 yui.web，切到 Sub2API 会改变公开商店页来源。
- 若误改 Nginx/Cloudflare 指向，可能影响 `/api/*`、`/v1/*` 或其他公网路径。
- 若本地容器端口、域名 base path 或前端路由不匹配，公网 `/shop` 可能出现 404、空白页或静态资源路径错误。

## 执行策略

1. 只读确认本地服务 `sub2api-dev`、公网当前响应、Nginx/Cloudflare/Tunnel 状态。
2. 启动或切换 CLIProxyAPI 到 `127.0.0.1:8317`。
3. 启动或切换 Sub2API 到 `127.0.0.1:18080`。
4. 定位当前 `aaccx.pw/shop` 的实际入口配置。
5. 备份会被修改的入口配置文件，并记录恢复路径。
6. 仅修改 `/shop` 相关 location/route，不扩大到 `/api/*`、`/v1/*`。
7. reload/restart 入口服务后验证：
   - `https://aaccx.pw/shop`
   - `https://aaccx.pw/shop/`
   - 静态资源加载
   - 不破坏 `http://127.0.0.1:18080/health`
6. 写入结果文档。

## 回滚边界

- 若修改 Nginx：恢复备份配置并 `nginx -t && nginx -s reload`。
- 若修改 Cloudflare Tunnel：恢复原 tunnel 配置并重启 cloudflared。
- 若只重启本地 Docker：保持 `sub2api-localdev` 原 project，可再次 `docker compose --env-file .env.local-dev -p sub2api-localdev -f docker-compose.dev.yml up -d sub2api`。

## 不做事项

- 不改生产 DB。
- 不推送远端仓库。
- 不改公网 `/api/*`、`/v1/*` 的既有业务入口，除非只读确认它们已经依赖同一个目标且必须一起 reload。
