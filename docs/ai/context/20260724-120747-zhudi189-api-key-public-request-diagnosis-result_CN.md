# `zhudi189@gmail.com` 真实 API Key 公网请求排查结果

时间：2026-07-24 12:07:47

## 结论

用户的有效 API Key 可正常使用，公网与双层 Sub2API 链路均正常：

- 使用该用户 `api_keys.id=90` 的有效 Key（未记录明文）
- 请求：`POST https://api.aaccx.pw/v1/responses`
- 模型：`gpt-5.5`
- 返回：HTTP `200`，响应状态 `completed`，模型输出 `OK`

## 计费落账

本次真实请求已在外层 `sub2api-dev` 的 `usage_logs.id=193055` 正常落账：

- `input_tokens=551`
- `output_tokens=5`
- `actual_cost=total_cost=0.0096500000 USD`
- `billing_incomplete=false`

## 额度状态

用户当前活跃订阅 `id=115` 的本周额度为 `520 USD`，已使用 `514.53604864 USD`，剩余 `5.46395136 USD`。

因此，用户反馈“不能用”不是 18080 与 18086 的连接问题；更可能是后续请求预算超过剩余额度而被拦截。当前 2x 实际计费倍率会使额度更快耗尽。

## 安全说明

未在本文档、命令输出或日志中保存完整 API Key。
