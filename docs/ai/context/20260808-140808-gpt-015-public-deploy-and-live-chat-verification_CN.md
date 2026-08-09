# GPT 0.15 新渠道公网发布与真实对话验证

## 公网恢复

- 根据管理员明确授权，恢复既有公网链路；未新建或重配 Cloudflare Tunnel。
- `sub2api-public-nginx-local` 已启动。挂载的既有 Nginx 配置上游为 `host.docker.internal:18082`；`nginx -t` 通过后已执行 reload。
- 既有 `cloudflared-windows-aaccx.yml` 对应的 Tunnel 已以隐藏进程启动。
- 本地 Nginx 健康检查 `http://127.0.0.1:8080/health` 和公网 `https://api.aaccx.pw/health` 均返回 HTTP `200`。

## 新分组真实请求

- 管理员提供的测试 API Key 原绑定分组 `9`。由于单把 API Key 只能绑定一个分组，已重绑至新分组 `13`，使请求只能选择新渠道。
- 使用 `https://api.aaccx.pw/v1/models` 验证模型列表，HTTP `200`，返回 `20` 个模型。
- 使用公网 `POST /v1/chat/completions` 对 `gpt-5.6-sol` 发起最小真实对话，HTTP `200`，模型回复“新渠道连通”。
- 用量记录 `usage_logs.id=308554`：`group_id=13`、`account_id=1132`、`rate_multiplier=0.1500`、`total_cost=0.0049250000 USD`、`actual_cost=0.0147750000 USD`。该记录证明请求命中新渠道并按 0.15 分组倍率计费。
- 当前应用的 `BILLING_FINAL_MULTIPLIER=20`，因此本次实际扣费为基础成本的 `3.0` 倍，即 `0.15 × 20`；它不改变分组的 0.15 倍率配置。

## 测试 Key 扣费归属

- 该 Key 属于 `xiaobianfuai@gmail.com`（用户 ID `448`）。
- 上述用量的 `billing_type=0`，即普通钱包余额；未使用流量卡或订阅套餐。
- `actual_cost=0.0147750000 USD` 与用户余额更新在同一数据库时间点提交，当前余额为 `99930.85747458 USD`。

## 回归验证

- `go test -tags unit ./internal/repository -run "TestSchedulerCachePurgeLegacyCredentialPayloadsDeletesOnlyAccountJSON|TestCredentialCodec" -count=1` 通过。
- API Key 和上游 Key 均未记录在仓库、本文档或命令输出中。
