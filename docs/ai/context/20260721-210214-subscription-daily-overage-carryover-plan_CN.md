# 订阅日额度超额顺延计划

## 背景

额度削减后，部分用户当天已用金额可能高于新的每日额度。用户希望当天视为已用满，超出的部分从后续自然日额度中继续抵扣。

## 目标

- 不改写 `usage_logs` / `usage_facts` 的真实发生时间。
- 订阅日窗口过期时，将上一日超额部分作为新日窗口的初始已用量。
- 超额超过一天额度时，按经过的自然日逐日抵扣，直到债务归零。
- 无限额订阅不产生 carryover。
- 管理员手动重置额度仍是强制清零，不承接 carryover。
- Dashboard 今日额度展示应能看到 carryover 后的有效占用。

## 实现范围

- 后端订阅窗口模型：增加纯函数计算日额度 carryover。
- 订阅限额热路径：过期日窗口不再简单清零，而是按 group daily limit 计算 carryover。
- 订阅窗口维护 SQL：`RefreshExpiredUsageWindows` 对过期日窗口写入 carryover。
- 请求成功后订阅用量累加 SQL：窗口过期时写入 `carryover + 本次成本`，避免午夜第一笔请求丢失债务。
- Dashboard quota 读模型：今日用量取今天事实用量与当前订阅日窗口用量的较大值，用于展示 carryover。
- 单元/集成测试覆盖 daily carryover、无限额、请求落库和 Dashboard 展示。

## 非目标

- 不给历史日志造明天的请求。
- 不改变周/月额度语义。
- 不改公网运行态。
- 不自动迁移生产数据。

## 验证

- Go 单元测试：订阅模型、缓存资格判断、SQL 生成相关测试。
- Go 集成测试：订阅窗口刷新、用量累加、Dashboard quota。
- 必要时重建本地 `sub2api-dev` 并清本地 Redis 订阅缓存。
