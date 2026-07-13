# 流量卡进度条修复结果

## 结果

本地分支 `codex/fix-traffic-credit-progress` 已修复 `/subscriptions` 流量卡进度条把剩余额度误作动态分母的问题。

修复后：

- 10 USD 流量卡未使用时显示 `$0.00 / $10.00`，进度 0%。
- 剩余 7 USD 时显示 `$3.00 / $10.00`，进度 30%。
- 某个流量卡批次用满后继续按历史规则从可用汇总移除。
- 最后一张可用流量卡用满后，整个流量卡卡片自动消失。

真实扣费、扣费优先级、流水、有效期、购买履约和运行态数据均未修改。

## 实现

### 后端

`TrafficCreditSummary` 新增：

```json
"total_initial_usd": 10
```

`trafficPackRepository.GetSummary()` 使用同一条 SQL、同一组条件汇总：

- `SUM(initial_usd)` -> `total_initial_usd`
- `SUM(remaining_usd)` -> `total_remaining_usd`

过滤条件保持：

```sql
remaining_usd > 0 AND expires_at > now
```

因此已耗尽和已过期批次不会进入初始总额或剩余额度，保持“用满即消失”。

### 前端

`SubscriptionsView` 使用：

```ts
used = Math.max(total_initial_usd - total_remaining_usd, 0)
```

进度条的分子为 `used`，固定分母为当前仍可用批次的 `total_initial_usd`。卡片可见条件仍为 `total_remaining_usd > 0`。

## TDD 证据

### 后端 RED

先增加 `TotalInitialUSD`、多批次、耗尽移除和过期归零断言。生产代码修改前，目标测试因字段不存在而编译失败：

```text
summary.TotalInitialUSD undefined
FAIL github.com/Wei-Shaw/sub2api/internal/repository [build failed]
```

最小实现后相同测试 2/2 通过。

### 前端 RED

先增加部分消费场景。生产代码修改前，测试真实收到：

```text
Received: ...总计$0.00 / $7.00
Expected: $3.00 / $10.00
```

最小实现后订阅页测试 4/4 通过，其中包含全部耗尽后隐藏。

## 验证

以下验证均在隔离 worktree 内执行，没有启动服务或监听端口：

- `GOMAXPROCS=2 go test -p=1 -count=1 -tags=unit ./internal/repository ./internal/service`：通过。
- `pnpm exec vitest run src/views/user/__tests__/SubscriptionsView.spec.ts src/views/user/__tests__/PaymentView.spec.ts`：37/37 通过。
- `pnpm run typecheck`：通过。
- `pnpm run build`：通过，869 个模块完成构建；仅有项目既有动态导入、大 chunk 和 Browserslist 过期提示。
- `go test -count=1 -tags=embed ./internal/web`：通过。
- `go test -count=1 ./cmd/server -run '^$'`：通过。
- `git diff --check`：通过。

独立代码审查结论：无 Critical、Important 或 Minor 问题，Ready to merge。

## 提交

- `7c69b04bb fix: expose traffic credit initial total`
- `e094c2af4 fix: correct traffic credit progress display`

设计和计划提交：

- `cb03b21d0 docs: 记录流量卡进度修复设计`
- `bc3c347cb docs: 记录流量卡进度修复计划`

## 运行态范围

- 未重启、重建或替换 `sub2api-candidate`。
- 未访问运行态写接口。
- 未修改 PostgreSQL、Redis、nginx、CLIProxyAPI 或容器配置。
- 未启动开发服务器。
- 当前公网 `127.0.0.1:18084` 服务未受影响。
- 本次未部署公网。
