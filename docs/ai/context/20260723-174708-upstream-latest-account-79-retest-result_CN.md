# 内层 latest 账号 79 复测结果

## 范围

- 操作对象：本地内层 latest Sub2API，`http://127.0.0.1:18086`。
- 账号：`id=79 / 30D team 26835`。
- 分组：`groups.id=2 / internal-openai-upstream`。
- 测试模型：`gpt-5.4`。
- 未触碰公网 Nginx、Cloudflare、公网容器、外层定制版 Sub2API 用户/套餐/流量卡/计费事实。

## 复测前状态

- `accounts.id=79` 存在。
- `account_groups` 绑定存在：`account_id=79, group_id=2`。
- 复测前 DB 状态：`status=error, schedulable=true`。
- 运行日志显示该账号此前被真实转发请求命中，并因连续上游 `403` 触发 `openai_403_temp_unschedulable`，第三次后触发 `account_disabled_auth_error`。

## 复测过程

- 先通过正式管理接口清理旧错误态和临时不可调度状态，避免旧状态影响复测。
- 随后调用 `POST /api/v1/admin/accounts/79/test`，请求体仅指定 `model_id=gpt-5.4` 和短测试 prompt。
- 管理测试接口 HTTP 状态为 `200`，内容类型为 `text/event-stream`。
- SSE 事件只返回 `test_start` 和 `error`，没有 `complete/success`。

## 错误

- 上游返回：`403`。
- 错误消息：`Agent runtime has been deleted.`。
- 错误码：`biscuit_baker_service_agent_error_status`。

## 当前状态

- 为避免该账号继续进入可用调度池，复测后已将 `id=79` 调整为：
  - `status=error`
  - `schedulable=false`
- 分组绑定仍保留：`group_id=2`。
- 当前内层 OpenAI OAuth 账号统计：
  - 总数：79
  - `active/schedulable`：43
  - `error / false`：36

## 结论

- 用户判断“没有成功添加”在可用性层面成立。
- 更准确地说：账号已经导入 DB 并绑定到账号池分组，但该账号对应的 OpenAI agent runtime 已被删除，`gpt-5.4` 真实上游测试失败，因此不能算成功添加了一个可用账号。
