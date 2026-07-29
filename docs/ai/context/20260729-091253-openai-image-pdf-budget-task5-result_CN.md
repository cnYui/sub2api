# OpenAI 图片与 PDF 请求前预算 Task 5 结果

时间：2026-07-29 09:12:53 +09:00

## 范围

- 仅修改 `codex/openai-billing-atomic-hold` 隔离 worktree 的本地代码与测试。
- 未启动、重启、迁移或写入公网 `18080/18086`、数据库、Redis、Nginx 或流量链路。

## 实现

- 新增附件检查器：内联图片提取尺寸和 `low/high/auto`；`file_id` 在请求前拒绝；原始 base64 与 data URL PDF 均解析文本、页数和 MediaBox。
- 附件上限为 20 MiB、PDF 200 页；不可解析、超限、视频和超时附件均在请求前失败。PDF 解析等待受 2 秒上下文限制。
- 远程图片下载复用 URL 格式校验，并在预解析和实际连接时拦截私网/DNS 重绑定；禁用代理、限制重定向、单附件限制 20 MiB。
- 图生图 multipart 上传和 mask 从已读取字节提取真实尺寸；URL、mask 与上传统一进入附件输入 Token 估算，不能按零成本继续授权。
- 预算器把图片、PDF 文本和每页视觉 Token 纳入输入成本；多图输出预算不足时仅收紧 `n`，至少保留一张，附件输入不收紧。

## 验证

已通过：

```powershell
go test -tags=unit ./internal/service -run 'TestInspectOpenAIAttachments|TestEstimateOpenAIAttachmentInputTokens|TestFitOpenAIBillingBudgetReducesImageCount|TestOpenAIGatewayServiceParseOpenAIImagesRequest_MultipartEditReadsUploadDimensions|TestOpenAIImagesInputTokenUpperBoundUsesUploadDimensions|TestInspectOpenAIImagesInputTokensIncludesURLAndMask' -count=1
go test -tags=unit ./internal/util/urlvalidator -count=1
```

未将此前超过本机命令时限且未取得退出码的 `go test -tags=unit ./internal/service` 全量套件标记为通过；提交前仅以本轮定向结果为证据。
