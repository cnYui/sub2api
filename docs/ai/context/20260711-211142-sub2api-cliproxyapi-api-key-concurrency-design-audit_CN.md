# Sub2API 与 CLIProxyAPI API Key 并发设计核对

时间：2026-07-11 21:11 JST

## 核对范围

- `20260710-112016-api-key-concurrency-replacement-design_CN.md`
- `20260710-112016-api-key-concurrency-replacement-implementation-plan_CN.md`
- `20260710-114846-api-key-concurrency-replacement-result_CN.md`
- Sub2API 当前本地 `main`、公网 18084 镜像与 Redis 运行态
- CLIProxyAPI 当前源码、`config.yaml`、8317 运行进程与目标测试

## 结论

按三份文档本身核对，两个项目的主要实现方向一致：

- Sub2API 已把模型请求入口的第一层调用方槽位从 `user_id` 替换为 `api_key_id`。
- Redis 当前使用 `concurrency:api_key:{apiKeyID}`，未发现运行中的 `concurrency:user:*` 槽位。
- CLIProxyAPI 没有按 Sub2API 共用的上游 Key 设置 5 并发；普通请求当前为 `global=100/per-api-key=100`，图片为 `10/10`。
- Sub2API 上游账号级并发仍保留，当前唯一账号 `cliproxy-local-openai` 为 10，符合原设计“继续保护上游账号池”。

但用户最新口头目标“每个 API Key 硬上限 5，Sub2API 只负责鉴权和计费后进入 CLIProxyAPI”与当前实现存在两处实质差异，不能直接认定为完全满足。

## 发现

### 1. 每把 API Key 不是硬编码最高 5

Sub2API 认证上下文把 `apiKey.User.Concurrency` 写入 `AuthSubject.Concurrency`，各网关入口再用该值调用 `AcquireAPIKeySlotWithWait(apiKey.ID, subject.Concurrency, ...)`。

因此当前真实语义是“每把 API Key 的上限等于所属用户的 `users.concurrency`”，不是“无条件最高 5”。管理员编辑用户并发或并发兑换码把用户值改为 10 后，该用户每把 Key 都会获得 10 个槽位；值小于等于 0 时并发服务还会按不限流处理。

当前运行库 95 个未删除用户的 `users.concurrency` 全部为 5，所以当前运行结果恰好是每 Key 5，但代码没有硬上限保证。

这也是原设计文档自身的语义矛盾：目标写“每把最多 5”，方案却写“复用 `subject.Concurrency`，默认 5”。后者只能保证默认值，不能保证最高值。

### 2. Sub2API 仍有全站共享的上游账号并发 10

API Key 槽之后，Sub2API 仍会执行账号选择并抢 `concurrency:account:{accountID}`。当前运行库只有一个上游账号 `account_id=1/cliproxy-local-openai`，并发为 10。

因此即使多个用户创建多把 Key，每把 Key 各自可有 5 个槽，全站真正进入 CLIProxyAPI 的普通上游请求仍会先被 Sub2API 的共享账号并发 10 限制。两个各跑 5 并发的 Key 就可能占满该账号。

这符合三份文档“保留上游账号级并发”的设计，但不完全符合“Sub2API 只做 API Key 识别和计费，核验后直接进入 CLIProxyAPI”的最新口头描述。是否保留这层 10 并发，需要明确为新的架构决策。

### 3. CLIProxyAPI 当前配置兼容设计，但不是用户 Key 级限制

CLIProxyAPI 的 `inboundlimit` 在自身认证完成后读取 `apiKey` principal。Sub2API 调 CLIProxyAPI 使用的是共用上游访问 Key，所以 CLIProxyAPI 无法识别原始 Sub2API 用户 Key。

当前配置：

- 普通请求：`global-concurrency=100`、`per-api-key-concurrency=100`
- 图片生成/编辑：`global-concurrency=10`、`per-api-key-concurrency=10`

对当前单个共用上游 Key 而言，普通请求的 per-key 100 与 global 100 基本等价，是 CLIProxyAPI 全局保护，不会把每个用户错误限制为 5。图片 10/10 同理是全站图片保护。

8317 当前二进制包含 inbound limiter，进程于 `2026-07-11 11:16 JST` 启动，晚于 `config.yaml` 的 `2026-07-11 10:24 JST` 修改时间，因此当前进程启动时已读取该配置。

## 入口覆盖核对

以下生成型入口已按 API Key 抢槽：

- Claude Messages
- OpenAI Responses 与 Responses 子路径
- OpenAI Chat Completions
- Embeddings
- Images Generations / Edits
- Gemini native generate/stream
- Responses WebSocket 每 turn

`/v1/models`、`/v1/usage`、Gemini models 查询和 `messages/count_tokens` 不占模型生成并发槽，符合当前设计边界。

## 运行态证据

- 公网镜像：`sub2api-candidate:20260710-214000-a575f28c1-admin-sub-revoke-repurchase`
- API Key 并发提交：`176605740 feat: enforce per-api-key concurrency`
- `176605740` 是公网镜像提交 `a575f28c1` 的祖先。
- Redis 审计时存在 `concurrency:api_key:32/70/84/106`，未发现 `concurrency:user:*`。
- 唯一 Sub2API 上游账号并发：10。
- CLIProxyAPI 二进制 VCS revision：`0a02768e3cfd`，包含 inbound limiter；当前监听 8317。

## 测试

通过：

```bash
cd backend && GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/service ./internal/handler ./internal/repository
```

```bash
cd /Users/wujianxiang/CodeSpace/CLIProxyAPI && go test ./internal/api/middleware/inboundlimit ./internal/config
```

## 建议决策

如果最终要求是“无条件每 Key 最高 5”，应让 API Key 槽位使用固定常量 5，或至少使用 `min(subject.Concurrency, 5)`；同时停止把后台用户并发和并发兑换码解释成可提高 Key 上限，否则控制面仍会误导管理员。

如果最终要求是“默认每 Key 5，但允许管理员为某个用户调整其每 Key 上限”，当前代码符合该语义，应把文档中的“最高 5”改为“默认 5”。

如果最终要求是“Sub2API 鉴权计费后不再限制全站上游并发”，还需要单独决定移除或提高 Sub2API `cliproxy-local-openai` 的账号并发 10；这不属于原三份文档的既定范围。

本轮仅只读核对并新增审计文档/AGENTS 索引，未改业务代码、数据库、Redis、nginx、容器或 CLIProxyAPI 配置。
