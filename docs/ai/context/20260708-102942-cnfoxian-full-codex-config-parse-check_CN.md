# cnfoxian@gmail.com 完整 Codex 配置核对

## 背景

- 用户提供 Windows Codex 完整 `config.toml`，包含 custom provider、插件、marketplace、desktop、node_repl MCP、Windows sandbox、trusted projects 等配置。
- 目标：确认这份完整配置是否仍可用于访问 `https://api.aaccx.pw/v1` 的 `gpt-5.5`。

## 解析结果

- 用 Python `tomllib` 对完整 TOML 内容做语法解析，结果：`toml_parse=ok`。
- 解析出的请求关键字段：
  - `model_provider = "custom"`
  - `model = "gpt-5.5"`
  - `[model_providers.custom].wire_api = "responses"`
  - `[model_providers.custom].requires_openai_auth = true`
  - `[model_providers.custom].base_url = "https://api.aaccx.pw/v1"`
- `notify` 的 Windows 路径双反斜杠写法可解析为正常 Windows 路径。
- `marketplaces.*.source`、`mcp_servers.node_repl.command/env`、`projects.'...'` 使用单引号 literal string，反斜杠不会被 TOML 当作转义，语法上可用。

## 运行判断

- 请求链路相关配置与此前真实 `codex exec` 跑通的配置一致；此前同用户 active Key 已通过 `provider=custom`、`model=gpt-5.5`、`base_url=https://api.aaccx.pw/v1` 成功返回 `OK`，并在公网 DB 写入 `usage_logs.id=63383/63390`。
- 完整配置中的插件、marketplace、desktop、MCP、Windows sandbox、trusted projects 不改变模型请求的 provider/base_url/model/wire_api。
- 因此在用户 Windows 环境中，只要这些本地路径真实存在、API Key 登录/注入正常，这份配置仍应能访问 `gpt-5.5`。

## 风险点

- `disable_response_storage = true` 在 Codex 0.142.5 的 `--strict-config` 下是 unknown field；普通运行会忽略，但推荐删除，避免严格模式或后续版本启动失败。
- 如果用户仍报 502，需要优先确认实际请求模型是否被客户端改成 `step-3.7-flash`；该模型已验证会在当前唯一 OpenAI 上游返回 502，而 `gpt-5.5` 可用。
