# 2466439791@qq.com API Key 公网联通性测试计划

## 目标

确认用户 `2466439791@qq.com` 是否已生成 API Key；如果已生成，使用该用户的 active API Key 通过当前公网入口向模型服务发送一次真实消息请求，判断 Key 是否可连通。

## 约束

- 不修改用户、套餐、API Key、上游账号或计费配置。
- 不在文档、日志摘录或回复中记录完整 API Key，只记录掩码。
- `/v1/models` 只能作为鉴权和可见模型验证，最终以真实 `/v1/chat/completions` 请求结果为准。

## 步骤

1. 从当前运行态数据库查询用户、身份、active API Key、分组和订阅状态。
2. 选择该用户现有 active API Key，生成本地临时变量用于请求，不落盘。
3. 通过公网入口 `https://api.aaccx.pw/v1/models` 验证 Key 鉴权和模型可见性。
4. 通过公网入口 `https://api.aaccx.pw/v1/chat/completions` 发送一条短消息，记录 HTTP 状态、模型、返回内容摘要和请求 ID。
5. 新建结果文档保存结论，仍不记录完整 Key。
