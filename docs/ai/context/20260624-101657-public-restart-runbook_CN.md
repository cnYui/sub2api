# 公网 Sub2API 重启与低影响发布方法

## 结论

当前公网主链路是 `Cloudflare Tunnel -> nginx 127.0.0.1:8080 -> Sub2API 127.0.0.1:18080 -> CLIProxyAPI 127.0.0.1:8317`。

`https://api.aaccx.pw/v1/*` 和控制台前端都经过 Sub2API。当前前端资源由 Go `embed` 编进 Sub2API 后端二进制，运行中的服务不会自动读取本地源码或新构建目录。因此：

- 只 `nginx reload` 不会加载新的前端代码。
- 只重启 CLIProxyAPI 不会更新控制台前端。
- 重启 Sub2API 会短暂影响 `https://api.aaccx.pw/v1/*`，正在进行的 Codex 流式请求可能断开。
- 如果只想避免长时间中断，优先使用蓝绿切换；如果能接受几秒到几十秒中断，直接重启最简单。

## 发布前检查

确认当前公网是否仍是旧资源：

```bash
curl -sS https://aaccx.pw/dashboard | rg 'app-index-|index-'
curl -sS https://aaccx.pw/assets/index-BMta9z_W.css | node -e "let s='';process.stdin.on('data',d=>s+=d).on('end',()=>console.log({hasZ35:s.includes('z-index:35'),hasZ30:s.includes('z-index:30')}))"
```

确认新构建产物是否包含修复：

```bash
rg -n 'z-\[35\]|z-index:35' frontend/src/components/layout/AppSidebar.vue backend/internal/web/dist/assets
```

不要在日志、命令历史或文档里写完整 API Key、数据库密码、SMTP 密码、HMAC secret。

## 方法一：直接重启 Sub2API

适用场景：可以接受 `api.aaccx.pw/v1` 短暂不可用，且当前没有重要 Codex 长流式请求。

步骤：

```bash
# 1. 使用包含目标改动的代码树构建前端和后端镜像或二进制
cd /Users/wujianxiang/CodeSpace/sub2api-mobile-touch-overlay-fix-20260623
cd frontend
pnpm install --frozen-lockfile
pnpm build
cd ..

# 2A. Docker Compose 部署时，重建并重启 Sub2API 服务
docker compose -f deploy/docker-compose.yml up -d --build --no-deps sub2api

# 2B. systemd 二进制部署时，替换 /opt/sub2api/sub2api 后重启
sudo systemctl restart sub2api
```

影响：

- 会影响 `https://api.aaccx.pw/v1/*`。
- nginx、Cloudflare Tunnel、CLIProxyAPI 不一定需要重启。
- 重启后新请求恢复，断开的长流式请求需要客户端重试。

验证：

```bash
curl -fsS http://127.0.0.1:18080/health
curl -sS https://aaccx.pw/dashboard | rg 'app-index-|index-'
curl -sS https://aaccx.pw/assets/index-BVx1zGpw.css | node -e "let s='';process.stdin.on('data',d=>s+=d).on('end',()=>console.log({hasZ35:s.includes('z-index:35')}))"
```

如果验证失败，先看 Sub2API 日志，不要立刻重启 CLIProxyAPI：

```bash
docker compose -f deploy/docker-compose.yml logs --tail=200 sub2api
sudo journalctl -u sub2api -n 200 --no-pager
```

## 方法二：蓝绿切换

适用场景：希望尽量不打断当前 `api.aaccx.pw/v1` 请求。

思路：

1. 保留旧 Sub2API 在 `127.0.0.1:18080`。
2. 新起一套 Sub2API 到 `127.0.0.1:18082`。
3. 健康检查和前端资源检查通过后，把 nginx upstream 从 `18080` 切到 `18082`。
4. `nginx reload` 是平滑 reload，旧 nginx worker 会继续处理已有连接。
5. 等旧流式请求自然结束后，再停止旧 Sub2API。

示例命令：

```bash
# 新实例必须使用相同配置和数据源，但监听不同端口
SERVER_PORT=8080 HOST=0.0.0.0 docker run -d \
  --name sub2api-green \
  -p 127.0.0.1:18082:8080 \
  --env-file /path/to/sub2api.env \
  -v sub2api_data:/app/data \
  sub2api:mobile-touch-overlay-fix

curl -fsS http://127.0.0.1:18082/health
curl -sS http://127.0.0.1:18082/ | rg 'index-|app-index-'

# 修改 nginx，把 Sub2API upstream 从 127.0.0.1:18080 改到 127.0.0.1:18082
nginx -t
nginx -s reload
```

注意：

- 蓝绿期间不要同时运行两套会重复执行后台任务的实例，除非确认后台任务不会重复扣费、重复发送邮件或重复调度。
- 如果 Sub2API 没有独立关闭后台任务的开关，蓝绿只适合短窗口切换：新实例验证通过后尽快切流量，再尽快停旧实例。
- 不能同时让新旧实例使用不同数据库 schema。

回滚：

```bash
# nginx upstream 改回 127.0.0.1:18080
nginx -t
nginx -s reload
```

## 方法三：仅前端样式小修的零重启临时覆盖

适用场景：只需要临时修复当前手机端遮罩层级，不想影响 `api.aaccx.pw/v1`；后续仍应走正式重建发布。

当前 Sub2API 会优先读取 `data/public` 下同路径静态文件。可以复制当前线上 CSS 到覆盖目录，并追加一个只命中移动遮罩的高优先级规则。

示例：

```bash
mkdir -p data/public/assets
curl -fsS http://127.0.0.1:18080/assets/index-BMta9z_W.css \
  -o data/public/assets/index-BMta9z_W.css
cat >> data/public/assets/index-BMta9z_W.css <<'CSS'

.fixed.inset-0.z-30.bg-black\/40.lg\:hidden {
  z-index: 35;
}
CSS
```

验证：

```bash
curl -sS http://127.0.0.1:18080/assets/index-BMta9z_W.css | rg 'z-index: 35|z-index:35'
curl -sS https://aaccx.pw/assets/index-BMta9z_W.css | rg 'z-index: 35|z-index:35'
```

限制：

- 这是临时覆盖，不替代源码修复和正式发布。
- 只适合本次这类纯样式层级问题。
- 如果后续正式重启发布了新资源 hash，应删除旧覆盖文件，避免继续维护过期静态资源。

## 推荐顺序

本次手机端触屏修复只涉及前端遮罩层级：

1. 如果正在使用 `https://api.aaccx.pw/v1` 跑 Codex，先不要直接重启。
2. 可以先用“零重启临时覆盖”快速缓解手机端点击问题。
3. 等 API 空闲窗口，再用“直接重启”做正式发布。
4. 如果必须在高峰期正式发布，使用“蓝绿切换”，但要确认后台任务不会重复执行。
