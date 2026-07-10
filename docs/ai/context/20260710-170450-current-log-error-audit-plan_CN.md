# 当前日志报错排查计划

- 时间：2026-07-10 17:04:50 JST
- 目标：只读查看当前 Sub2API/CLIProxyAPI/nginx/数据库日志，解释“很多用户说报错、用不了”的主要原因。
- 范围：
  - 查看运行容器健康状态、应用日志、nginx access/error、CLIProxyAPI 进程与日志。
  - 查询运行态数据库中的近期错误日志、usage 失败分布、用户/接口/模型集中度。
  - 区分 Cloudflare/nginx 源站不可达、Sub2API 本地拦截、上游 CLIProxyAPI/OpenAI 错误、用户额度/订阅/协议错误。
- 约束：
  - 不重启、不发布、不改 DB、不改 Redis、不改 nginx、不改 CLIProxyAPI 配置。
  - 不输出完整 API Key、商户密钥或用户敏感长 token。
- 判定方法：
  - 先看最近 1-2 小时错误高频词和 HTTP 状态分布。
  - 再按接口、模型、用户、上游账号聚合，判断是否为全局故障或局部用户问题。
  - 如需后续修复，另写实施计划并经确认。
