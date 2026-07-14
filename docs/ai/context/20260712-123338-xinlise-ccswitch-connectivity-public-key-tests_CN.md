# xinlise CC Switch 连接失败与两把 Key 公网实测

时间：2026-07-12 12:33 JST

## 结论

- 截图中的 CC Switch 连接失败与之前的 `thinking_signature_invalid` 不是同一个问题。
- 之前的加密上下文错误主要是服务端兼容问题：客户端正常续用旧会话，但 CLIProxyAPI 在多个 OAuth 上游账号之间轮询，账号绑定的 encrypted reasoning 无法被其他账号验证；当前错误分类又没有匹配上游直接返回的 `thinking_signature_invalid`，导致清理和重试未触发。用户只是触发了问题，不应归责为用户配置错误。
- 本次 CC Switch 截图则发生在 HTTP 状态码产生之前。两把 API Key 使用与 CC Switch 健康检测相同的公网 URL、User-Agent、流式请求格式和 `gpt-5.5@low` 语义均真实返回 200，因此不是 Base URL、Key、套餐、额度、模型或 Sub2API/CLIProxyAPI 故障。
- 截图发生时请求没有到达 Sub2API，而同一用户的 Codex Desktop 随后继续成功。最可能原因是 CC Switch 没有走 Codex Desktop 正在使用的代理/VPN 路径，或 CC Switch 保存了失效的“全局代理”配置。若用户 Terminal 直连也失败，则进一步归因到其本机 DNS、VPN、TLS 或当前网络出口。

## 截图信息

- 文件：`/Users/wujianxiang/Downloads/484.PNG`
- 创建时间：`2026-07-12 12:26:10 JST`，即北京时间 `11:26:10+08`。
- CC Switch Provider：`default`
- Base URL：`https://api.aaccx.pw/v1`
- 提示：`Connection failed: error sending request for url (https://api.aaccx.pw/v1)`，并归类为 DNS / 连接 / TLS / 超时。

## CC Switch 实际检测逻辑

- 本地源码：`/Users/wujianxiang/CodeSpace/cc-switch/src-tauri/src/services/stream_check.rs`
- Codex Provider 的健康检测不会只 GET Base URL，而是把 `/v1` Base URL拼成 `POST /v1/responses`。
- 默认模型是 `gpt-5.5@low`，请求为 Responses API 流式格式，User-Agent 为 `codex_cli_rs/0.80.0 ... Terminal`。
- `reqwest` 只有在未取得 HTTP 响应、且 `e.is_connect()` 时才返回截图中的 `Connection failed: error sending request for url`。
- 如果已经到达 Cloudflare/Sub2API并获得 400、401、403、429 或 5xx，CC Switch 会显示对应 HTTP 分类，不会显示当前连接错误。
- CC Switch 的全局 HTTP Client 会优先使用应用内“全局代理”；未设置时会读取进程环境中的系统代理。错误或已失效的代理地址会让健康检测在到达公网前失败。

## 公网基础链路

- DNS A：`104.21.23.246`、`172.67.214.182`
- DNS AAAA：Cloudflare IPv6 地址正常返回。
- `GET https://api.aaccx.pw/v1`：HTTP 200，约 `0.19s`。
- TLS：TLS 1.3，证书 `CN=aaccx.pw`，SAN 包含 `*.aaccx.pw`，Let's Encrypt 签发，有效期 `2026-06-12` 至 `2026-09-10`，校验结果 `0 (ok)`。

## 服务器侧时间证据

- 截图前没有该用户 `codex_cli_rs/0.80.0` 的成功 usage；本轮测试产生的是该用户最早两条该 User-Agent 记录。
- 截图后约 89 秒，`api_key_id=102` 的 Codex Desktop 在 `2026-07-12 11:27:35+08` 成功请求 `gpt-5.5`，费用 `0.146345 USD`。
- 说明用户账号和公网服务在截图附近仍可用，但 CC Switch 的独立 HTTP Client 没有使用相同的网络路径。

## 两把 API Key 公网实测

测试入口统一为：

`https://api.aaccx.pw/v1/responses`

测试模型统一为 `gpt-5.5`，均为最小流式请求。

### CC Switch 健康检测格式

- `api_key_id=99/codex`
  - HTTP 200
  - SSE 11 个事件
  - `response.completed=1`
  - 错误事件 0
  - `usage_logs.id=95313`
  - 费用 `0.023980 USD`
- `api_key_id=102/佳一老师`
  - HTTP 200
  - SSE 9 个事件
  - `response.completed=1`
  - 错误事件 0
  - `usage_logs.id=95314`
  - 费用 `0.004036 USD`

### Codex Desktop 格式对照

- `api_key_id=99/codex`
  - HTTP 200
  - `response.completed=1`
  - 文本为 `OK`
  - `usage_logs.id=95315`
  - 费用 `0.004391 USD`
- `api_key_id=102/佳一老师`
  - HTTP 200
  - `response.completed=1`
  - 文本为 `OK`
  - `usage_logs.id=95316`
  - 费用 `0.004391 USD`

四次测试合计新增费用：`0.036798 USD`，全部扣到 `subscription_id=98/group_id=12`。

## 建议处理顺序

1. 在用户电脑打开 CC Switch“设置 -> 代理 -> 全局代理”。
2. 如果不需要代理，清空旧代理配置；如果必须使用 Clash、Surge 或其他代理，填写当前真实监听地址和端口，并先点击“测试连接”。
3. 修改代理或 VPN 后完全退出并重新打开 CC Switch，确保其全局 HTTP Client 重新读取网络配置。
4. 在用户 Terminal 执行：

   ```bash
   curl -sS -o /dev/null \
     --connect-timeout 10 \
     -w 'http=%{http_code} remote=%{remote_ip} tls=%{ssl_verify_result}\n' \
     https://api.aaccx.pw/v1
   ```

   预期为 `http=200`、`tls=0`。
5. 如果 Terminal 成功而 CC Switch 仍失败，问题已收敛到 CC Switch 全局代理或应用缓存；不要修改 Base URL、Key、套餐或额度。
6. 如果 Terminal 也失败，再检查本机 DNS、VPN/TUN、代理端口、防火墙和系统时间。Base URL 应继续保持 `https://api.aaccx.pw/v1`。

## 本轮影响

- 已按用户授权用两把 Key 从公网分别执行 CC Switch 格式和 Codex Desktop 格式真实测试。
- 四次请求均成功并自然新增 usage，合计 `0.036798 USD`。
- 未输出完整 API Key。
- 未修改用户、Key、订阅、订单、额度配置、DB、Redis、nginx、Cloudflare、容器或 CLIProxyAPI 配置。
- 未修改业务代码、未重启、未部署。
