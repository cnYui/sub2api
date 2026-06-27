# 新生成 API Key 联通性测试计划

## 背景

用户担心普通用户在 `/keys` 页面连续生成两个或三个新 API Key 后，新 Key 可能无法请求到模型。

## 必须验证

- 使用与页面一致的创建接口 `POST /api/v1/keys`，创建多个新 Key。
- 每个新 Key 创建后立即请求公网入口：
  - `GET https://aaccx.pw/v1/models`
  - `POST https://aaccx.pw/v1/chat/completions`
- 重点确认返回不是 `401 INVALID_API_KEY`、`403 GROUP_*`、`403 SUBSCRIPTION_NOT_FOUND` 或 `429 USAGE_LIMIT_EXCEEDED`。
- 不在文档、提交或回复中记录完整 API Key，只记录掩码、状态码和必要诊断字段。

## 初步代码判断

- 前端 `KeysView.vue` 在提交前强制 `group_id !== null`，页面正常创建 Key 时必须选择分组。
- 后端 `APIKeyService.Create()` 在指定订阅分组时会校验用户是否有该分组 active subscription。
- 网关认证中间件会在订阅分组请求时再次通过 `GetActiveSubscription(user_id, group_id)` 和 `ValidateAndCheckLimits()` 校验。

## 测试方法

1. 优先使用当前已登录页面或同等用户态创建 2-3 个新 Key。
2. 对每个新 Key 立即执行 `/v1/models` 和最小 chat completion。
3. 查询数据库记录，核对新 Key 的 `user_id`、`group_id`、`status`、订阅状态和用量记录。
4. 若出现失败，按认证链路定位根因后再决定是否改代码。
