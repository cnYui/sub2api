# diagnose-sub2api-cpa skill 创建结果

## 结果

已创建个人 Codex skill：

- 路径：`C:\Users\yui\.codex\skills\diagnose-sub2api-cpa`
- 主文件：`SKILL.md`
- 参考：`references/runtime-map.md`
- 脚本：`scripts/public_smoke.ps1`
- UI 元数据：`agents/openai.yaml`

## 覆盖范围

- 公网真实入口：`https://api.aaccx.pw/v1`
- 文本 smoke：`GET /v1/models`、`POST /v1/responses`、`POST /v1/chat/completions`
- 图片 smoke：`POST /v1/images/generations`，需要显式 `-IncludeImage`
- 扣费核对：`usage_facts`、`usage_logs`、用户 `/dashboard` 和 `/usage`
- 常见排查：TLS/x509、8317 HTTPS、Sub2API account 1、Redis 调度快照、CPA 凭证池、`auth_unavailable`、429/502/503/504 和 `Retry-After`
- 安全边界：默认只读；不记录完整 Key；不默认改 DB/Redis/容器/Nginx/Cloudflare/CPA auth 文件

## 验证

- `quick_validate.py C:\Users\yui\.codex\skills\diagnose-sub2api-cpa` 通过。
- `public_smoke.ps1` PowerShell 语法解析通过。
- 缺失 `SUB2API_PUBLIC_API_KEY` 时脚本会停止并提示，不会请求公网。
- 使用本地 mock HTTP 服务验证文本 smoke 分支通过，覆盖 models/responses/chat completions。
- 使用本地 mock HTTP 服务验证 `-IncludeImage` 分支通过，覆盖 images generations。
- 未执行真实公网请求，未消耗额度，未读取或写入完整管理员 API Key。

## 使用提示

完整部署后验收时先临时设置：

```powershell
$env:SUB2API_PUBLIC_API_KEY = "<临时填入，不要保存>"
powershell -ExecutionPolicy Bypass -File "$HOME\.codex\skills\diagnose-sub2api-cpa\scripts\public_smoke.ps1"
powershell -ExecutionPolicy Bypass -File "$HOME\.codex\skills\diagnose-sub2api-cpa\scripts\public_smoke.ps1" -IncludeImage
```

脚本输出的 `x_client_request_id` 是后续查 `usage_facts`、`usage_logs` 和面板的主关联 ID。
