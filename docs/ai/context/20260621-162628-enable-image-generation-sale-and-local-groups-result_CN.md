# 开启三个在售套餐与本机分组生图能力

时间：2026-06-21 16:26

## 背景

用户确认采用上一轮建议：三个在售套餐和本机自用分组全部打开生图能力，计费按图片分辨率固定单价计算。

本次只修改 Sub2API 运行态分组配置，不修改代码、不修改 CLIProxyAPI、不修改支付、订阅周期或套餐日限额。

## 执行方案

通过 Sub2API 本地后台 API 更新四个分组：

- `groups.id=2`：`codex-pool-19-usd`，对应 29 元订阅池，每日 19 USD
- `groups.id=3`：`codex-pool-29-usd`，对应 39 元订阅池，每日 29 USD
- `groups.id=4`：`codex-pool-49-usd`，对应 59 元订阅池，每日 49 USD
- `groups.id=5`：`codex-pool-local-unlimited`，本机自用无限额分组

统一配置：

```json
{
  "allow_image_generation": true,
  "image_rate_independent": false,
  "image_rate_multiplier": 1,
  "image_price_1k": 0.10,
  "image_price_2k": 0.20,
  "image_price_4k": 0.40
}
```

说明：

- 三个在售订阅池按图片实际分辨率消耗订阅日额度。
- 本机无限额分组同样配置单价，用于统一用量成本口径；由于该分组没有日/周/月额度限制，不限制本机自用量。
- 后台 API 更新分组后会调用 `InvalidateAuthCacheByGroupID`，已有 API Key 下次请求会读取新的分组权限和价格。

## 验证结果

后台 API 更新后读取四个分组，结果如下：

| 分组 | 日限额 | 生图 | 1K | 2K | 4K |
| --- | ---: | --- | ---: | ---: | ---: |
| `codex-pool-19-usd` | 19 USD | 已开启 | 0.10 | 0.20 | 0.40 |
| `codex-pool-29-usd` | 29 USD | 已开启 | 0.10 | 0.20 | 0.40 |
| `codex-pool-49-usd` | 49 USD | 已开启 | 0.10 | 0.20 | 0.40 |
| `codex-pool-local-unlimited` | 无限制 | 已开启 | 0.10 | 0.20 | 0.40 |

三档在售套餐原有日限额保持不变：`19 / 29 / 49 USD`。

## 端口验证

使用本机 API Key 访问 Sub2API 本地入口：

`POST http://127.0.0.1:18080/v1/images/generations`

发送缺少 `prompt` 的最小请求：

```json
{
  "model": "gpt-image-2"
}
```

返回：

```json
{
  "status": 400,
  "message": "Invalid request: prompt is required",
  "permission_blocked": false,
  "prompt_validation_seen": true
}
```

该验证不触发真实上游生图，也不产生图片成本；返回参数校验错误而不是生图权限拦截，说明本机分组生图开关已进入认证和路由链路。

## 安全说明

本次未在文档中记录完整 API Key、JWT、管理员密码、HMAC secret 或 OAuth token。
