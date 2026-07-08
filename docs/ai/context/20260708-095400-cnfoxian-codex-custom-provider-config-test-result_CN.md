# cnfoxian@gmail.com Codex custom provider 配置真实访问测试

## 背景

- 用户截图为 Windows Codex `config.toml`，核心配置：
  - `model_provider = "custom"`
  - `model = "gpt-5.5"`
  - `disable_response_storage = true`
  - `model_reasoning_effort = "medium"`
  - `[model_providers.custom]`
  - `wire_api = "responses"`
  - `requires_openai_auth = true`
  - `base_url = "https://api.aaccx.pw/v1"`
- 测试方式：本机创建临时 `CODEX_HOME`，写入等价配置，使用 `cnfoxian@gmail.com` 当前 active API Key 运行 `codex exec`；未输出或记录完整 API Key。
- 本机 Codex 版本：`codex-cli 0.142.5`。

## 配置核对

- `model_provider = "custom"` 与 `[model_providers.custom]` 匹配，语义正确。
- `model = "gpt-5.5"` 正确；该模型在当前公网链路可用。
- `wire_api = "responses"` 正确；会走 `/v1/responses`。
- `base_url = "https://api.aaccx.pw/v1"` 正确；Codex 会拼接为 `https://api.aaccx.pw/v1/responses`。
- `requires_openai_auth = true` 可用；Codex manual 说明该模式用于通过 OpenAI 认证访问 LLM proxy，并且会忽略 `env_key`。本次用 `CODEX_API_KEY` 注入该用户 Key，真实访问成功。
- `disable_response_storage = true` 在 Codex 0.142.5 的 `--strict-config` 下不是公开识别字段，报错：
  - `unknown configuration field disable_response_storage`
  普通启动时会忽略该字段并继续运行，但推荐从配置里删掉，避免版本/严格模式下启动失败。
- 截图中的 `notify = [...]` 行在图片里被截断；若实际文件没有完整闭合 `]` 或 Windows 路径转义不完整，会导致 TOML 解析失败。该字段与 API 访问无关，可先移除排除干扰。

## 真实访问结果

### 带截图字段普通运行

- 配置包含 `disable_response_storage = true`，不加 `--strict-config`。
- `codex exec` 返回 `OK`，退出码 `0`。
- stderr 显示：
  - `model: gpt-5.5`
  - `provider: custom`
  - `reasoning effort: medium`
- 数据库新增 `usage_logs.id=63383`：
  - `requested_model=gpt-5.5`
  - `billing_type=1`
  - `group_id=2`
  - `subscription_id=64`
  - `total_cost=0.0128140000`
  - User-Agent：`Codex Desktop/0.142.5 ... codex_exec; 0.142.5`

### 去掉无效字段严格运行

- 配置移除 `disable_response_storage = true`，加 `--strict-config`。
- `codex exec` 返回 `OK`，退出码 `0`。
- 数据库新增 `usage_logs.id=63390`：
  - `requested_model=gpt-5.5`
  - `billing_type=1`
  - `group_id=2`
  - `subscription_id=64`
  - `total_cost=0.0127390000`
  - User-Agent：`Codex Desktop/0.142.5 ... codex_exec; 0.142.5`

## 推荐配置

```toml
model_provider = "custom"
model = "gpt-5.5"
model_reasoning_effort = "medium"

[model_providers.custom]
name = "custom"
wire_api = "responses"
requires_openai_auth = true
base_url = "https://api.aaccx.pw/v1"
```

## 结论

- 用户这套配置的 provider/base_url/model/wire_api 思路是对的，真实访问公网 18084 可以成功。
- 不推荐保留 `disable_response_storage = true`，当前 Codex 0.142.5 严格配置会报 unknown field。
- 如果用户仍报 502，要看他实际请求的模型是否还是 `step-3.7-flash`；`gpt-5.5` 用这套配置已验证可用。
