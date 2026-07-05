# 仅购买 GPT 流量包用户无法创建可用 OpenAI API Key 诊断

## 结论

- 问题真实存在。
- 更精确地说：仅购买 OpenAI/GPT 流量包、没有 active 订阅的用户，当前仍能看到 1 个可选分组 `default`，但该分组是 `anthropic/standard`，没有任何可绑定的 `openai` 分组。
- 因为流量包计费需要请求进入 OpenAI 平台链路，新用户没有可绑定的 OpenAI 分组，就无法创建能使用 GPT 流量包的 API Key。

## 当前公网入口

- 本机 nginx 配置显示 `api.aaccx.pw` 和 `aaccx.pw` 的 Sub2API API 路由反代到 `127.0.0.1:18084`。
- 当前公网判断以 `sub2api-candidate-postgres` 为准。

## 代码证据

- 前端 `frontend/src/views/user/KeysView.vue` 创建 API Key 时强制 `group_id` 非空；分组选项来自 `groupsAPI.getAvailable()`。
- `frontend/src/api/groups.ts` 调用 `GET /groups/available`。
- 后端 `backend/internal/service/api_key_service.go`：
  - `GetAvailableGroups()` 获取 active subscriptions 后构建 subscribed group 集合。
  - 对 `subscription` 类型分组，只允许用户有 active subscription 时绑定。
  - 对 `standard` 类型分组，才走公开/专属 allowed group 逻辑。
  - 该逻辑没有把 OpenAI/GPT 流量包权益映射成可绑定的 OpenAI 分组。
- `Create()` 和 `Update()` 再次调用同一套 `canUserBindGroup()` 校验，因此即使前端绕过，也不能直接绑定无 active subscription 的 OpenAI 订阅分组。

## 运行态数据证据

18084 当前 active groups：

- `default`：`anthropic / standard / non-exclusive`
- `codex-pool-19-usd`：`openai / subscription`
- `codex-pool-29-usd`：`openai / subscription`
- `codex-pool-49-usd`：`openai / subscription`
- `codex-pool-local-unlimited`：`openai / subscription`
- `codex-pool-89-usd`：`openai / subscription`
- `codex-pool-69-usd`：`openai / subscription`

18084 查询结果：

- 有 15 个 active 用户满足：OpenAI 流量包余额大于 0、没有 active subscription。
- 这 15 个用户按当前后端规则均为 `available_total=1`、`available_openai=0`。
- 其中多数用户没有任何 API Key；也就是新用户购买流量包后确实无法创建可用的 OpenAI API Key。
- 存在一个历史用户已有 active OpenAI Key 但无 active subscription，这类旧 Key 可以进入流量包兜底链路；问题集中在“新建 Key 选不到 OpenAI 分组”。

## 根因

当前系统把“API Key 可以绑定哪些 OpenAI group”完全建模为订阅权益；而 GPT 流量包是用户级 platform 余额权益，没有对应的 group 绑定授权。计费层已经支持 OpenAI 流量包兜底，但 API Key 创建/换绑权限层没有给流量包用户一个可进入 OpenAI 链路的 group。

## 后续修复方向

- 不建议把用户流量包转换成虚假的 active subscription，因为会污染订阅事实源和套餐展示。
- 更合理的方向是在 API Key 分组授权层引入“用户有可用 OpenAI 流量包时，可绑定一个受控的 OpenAI 流量包入口分组”。
- 这个入口分组可以复用现有 OpenAI subscription group 中的某个基础分组，或新增一个明确的 `traffic-pack-openai` 分组；需要同步校验账号绑定、模型范围、限额和 UI 文案。
