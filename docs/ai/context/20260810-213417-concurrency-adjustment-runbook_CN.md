# 并发调整操作手册

> 目标：解除当前并发瓶颈（上游账号默认并发 3），支撑 60 人两天连续使用、峰值约 300 并发。
> 关联分析：见 `20260810-211500-concurrency-capacity-analysis_CN.md`。

## 前置说明

- **瓶颈根因**：`backend/ent/schema/account.go` 中 `field.Int("concurrency").Default(3)`，上游账号并发默认仅 3。所有用户请求路由到同一上游账号时，在途仅 3，其余全部排队，吞吐被压到极低，同时把 P99 拉高、拖垮健康分。
- **两道并发闸门**：用户闸门（user gate，容量 = 用户并发 + 20 排队位）+ 账号闸门（account gate，30s 超时、无排队上限）。本手册三步分别调整这两道闸门。
- **热生效**：并发值在取槽时实时读取，三步操作**均无需重启服务**。
- **执行环境**：以下均为生产运行时数据库变更，需在管理后台 UI 或携带管理员 Token 的已认证 API 执行；本仓库代码环境无法直接执行。

---

## 操作 1：上游账号并发 3 → 100（最高优先级）

这是解除瓶颈的关键一步，建议**最先执行**。

### 接口

`POST /api/v1/admin/accounts/bulk-update`

### 参数（payload）

```json
{
  "account_ids": [<账号ID列表>],
  "concurrency": 100
}
```

- `account_ids`：目标账号 ID 数组。若省略则必须传 `filters`（二者至少其一，否则报 `account_ids or filters is required`）。
- `concurrency`：`*int`，本次要设置的并发值，填 `100`。
- 也可用 `filters` 批量筛选账号；只想改并发时，其余字段（`name`/`priority`/`rate_multiplier` 等）全部留空即可。

### UI 操作路径

账号管理页 → 全选上游账号 → 批量编辑 → 并发填 `100` → 保存。

### 验证方法

1. 重新拉取账号列表，确认目标账号 `concurrency` 均已变为 100。
2. 观察 `/admin/ops` 运维监控页：**请求时长 P99 应明显下降**（排队等待消失）、**异常数下降**。若 P99 骤降即证明瓶颈就是原来的 `3`。
3. 若峰值 300 时仍有排队/超时（账号闸门 30s 超时抛 ConcurrencyError），继续把并发上调至 120~300。

### 回滚方法

再次调用同接口，将 `concurrency` 改回原值：

```json
{ "account_ids": [<相同账号ID列表>], "concurrency": 3 }
```

热生效，无需重启。

---

## 操作 2：当前所有用户并发 → 20

调整用户闸门。用户闸门容量 = 用户并发 + 20 排队位，设为 20 即每用户在途 20、排队上限 20。

### 接口

`POST /api/v1/admin/users/batch-concurrency`

### 参数（payload）

```json
{
  "all": true,
  "mode": "set",
  "concurrency": 20
}
```

- `all`：`true` 表示对全部用户生效，后端自动分页（每页 500）逐批处理，无需手动传 ID。
- `mode`：必填，`set`（覆盖为该值）或 `add`（在现值上增加）。此处用 `set`。
- `concurrency`：目标并发值 `20`。
- 如需只改部分用户：`all` 传 `false` 或省略，改传 `user_ids`（数组，单次上限 500）。

### UI 操作路径

用户管理页 → 批量操作 → 修改并发 → 设为 `20` → 选择「全部」→ 执行。

### 验证方法

1. 抽查若干用户，确认其并发额度已为 20。
2. 返回体 `affected` 字段应等于当前用户总数。

### 回滚方法

同接口重新执行，把 `concurrency` 设回原值（例如原默认 20 则本步实际无变化；若之前存在个别自定义值，`set` 会统一覆盖，回滚需按原始分布逐批还原）：

```json
{ "all": true, "mode": "set", "concurrency": <原值> }
```

> 提示：`set` 为破坏性覆盖，执行前若担心个别用户有自定义并发，建议先导出/记录当前用户并发分布，便于精确回滚。

---

## 操作 3：新用户默认并发 → 20（`default_concurrency`）

只影响**未来新注册**用户，注册时按此值发放并发额度（`auth_service.go` 中 `plan.Concurrency = GetDefaultConcurrency(ctx)`）。**不影响已有用户，也与上游账号并发无关。**

### 接口

- 读取：`GET /api/v1/admin/settings`
- 更新：`PUT /api/v1/admin/settings`

### 参数（payload）

`PUT` 提交完整设置对象，其中字段：

```json
{
  "default_concurrency": 20
  // ... 其余设置字段照原样回传
}
```

- `default_concurrency`：`int`。后端校验 `< 1` 会被强制归一为 1，填 `20` 合法。
- 建议先 `GET` 取回完整设置对象，仅改 `default_concurrency` 一项后整体 `PUT` 回去，避免遗漏其它字段。

### UI 操作路径

后台系统设置 → 「默认并发量」（`default_concurrency`）→ 设为 `20` → 保存。

### 验证方法

1. `GET /api/v1/admin/settings` 确认 `default_concurrency` 为 20。
2. 注册一个测试新用户，确认其并发额度为 20（老用户不受影响，可对比确认）。

### 回滚方法

同接口把 `default_concurrency` 改回原值（默认回退到配置 `cfg.Default.UserConcurrency`）：

```json
{ "default_concurrency": <原值> }
```

---

## 执行顺序与整体校验

1. 先做**操作 1**（账号并发 3→100），立即观察 `/admin/ops` 的 P99 与异常数变化——这是验证瓶颈假设的核心信号。
2. 再做**操作 2**（现有用户→20）与**操作 3**（默认→20），保证用户闸门与新用户默认一致。
3. 峰值压力（约 300 并发）到来时，若账号闸门仍出现超时，按操作 1 的回滚接口反向操作、把账号并发继续上调（120~300）。

所有变更热生效，无需重启服务；回滚均通过重新调用对应接口、填回原值完成。
