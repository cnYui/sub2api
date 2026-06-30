# luzhiyuan2026@163.com 公网流量包扣费只读排查计划

## 目标

- 确认公网 18084 候选库中 `luzhiyuan2026@163.com` 今天订阅额度耗尽后，是否使用了 10 USD GPT/OpenAI 流量包。
- 确认是否产生真实扣费记录，而不是只在 API 返回层面放行。

## 范围

- 只读查询 `sub2api-candidate-postgres` 对应公网候选数据库。
- 不读取、不输出完整 API Key、内部 token、SMTP 密码或其他密钥。
- 不修改容器、数据库、Redis、nginx 或公网运行态。

## 排查路径

1. 根据邮箱定位 `users.id`。
2. 查询该用户当前 active subscription、今日用量、日额度和所属 group。
3. 查询该用户 10 USD 流量卡余额、状态、创建时间和最近变动。
4. 查询今天的 `traffic_credit_ledger` deduction 记录，确认金额、时间、关联 usage/order。
5. 对照今天 `usage_logs` / `billing_usage_entries` 中 `subscription_id IS NULL` 的记录，判断是否由流量卡兜底扣费。
6. 必要时查看应用日志中该用户相关 request/order/ledger 证据，但不输出敏感字段。

## 预期结论格式

- 明确回答：有没有使用流量包、有没有真实扣费。
- 给出扣费金额、扣费时间、扣费前后余额和对应用量记录。
- 若证据不足，说明缺口和下一步只读验证方式。
