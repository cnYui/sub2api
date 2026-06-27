# 2246950894 公网模型连通性排查计划

## 目标

排查 `2246950894@qq.com` 使用数据库中 active API Key 从当前公网入口访问模型服务时，是否能复现用户反馈的 `502 Bad Gateway`。

## 必须确认

- 用户、API Key、订阅、分组是否仍为 active。
- 使用数据库中的 active API Key 访问当前公网 OpenAI 兼容入口：
  - `GET https://api.aaccx.pw/v1/models`
  - `POST https://api.aaccx.pw/v1/chat/completions`
  - 必要时补测 `POST https://api.aaccx.pw/v1/responses`
- 查询最近日志中是否存在该用户或该 Key 对应的 `502`、上游错误、鉴权错误或超时。

## 约束

- 不修改用户权益、Key、订阅、分组、上游账号池或 nginx 配置。
- 不在文档、日志和回复中记录完整 API Key、内部 token、HMAC secret。
- 如果公网请求成功，只能说明当前时刻和测试路径连通；仍需结合日志判断用户看到的 `502` 是否来自历史上游抖动、错误路径或客户端配置。

## 步骤

1. 读取数据库中 `2246950894@qq.com` 的用户、active API Key、订阅和分组信息，输出时只保留 Key 掩码。
2. 用完整 Key 仅在本机命令内发起公网请求，记录 HTTP 状态、请求 ID、响应摘要和是否落库。
3. 查询 `usage_logs`、`ops_error_logs`、`ops_system_logs` 中该用户、该 Key 前缀或相关请求时间窗口的记录。
4. 将结论和证据新建结果文档保存到 `docs/ai/context/`。
