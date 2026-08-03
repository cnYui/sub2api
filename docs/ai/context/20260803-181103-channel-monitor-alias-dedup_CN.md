# 渠道监控模型别名去重

## 问题

DeepSeek 监控的附加模型同时存在以下三个 Flash 标识：

- `DeepSeek-V4-Flash`
- `deepseek-v4-flash`
- `deepseep-v4-flash`

前两项仅大小写不同，第三项是上游返回的拼写别名。它们不应作为独立模型参与监控展示。

## 处理

- 保留规范模型 `deepseek-v4-flash`。
- 删除大小写别名 `DeepSeek-V4-Flash`。
- 删除拼写别名 `deepseep-v4-flash`。

DeepSeek 监控最终只保留主模型 `deepseek-v4-pro` 和一个附加模型 `deepseek-v4-flash`。

## 验证

- 已通过管理端保存配置。
- 直接查询 `channel_monitors.extra_models` 确认只有 `deepseek-v4-flash`。
- 对全部 7 条监控按不区分大小写分组检查，无残留重复附加模型。
