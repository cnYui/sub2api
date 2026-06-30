# 79/99 元套餐上游绑定本地代码结果

时间：2026-07-01 07:53 JST

## 改动

新增运行态脚本：

- `scripts/bind-codex-subscription-upstreams.mjs`

脚本行为：

- 默认 dry-run。
- 默认 PostgreSQL 容器：`sub2api-candidate-postgres`。
- 默认上游账号：`cliproxy-local-openai`。
- 默认目标 group：`codex-pool-69-usd`、`codex-pool-89-usd`。
- `--apply` 后才写入数据库。
- SQL 按名称查找账号和 group，不写死 `accounts.id`。
- 写入 `account_groups`，优先级为 `1`。
- 写入 `scheduler_outbox` 的 `group_changed` 事件，触发调度快照刷新。

新增脚本测试：

- `scripts/__tests__/bind-codex-subscription-upstreams.test.mjs`

更新 usage guide 文案：

- `frontend/src/views/user/UsageGuideView.vue`
- `frontend/src/views/user/__tests__/UsageGuideView.spec.ts`

文案从 `29/39/59/99 元套餐已支持生图和图生图` 改为 `29/39/59/79/99 元套餐已支持生图和图生图`。

## 设计保持

继续沿用现有 29/39/59 的设计：

- 通用迁移只负责 `groups` 和 `subscription_plans`。
- 运行态上游绑定通过 `account_groups` 完成。
- 不在通用迁移里写死上游账号 ID 或名称。

## TDD 记录

脚本测试红灯：

```bash
node --test scripts/__tests__/bind-codex-subscription-upstreams.test.mjs
```

结果：失败，原因是 `scripts/bind-codex-subscription-upstreams.mjs` 不存在。

前端测试红灯：

```bash
pnpm -C frontend exec vitest run src/views/user/__tests__/UsageGuideView.spec.ts
```

结果：失败，原因是页面仍缺少 `29/39/59/79/99 元套餐已支持生图和图生图`。

## 验证

已通过：

```bash
node --test scripts/__tests__/bind-codex-subscription-upstreams.test.mjs
```

结果：5 个测试通过。

已通过：

```bash
pnpm -C frontend exec vitest run src/views/user/__tests__/UsageGuideView.spec.ts
```

结果：1 个测试文件、6 个测试通过。

已通过：

```bash
node scripts/bind-codex-subscription-upstreams.mjs --dry-run
```

输出显示默认目标为 `sub2api-candidate-postgres`、`cliproxy-local-openai`、`codex-pool-69-usd` 和 `codex-pool-89-usd`，未写数据库。

已通过：

```bash
git diff --check
```

无空白错误。

## 未执行

本轮未执行：

```bash
node scripts/bind-codex-subscription-upstreams.mjs --apply
```

因此还没有修改公网 18084 运行态数据库，也没有验证 99 元用户真实请求 200。

公网执行前仍需确认 18084 已具备 `codex-pool-69-usd`；如果尚未具备，需要先发布/应用本地 main 的 156/157 迁移补齐 79 元套餐。
