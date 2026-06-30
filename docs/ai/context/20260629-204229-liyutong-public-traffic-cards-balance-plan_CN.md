# liyutong2883@gmail.com 公网两张 10 USD 流量卡余额只读排查计划

## 目标

- 确认公网 18084 候选库中 `liyutong2883@gmail.com` 当前保留的两张 10 USD GPT/OpenAI 流量卡是否都已经用完。
- 对每张卡分别确认初始额度、剩余额度、扣费次数、扣费总额、首次/末次扣费时间。
- 确认扣费账本是否能对应到真实 `usage_logs`，避免只看余额字段导致误判。

## 范围

- 只读查询 `sub2api-candidate-postgres`。
- 不读取、不输出完整 API Key、内部 token、HMAC secret、SMTP 密码或其他敏感字段。
- 不修改公网应用、DB、Redis、nginx 或容器运行态。

## 排查路径

1. 根据邮箱定位 `users.id` 和当前余额。
2. 查询该用户所有 `user_traffic_credits`，重点看 `initial_usd=10` 的 OpenAI 卡。
3. 按 `credit_id` 汇总 `traffic_credit_ledger` 的 `deduction`：次数、金额、余额轨迹、首末时间。
4. 将 ledger 按 `request_id` join `usage_logs`，确认扣费金额与真实用量金额一致。
5. 查询今天和全量是否仍有可用流量卡余额。
6. 保存最终只读结论到新的上下文文档。
