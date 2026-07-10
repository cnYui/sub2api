# GPT-5.6 模型列表刷新排查结果

## 背景

用户使用本地自动 Key 请求 `https://api.aaccx.pw/v1/models`，返回列表中没有 GPT-5.6。用户预期 2026-07-10 当前应已可使用 GPT-5.6，询问是否需要重启以及如何刷新模型列表。

## 只读排查结论

- `https://api.aaccx.pw/v1/models` 当前返回的是 Sub2API 后端 `backend/internal/pkg/openai/constants.go` 的静态 `openai.DefaultModels` 列表。
- 这把 Key 对应 `api_keys.id=32`、`user_id=13`，Key 自身 `group_id=NULL`，通过 active 订阅落到 `user_subscriptions.id=45` / `group_id=5`。
- `group_id=5` 为 `codex-pool-local-unlimited`，`platform=openai`，`models_list_config={"enabled": false}`。
- `group_id=5` 绑定的唯一 OpenAI 账号是 `accounts.id=1 / cliproxy-local-openai`，`base_url=http://host.docker.internal:8317/v1`，无 `credentials.model_mapping`，无 `extra.model_mapping`。
- Sub2API `GatewayService.GetAvailableModels()` 在分组账号没有 `model_mapping` 时返回 `nil`，`GatewayHandler.Models()` 因而回落到 `openai.DefaultModels`。
- `/v1/models` 有短缓存，默认 15 秒，但这只影响秒级刷新；当前问题不是缓存没有过期，而是展示源没有包含 GPT-5.6。

## 上游与真实调用验证

- 直连上游 CLIProxyAPI `http://127.0.0.1:8317/v1/models` 已返回：
  - `gpt-5.6-sol`
  - `gpt-5.6-terra`
  - `gpt-5.6-luna`
- 使用 Sub2API 公网 `/v1/responses` 真实请求 `model=gpt-5.6-sol` 返回 HTTP 200，状态 `completed`。
- 使用 Sub2API 公网 `/v1/responses` 真实请求 `model=gpt-5.6` 返回 HTTP 502；当前链路不应直接对外展示 `gpt-5.6` alias，除非后续显式增加 alias 映射到 `gpt-5.6-sol` 并完成验证。

## 根因

GPT-5.6 在上游已经可见，且 `gpt-5.6-sol` 在 Sub2API 真实调用可用；当前 `/v1/models` 不显示 GPT-5.6 的根因是 Sub2API OpenAI 模型列表展示层仍走静态默认列表。单纯重启应用不会生成新模型，因为运行态配置和代码默认列表都没有 GPT-5.6。

## 推荐刷新方式

运行态可选方案：

1. 更稳的永久方案：修改代码 `openai.DefaultModels` 加入 `gpt-5.6-sol`、`gpt-5.6-terra`、`gpt-5.6-luna`，走 TDD、构建镜像并重新部署。部署后等待 `/v1/models` 15 秒短缓存自然过期，或重启应用容器强制清空进程内缓存。
2. 不发版的运行态方案：给 `cliproxy-local-openai` 账号配置完整 `credentials.model_mapping`，把所有希望允许和展示的模型都做显式映射，例如 `gpt-5.6-sol -> gpt-5.6-sol`、`gpt-5.6-terra -> gpt-5.6-terra`、`gpt-5.6-luna -> gpt-5.6-luna`，同时补齐现有默认模型的 identity mapping，避免只加入 5.6 后旧模型被 `model_mapping` 白名单拦掉。
3. 若还需要按分组隐藏/排序，再在对应 OpenAI 分组启用 `groups.models_list_config`。注意 `models_list_config` 会被 `availableModels/defaultModels` 过滤，当前默认列表没有 5.6；因此在不改代码的情况下，必须先有账号 `model_mapping` 作为 allowed source，单独修改 `models_list_config` 不会让 5.6 显示。

注意：

- `models_list_config` 只控制 `GET /v1/models` 展示，不参与账号白名单、模型映射或网关调度；但它不能凭空放行不在默认列表或账号 `model_mapping` 中的模型。
- `model_mapping` 会参与账号模型支持判断和上游模型改写；如果走运行态 `model_mapping` 方案，必须一次性放入完整允许集，并先备份数据库。
- 如果希望客户端直接使用 `gpt-5.6`，需要单独做 alias 映射设计，例如 `gpt-5.6 -> gpt-5.6-sol`，并补测试/验证；不要只把 `gpt-5.6` 加进 `/v1/models`，否则列表显示可用但真实请求仍可能 502。

## 本轮未做事项

- 未修改数据库。
- 未重启容器。
- 未提交代码。
- 未构建或部署镜像。
