# DeepSeek 账号实时请求验证

## 验证结果

2026-08-24 17:14（数据库时区 `+08:00`）通过生产网关对 DeepSeek 分组发送最小请求：

- 请求模型：`deepseek-v4-flash`
- 请求内容：要求上游只返回 `DS_LIVE_OK`
- 账号 Key：用户 `636` 的测试 Key `api_key_id=343`，绑定本地分组 `8`
- 最终路由账号：`account_id=6`（DeepSeek模型官方0.7折价格）
- 网关结果：HTTP `200`
- 上游返回：`DS_LIVE_OK`
- 用量：输入 11 token、输出 41 token，共 52 token
- 实际扣费：`0.0018572400 USD`
- 用量记录：`usage_logs.id=362700`

## 恢复过程

1. 第一次请求时账号仍为 `error / schedulable=false`，网关返回 `503 no available accounts`，没有触达上游。
2. 通过管理员账号的“恢复状态”清除了错误状态，但系统保留了 `schedulable=false`；再次请求仍为 `503 no available accounts`。
3. 在账号 ID `6` 行开启“调度已开启”后重试，成功命中上游并返回 HTTP 200。

当前账号状态为 `active`，`schedulable=true`，`error_message` 为空；未重建容器、未更换凭证、未修改上游配置。历史 `GROUP_DELETED` 错误审计仍保留，但本次成功请求未新增上游错误。
