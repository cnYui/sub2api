# 生图教程 URL 二义性与 413 归因结果

## 背景

用户截图中看到大量 `413 Payload Too Large`，同时 `/usage-guide` 生图方法页面展示了图片接口说明。用户怀疑是否有人使用生图时 URL 或请求体填错，图片本身又较大，因此触发了大量 413。

## 日志归因

复查 `/opt/homebrew/var/log/nginx/access.log`：

- 全量 413 路径分布：
  - `/responses`：295 条
  - `/v1/responses`：19 条
  - `/v1/chat/completions`：2 条
  - `/v1/images/generations` / `/v1/images/edits`：0 条
- `/responses` 413 的 User-Agent 主要是 Windows `Codex Desktop/0.142.0`。
- 同一类 Codex Desktop User-Agent 周期性请求裸路径 `GET /models?client_version=0.142.0`，全量 access.log 中共 183 次，均不是 `/v1/models`。
- `/opt/homebrew/var/log/nginx/error.log` 中只有 1 条 256MB 边界 413 明细，是本轮验证 `268435457 bytes` 时产生，不是用户图片接口流量。

## 判断

- 当前这批 413 不是生图接口集中触发。若用户走图片接口，日志应出现 `/v1/images/generations` 或 `/v1/images/edits` 的 413；实际为 0。
- 裸 `/responses` 更符合 Codex Desktop 类客户端把 Base URL 配成不带 `/v1` 的入口，或者客户端本身按根路径拼接 Responses API。
- 图片请求确实可能很大，尤其图生图 multipart 上传或 base64 JSON；但这次 access.log 证据不支持“图片接口导致大量 413”。
- Nginx 上限已在前一轮从 100MB 调整到 256MB，并补了裸 `/responses` 等代理；公网真实 100MB+ 请求仍可能受 Cloudflare 套餐上传上限影响。

## 已修改

- `frontend/src/views/user/UsageGuideView.vue`
  - 生图方法不再用 `POST /v1/images/edits` 这种容易和 Base URL 重复拼接的句式。
  - 明确区分：
    - 客户端 Base URL：`https://api.aaccx.pw/v1`
    - 分离填写时的接口路径：`/images/edits`
    - 完整 URL：`https://api.aaccx.pw/v1/images/edits`
    - 文本生图完整 URL：`https://api.aaccx.pw/v1/images/generations`
- `frontend/src/views/user/__tests__/UsageGuideView.spec.ts`
  - 新增断言覆盖 Base URL、相对路径、完整 URL 分离说明。
  - 新增断言避免重新出现 `POST /v1/images/edits`。

## 验证

- 红灯验证：
  - `pnpm --dir frontend test:run src/views/user/__tests__/UsageGuideView.spec.ts`
  - 修改页面前失败，缺少 `客户端 Base URL 填 https://api.aaccx.pw/v1`。
- 绿灯验证：
  - `pnpm --dir frontend test:run src/views/user/__tests__/UsageGuideView.spec.ts`：6 tests passed。
  - `pnpm --dir frontend typecheck`：通过。
  - `pnpm --dir frontend build`：通过；仅有既有 Vite chunk 和 Browserslist 提示。

## 影响范围

- 本次只改普通用户 `/usage-guide` 生图教程文案和测试。
- 未修改后端路由、Nginx、Cloudflare、计费、API Key、订阅、图片生成价格或公网运行配置。
