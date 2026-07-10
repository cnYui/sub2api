# caogang@sdufe.edu.cn 后台查找与 99 元套餐直加计划

## 背景

- 用户要求检查后台 `https://aaccx.pw/admin/subscriptions` 为什么查不到 `caogang@sdufe.edu.cn`。
- 运行态只读查询已确认 `sub2api-candidate-postgres` 中存在该用户：`users.id=89`、邮箱 `caogang@sdufe.edu.cn`、状态 `active`。
- 若确认用户存在，需要绕过支付流程，直接给该用户添加 99 元套餐。

## 必须事实

- 本次只能操作当前运行态候选库：容器 `sub2api-candidate-postgres`，数据库 `sub2api`，用户 `sub2api`。
- 写库前必须备份 Postgres，并验证备份可读。
- 99 元套餐应复用现有套餐/分组配置，不手写价格或额度含义，避免和产品配置漂移。
- 不能通过支付创建订单；应直接新增或修正 `user_subscriptions` 记录，并保留审计上下文。

## 执行计划

1. 用内置浏览器登录后台，复现 `/admin/subscriptions` 搜索该邮箱的行为。
2. 数据库只读确认用户、现有订阅、套餐表、分组和 99 元套餐对应关系。
3. 写库前导出 `sub2api-candidate-postgres` 备份到 `deploy/backups/`，并用 `pg_restore -l` 验证。
4. 若用户无 active 订阅，则按现有后台/迁移模式插入 99 元套餐订阅；若已有 active 订阅，先停下来复核，避免双 active。
5. 写库后复查 `users`、`user_subscriptions` 与后台页面结果，并记录原因与结果。

## 风险与约束

- 若后台查不到是搜索 API 未按用户邮箱联表搜索，数据库直加能解决订阅状态，但不会自动修复后台搜索逻辑。
- 若该用户已有 active 订阅，不直接叠加第二个 active 订阅。
- 本次不构建镜像、不重启容器、不修改支付订单。
