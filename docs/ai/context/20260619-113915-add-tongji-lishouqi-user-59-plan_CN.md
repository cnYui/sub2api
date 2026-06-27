# 添加 tongji_lishouqi 用户到 59 元套餐执行计划

## 背景

用户要求将 `tongji_lishouqi@163.com` 添加到 Sub2API，并将登录密码重置为用户指定值，同时使用 59 元套餐。

## 当前检查

- Sub2API 当前运行库中没有该邮箱对应的 active 用户。
- `auth_identities` 中没有该邮箱相关身份。
- yui.web 旧库中也没有该邮箱相关用户或订单。
- 59 元套餐对应 Sub2API 分组 `codex-pool-49-usd`，`group_id=4`，每日额度 `49 USD`。
- `codex-pool-49-usd` 当前已绑定上游账号 `cliproxy-local-openai`，可用于模型请求。

## 执行步骤

1. 生成 bcrypt 密码哈希，不在文档记录明文密码。
2. 创建或恢复用户 `tongji_lishouqi@163.com`，角色 `user`，状态 `active`。
3. 创建 active subscription，绑定 `group_id=4`，有效期按 30 天设置。
4. 创建一个默认 API Key，绑定 `group_id=4`，用于立即验证联通性。
5. 使用新 Key 验证：
   - `GET https://aaccx.pw/v1/models`
   - `POST https://aaccx.pw/v1/chat/completions`
6. 记录掩码、HTTP 状态和用量写入结果，不记录完整 Key。
