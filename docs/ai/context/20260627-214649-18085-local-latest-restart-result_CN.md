# 18085 本地蓝绿测试环境重启结果

时间：2026-06-27 21:46 JST

## 目标

把当前本地 `main` 最新前后端版本重新部署到本地蓝绿测试端口 `18085`，并避免影响公网当前运行的 `18084` 候选链路。

## 执行

- 使用工作区：`/Users/wujianxiang/CodeSpace/sub2api`
- 使用分支：`main`
- 使用 HEAD：`e4704061d fix: 统一认证中间件计费准入`
- 构建镜像：`sub2api-smtp-test:20260627-214036`
- 镜像 digest：`sha256:00f926480c5f01e59a06796a1f8e9bfe569598d40e2a236a266880702d7bb4bc`
- 只替换应用容器：`sub2api-smtp-test`
- 保留测试数据库和 Redis：
  - `sub2api-smtp-test-postgres`
  - `sub2api-smtp-test-redis`
- 复用旧应用容器环境变量启动新容器，未打印或记录敏感配置值。
- 未执行会影响公网 `sub2api-candidate*` 的 compose/down/rm 操作。

## 验证

- `sub2api-smtp-test`：`Up ... (healthy)`，端口 `127.0.0.1:18085->8080/tcp`
- `sub2api-candidate`：`Up ... (healthy)`，端口 `127.0.0.1:18084->8080/tcp`
- `curl -fsS http://127.0.0.1:18085/health` 返回：`{"status":"ok"}`
- `curl -fsS http://127.0.0.1:18084/health` 返回：`{"status":"ok"}`
- 18085 前端资源：
  - `assets/index-CXmPznNo.js`
  - `assets/index-nffSQZgD.css`
  - `assets/pkg-i18n-CRLwLFIo.js`
  - `assets/pkg-misc-CjRx2-Hi.js`
  - `assets/pkg-misc-DB0Q8XAf.css`
  - `assets/pkg-vue-BqGtxt06.js`
- 18085 公开设置非敏感状态：
  - `registration_enabled=true`
  - `email_verify_enabled=true`
  - `password_reset_enabled=true`
  - `payment_enabled=false`
  - `site_name=天才程序员小站`

## 结论

18085 本地蓝绿测试环境已切到当前本地 `main` 最新镜像；公网 18084 候选环境健康状态保持正常，未被本次操作替换或重启。
