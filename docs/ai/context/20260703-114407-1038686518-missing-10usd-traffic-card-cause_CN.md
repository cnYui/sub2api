# 1038686518 缺少 10 USD 流量卡原因

## 结论

- `1038686518@qq.com` 在当前公网实际使用的 `sub2api-candidate-postgres` / 18084 数据层中没有 GPT/OpenAI 流量卡记录。
- 昨天 2026-07-02 的“给每个用户发 10 USD 流量卡”操作执行在 `sub2api-postgres` / 18080 数据层。
- 2026-07-02 15:00 JST 后 nginx 已切流到 18084，因此公网页面和公网 API 读取的是 18084，而不是发放时写入的 18080。

## 证据

- `20260702-132616-user-expiry-plus-10usd-traffic-card-result_CN.md` 记录：
  - 当时公网入口为 `api.aaccx.pw -> nginx 127.0.0.1:8080 -> sub2api 127.0.0.1:18080`
  - 操作对象为 `sub2api-postgres`
  - 新增 52 张 10 USD OpenAI/GPT 流量卡
- `20260702-150000-18084-main-head-rebuild-cutover-result_CN.md` 记录：
  - 2026-07-02 15:00 JST nginx 切流 `18080 -> 18084`
  - 当前公网拓扑为 `Cloudflare Tunnel -> nginx 127.0.0.1:8080 -> sub2api-candidate 127.0.0.1:18084`
  - 18084 使用 `sub2api-candidate-postgres`
- 当前只读核对：
  - 18084 中 `1038686518@qq.com` 有 active 订阅 `codex-pool-29-usd`
  - 18084 中该用户 `user_traffic_credits` 为 0 条
  - 18080 中仍可看到该用户 2026-07-02 批次的 10 USD 卡

## 影响

- 这是数据层切换导致的发放数据不在当前公网库中，不是前端渲染问题。
- 当前用户页面不显示 `GPT 流量包` 是符合当前 18084 数据事实的。

## 后续选择

1. 若 18084 是当前正式公网库，需要在 18084 中按同一批次逻辑补发缺失的 10 USD 流量卡。
2. 若 18080 才应是正式数据源，需要重新评估 nginx 是否应切回 18080。

## 敏感信息

- 本文档不记录完整 API Key、内部 token、SMTP 密码或 HMAC secret。
