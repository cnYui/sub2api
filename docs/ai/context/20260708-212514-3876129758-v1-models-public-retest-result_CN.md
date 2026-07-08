# 3876129758@qq.com /v1/models 公网复测结果

## 结论

- 2026-07-08 20:24:32+08，使用 `3876129758@qq.com` 当前未删除 active 自动 Key 真实请求 `https://api.aaccx.pw/v1/models`，已返回 HTTP 200。
- 本次复测覆盖两把 active Key：
  - `api_keys.id=106/Codex++`
  - `api_keys.id=105/Codex`
- 返回体为 OpenAI 兼容模型列表，包含 `gpt-5.5`、`gpt-5.4`、`gpt-5.4-mini`、`gpt-5.3-codex`、`gpt-image-2` 等模型。
- 应用日志中两条 `/v1/models` 均为 200，未再出现 `AUTO_KEY_UNSUPPORTED_ENDPOINT`。

## 运行态

- 当前运行容器：`sub2api-candidate`
- 当前镜像：`sub2api-candidate:20260708-211028-4bf902234-model-path-whitelist`
- 容器状态：healthy

## 数据核对

- `api_keys.id=105/106` 的 `last_used_at` 已更新到本次请求时间附近，说明 Key 被成功认证并进入应用。
- 最近 10 分钟 `usage_logs` 中 `/v1/models` 记录数为 0，说明模型列表请求不产生计费记录。

## 注意

- 不带 API Key 直接 curl `https://api.aaccx.pw/v1/models` 会被 Cloudflare 返回 403 HTML challenge，这不是 Sub2API 的 API 结果。
- 真实用户请求必须带 `Authorization: Bearer <API Key>`。
