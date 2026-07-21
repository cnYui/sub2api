# 2026-07-22 周滚动订阅额度 28 天读路径归一化结果

## 已完成

- `SettingService.GetDefaultSubscriptions` 读取默认订阅时，会再按 group 检查一次公共 Codex 订阅并把有效期归一化为 28 天。
- `SettingService.GetAllSettings` 现在会把 `default_subscriptions` 的公共 Codex 项统一收口到 28 天，避免后台 settings 页面继续透出旧 30 天值。
- `SettingService.GetAuthSourceDefaultSettings` 的各 provider 订阅默认值也同步做了同样的读路径归一化，默认发放链路不再依赖旧 30 天事实。
- 新增服务层测试，覆盖：
  - 默认订阅读取归一化
  - 系统设置读取归一化
  - auth source 默认订阅读取归一化

## 验证

- `go test -tags unit ./internal/service -run "TestSettingService_(GetDefaultSubscriptions_NormalizesPublicCodexValidity|GetAllSettings_NormalizesPublicCodexValidity|GetAuthSourceDefaultSettings_NormalizesPublicCodexValidity|UpdateSettings_DefaultSubscriptions_PublicCodexUses28Days|UpdateSettingsWithAuthSourceDefaults_PublicCodexUses28Days)"`
- `go test ./...`

## 备注

- 未改历史迁移和存储结构，只补了读路径的公共 Codex 归一化。
- 未执行提交、推送或运行态变更。
