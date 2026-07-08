# xinlise@gmail.com /v1/models 公网复测结果

## 结论

- 2026-07-08 20:26:25+08，使用 `xinlise@gmail.com` 当前两把未删除 active 自动 Key 真实请求 `https://api.aaccx.pw/v1/models`，均返回 HTTP 200。
- 覆盖 Key：
  - `api_keys.id=99/codex`
  - `api_keys.id=102/佳一老师`
- 返回体为 OpenAI 兼容模型列表，包含 `gpt-5.5`、`gpt-5.4`、`gpt-5.4-mini`、`gpt-5.3-codex`、`gpt-image-2` 等模型。
- 应用日志中对应两条 `/v1/models` 均为 `status_code=200`，未出现 `AUTO_KEY_UNSUPPORTED_ENDPOINT`。

## 运行态

- 当前运行容器：`sub2api-candidate`
- 当前镜像：`sub2api-candidate:20260708-211028-4bf902234-model-path-whitelist`
- 容器状态：healthy

## 数据核对

- `api_keys.id=99/102` 的 `last_used_at` 已更新到本次请求时间附近。
- 最近 10 分钟 `usage_logs` 中该用户 `/v1/models` 记录数为 0，说明模型列表请求不产生计费记录。

## 影响

- 本轮只做真实公网请求复测和只读数据库/日志核对。
- 未改数据库、未构建镜像、未替换/重启容器、未改 nginx/Redis。
