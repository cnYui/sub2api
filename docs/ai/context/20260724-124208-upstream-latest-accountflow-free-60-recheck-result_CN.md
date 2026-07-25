# 内层 latest accountflow 60 个 free 账号复核结果

## 范围

- 操作对象：本地内层 latest Sub2API，`http://127.0.0.1:18086`。
- 复核账号：`accountflow-redeem-sub2.json` 导入的 `id=92..151`。
- 未触碰公网 Nginx、Cloudflare、公网容器、外层 Sub2API 用户/套餐/流量卡/计费事实。

## 复核发现

- 60 个账号均已导入成功，范围为 `id=92..151`。
- 60 个账号均为：
  - `platform=openai`
  - `type=oauth`
  - `credentials.plan_type=free`
  - `status=active`
  - 已绑定 `groups.id=2 / internal-openai-upstream`
- 复核时发现 `id=118 / AiliBamert5013@outlook.com` 处于 `schedulable=true`，与该批 free 账号不进入 `gpt-5.4` 调度池的结论不一致。

## 修正

- 修正前已备份内层 latest 的 `accounts/account_groups/proxies`：
  - `backups/20260724-124142-upstream-latest-accountflow-free-60-recheck-precorrect.sql`
- 已通过正式管理接口 `POST /api/v1/admin/accounts/bulk-update` 将 `id=92..151` 统一设为：
  - `status=active`
  - `schedulable=false`
- 批量修正结果：`success=60`、`failed=0`。

## 最终核对

- `id=92..151`：
  - 总数：60
  - `active`：60
  - `schedulable=true`：0
  - `schedulable=false`：60
  - `plan_type=free`：60
- 当前内层 OpenAI OAuth：
  - 总数：151
  - `active/schedulable`：33
  - `error / false`：118

## 结论

- 这批 free 账号导入成功。
- 由于 `gpt-5.4` 测试已确认不支持 ChatGPT account 的 Codex 模式，这批账号不应进入 `gpt-5.4` 可调度池；当前已全部收紧为不可调度。
