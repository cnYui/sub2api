# 公网真实 Smoke：文本、生图与用户额度扣费结果

## 范围

- 时间：2026-07-22 本地 Windows 公网链路迁移后。
- 入口：`https://api.aaccx.pw/v1`。
- 用户 Key：由用户在对话中临时提供；本文档不记录完整 Key、密码、访问 token 或 Cloudflare/CPA 凭证。
- 归属用户：按 API Key 反查为 `2799523972@qq.com` / `user_id=31` / `api_key_id=65`。用户消息中写的 `27995293972@qq.com` 与 Key 归属邮箱不一致。

## 链路状态

- `cliproxyapi-local-dev`：`127.0.0.1:8317->8317/tcp`，healthy。
- `sub2api-dev`：`127.0.0.1:18080->8080/tcp`，healthy。
- `sub2api-public-nginx-local`：`127.0.0.1:8080->8080/tcp`，running。
- Windows `cloudflared` connector 已连接 tunnel `7f5fafd9-8a59-4013-ba42-3116dfc29463`，`windows_amd64 2026.7.2`。
- 公网只读检查：
  - `https://api.aaccx.pw/health`：200。
  - `https://aaccx.pw/shop`：200。
  - `https://aaccx.pw/purchase`：200。

## 请求前额度基线

- 订阅：`user_subscriptions.id=105`，`group=codex-pool-29-usd`。
- 当前权益段：`subscription_entitlement_periods.id=197`。
- 周窗口：`2026-07-22 00:00:00+08` 到 `2026-07-29 00:00:00+08`。
- 周限额：`78 USD`。
- 请求前周用量：`0 USD`。

## 真实公网请求

Smoke marker：`sub2api-smoke-20260722022501-89265abf`。

| Endpoint | 模型 | HTTP | x_client_request_id | DB request_id | 扣费 |
|---|---:|---:|---|---|---:|
| `POST /responses` | `gpt-5.6-terra` | 200 | `6e8b98f3-f2cf-400f-a1a5-fa0bc12b2d3d` | `client:6e8b98f3-f2cf-400f-a1a5-fa0bc12b2d3d` | `0.0020605` |
| `POST /chat/completions` | `gpt-5.6-terra` | 200 | `4fd0a9c2-71b9-4b43-881c-a673271b47fc` | `client:4fd0a9c2-71b9-4b43-881c-a673271b47fc` | `0.0008975` |
| `POST /images/generations` | `gpt-image-2` | 200 | `024553fd-22a0-4e5d-8de8-56a321fdfcbd` | `client:024553fd-22a0-4e5d-8de8-56a321fdfcbd` | `0.061895` |

补充：`GET /models` 使用同一 Key 返回 200；人工探测可见 13 个模型，包括 `gpt-5.6-sol`、`gpt-5.6-terra`、`gpt-5.6-luna`、`gpt-5.5`、`gpt-image-1`、`gpt-image-1.5`、`gpt-image-2`。

## 落库与扣费事实

- `usage_facts` 新增 3 条，全部 `billing_status=settled`，全部归属 `entitlement_period_id=197`。
- `usage_logs` 新增 3 条，全部 `billing_type=1`，归属 `subscription_id=105`。
- 生图明细：
  - `image_count=1`
  - `image_output_tokens=2058`
  - `image_output_cost=0.0617400000`
  - `actual_cost=0.0618950000`
- 文本明细：
  - `/v1/responses`：`input_tokens=359`，`output_tokens=5`，`cache_read_tokens=4352`，`actual_cost=0.0020605000`。
  - `/v1/chat/completions`：`input_tokens=329`，`output_tokens=5`，`actual_cost=0.0008975000`。

## 请求后额度

- 周用量：`0.064853 USD`。
- 周剩余：`77.935147 USD`。
- Dashboard quota API：
  - `period_mode=entitlement_period`
  - `quota_window_unit=week`
  - `window_usage_usd=0.064853`
  - `window_limit_usd=78`
  - `window_starts_at=2026-07-22T01:00:00+09:00`
  - `window_resets_at=2026-07-29T01:00:00+09:00`
- 订阅页 API：
  - `weekly_usage_usd=0.064853`
  - `effective_weekly_limit_usd=78`
  - `weekly_window_resets_at=2026-07-29T01:00:00+09:00`

## 未完成项

- 内置浏览器插件在当前 Windows 会话初始化失败，报错为 `failed to write kernel assets: 系统找不到指定的路径。`；因此本轮没有完成可视化页面截图核对。
- 已用同一登录接口读取 Dashboard quota 和订阅页 API，这两个接口是前端页面的数据源；与 DB 计数器一致。
