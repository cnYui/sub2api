# Codex attestation 两个 URL 只读核对

时间：2026-07-10 21:34 JST

## URL

- README 小节：https://github.com/openai/codex/blob/main/codex-rs/app-server/README.md#attestation-generation
- PR：https://github.com/openai/codex/pull/20619

## 核心内容

- README `Attestation generation` 小节说明：支持上游 attestation 的桌面宿主需要在 `initialize` 阶段声明 `capabilities.requestAttestation`，并处理 app-server 发起的 `attestation/generate` 请求。
- app-server 会在向 ChatGPT Codex 上游发出需要 `x-oai-attestation` 的请求前，临时向桌面端索要 token；桌面端返回 `{ "token": "v1.<opaque>" }`。
- app-server 收到 token 后会包装成类似 `{ "v": 1, "s": 0, "t": "v1.<opaque>" }` 的外层 envelope，并作为 `x-oai-attestation` 转发给上游。
- 如果 app-server 已尝试 attestation 但在自身边界内失败，会发送同形状 envelope，但没有 `t`，`s` 表示失败原因：`1=timeout`、`2=request failed`、`3=request canceled`、`4=malformed response`。
- 如果没有客户端在初始化时声明支持 attestation，app-server 会省略 `x-oai-attestation`。

## PR 20619

- 标题：`[codex] request desktop attestation from app`
- 状态：已合并，`merged_at=2026-05-08T19:36:02Z`
- 规模：23 commits，65 changed files，1087 additions，40 deletions
- PR 说明：让 `codex-rs/app-server` 向桌面端请求 attestation token，并在限定的 ChatGPT Codex 请求路径上附加 `x-oai-attestation`。
- PR 明确说明：此仓库代码不直接生成 DeviceCheck token；签名 macOS DeviceCheck 生成由桌面 app 侧 PR 负责。
- 验证记录中提到：签名 `Codex.app` 返回的 DeviceCheck token 可被 Apple 验证；本地 MITM 看到 `/backend-api/codex/responses` 的 GET WebSocket 与 POST 请求均带有 `x-oai-attestation`。

## 对 Sub2API 类中转的含义

- 这两个链接本身没有直接写“封号”。
- 但它们确认 Codex 官方链路已经新增设备证明相关请求与 `x-oai-attestation` 转发机制。
- 如果上游开始强依赖该 header，纯 API 中转或非官方桌面宿主不能生成真实桌面端 attestation，可能会出现请求失败、风控或兼容性问题。
- 在确认 Sub2API/CLIProxyAPI 能正确处理这条链路前，建议对 Codex 类中转入口保持谨慎，不要把“没有立刻报错”视为长期安全。
