# 添加 2316095427@qq.com 用户平台额度补充

## 背景

读取最近正常邮箱注册用户后确认：当前注册链路会为新用户初始化 4 条 `user_platform_quotas` 记录，平台分别为：

- `anthropic`
- `openai`
- `gemini`
- `antigravity`

当前系统没有配置 `default_platform_quotas`，因此默认平台额度字段均为 `NULL`，表示不在平台层额外限制。

## 决策

本次手工新增用户时，同时插入 4 条 `user_platform_quotas` 默认记录：

- `daily_limit_usd=NULL`
- `weekly_limit_usd=NULL`
- `monthly_limit_usd=NULL`
- usage 均为 `0`
- window start 均为 `NULL`

这样该用户的数据形态与正常注册用户一致；实际 29 元/月套餐限额仍由 `user_subscriptions.group_id=2` 对应的 `groups.daily_limit_usd=19` 控制。
