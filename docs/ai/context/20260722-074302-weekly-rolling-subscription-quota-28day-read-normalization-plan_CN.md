# 2026-07-22 周滚动订阅额度 28 天读路径归一化计划

## 目标

把公共 Codex 订阅的 28 天/周额度口径，从“写入时归一化”补到“读取时归一化”，避免旧的 30 天默认值继续从设置接口、默认发放配置和后台页面里透出。

## 范围

- `SettingService.GetDefaultSubscriptions`
- `SettingService.GetAllSettings`
- `SettingService.GetAuthSourceDefaultSettings`
- 必要的单测补充

## 做法

- 复用 `defaultSubGroupReader`，在读出默认订阅后按 group 再检查一次公共 Codex 订阅。
- 对公共 Codex group 统一把 `ValidityDays` 归一化成 `28`，其他组保持原样。
- 不改历史存储，不改迁移。

## 验证

- 补充设置服务读路径测试，覆盖默认订阅和 auth source 默认订阅的公共 Codex 归一化。
- 跑相关 Go 测试，再跑全量 Go 测试。 
