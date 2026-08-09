# GPT 0.15 新渠道接入与凭证安全验证

## 目标

- 新增 `GPT模型官方0.15倍新` 渠道，计费、模型范围和现有 0.15 OpenAI 分组一致。
- 不恢复已停止的公网 Nginx 与 Cloudflare Tunnel。
- 确认 Redis 调度缓存不再含上游明文凭证。

## 已执行的配置

- 新建分组 `groups.id=13`，名称 `GPT模型官方0.15倍新`，平台 `openai`，状态 `active`，倍率 `0.1500`。
- 新建账号 `accounts.id=1132`，同名，类型 `apikey`，状态 `active`，可调度，并发 `50`；账号绑定分组 `13`，优先级 `1`。
- 分组 `13` 与现有 0.15 分组 `9` 的倍率、模型列表配置、模型路由、支持范围、图片和视频计费策略均一致。
- 上游地址为 `https://huoshenai.net`；API Key 仅写入账号凭证并以 `enc:v1:` 加密存储，同时保留 HMAC 指纹用于重复凭证检测。密钥没有写入仓库、本文档、终端输出或 Redis。

## Redis 验证结论

- 启动清理会删除升级前的 `sched:acc:*`（保留 `sched:acc:last_used:*`）和 `sched:meta:*`，随后调度器会重建同名的脱敏账号元数据快照。
- 因此，键名存在不代表凭证存在；此前把 `accounts.type=apikey` 当成密钥特征的统计属于误报。
- 对 Redis 内 605 个调度 JSON 做字段级扫描：敏感字段 `0`；对常见明文密钥特征（`sk-`、`Bearer`、`X-API-Key`）扫描：`0`。

## 上游与运行状态验证

- 使用隔离迁移容器临时解密账号 `1132`，仅发起一次不计费的 `GET /v1/models`；结果为 HTTP `200`，返回 `8` 个模型。临时探针已删除。
- 本地应用容器 `sub2api-official-18082` 健康检查正常。
- `sub2api-public-nginx-local` 保持停止，未发现运行中的 `cloudflared` 进程；本次没有恢复公网服务。

## 后续注意事项

- 探针启动时发现 `security.url_allowlist.enabled=false` 的既有配置警告。该项未在本次变更中修改；若后续允许管理员录入任意上游地址，应单独启用白名单或限制为受控域名，降低 SSRF 风险。
