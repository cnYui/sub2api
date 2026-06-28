# 隐藏注册页优惠码并默认开启邀请返利计划

## 目标

- 从本地 `main` 新建分支完成改动。
- 注册页面不再展示“优惠码（可选）”输入框。
- 后端邀请返利总开关默认开启。
- 前端公共设置兜底态和后台设置表单默认态同步开启邀请返利。

## 设计

- 注册页只关闭前端展示与 URL 自动填充，不删除 `promo_codes` 后端、管理页、公开校验 API 或注册请求字段。这样历史功能仍可保留，后续需要恢复前端入口时风险最小。
- 邀请返利能力已有完整服务、表、用户页和后台页。本次只调整默认开关值：
  - 后端默认值从 `affiliate_enabled=false` 改为 `true`。
  - 初始化 settings 时写入 `affiliate_enabled=true`。
  - 新增迁移把已有 settings 里的 `affiliate_enabled` 同步更新为 `true`。
  - 前端 store/admin settings 的默认表单值同步为 `true`。
- 不修改返利比例，继续沿用当前默认 `20%`、冻结期 `0`、有效期 `0`、单人上限 `0`。

## 测试计划

- 前端：新增注册页测试，验证公开设置即使返回 `promo_code_enabled=true`，注册页也不渲染 `#promo_code` 输入框。
- 前端：补 app store 测试，验证兜底公开设置中 `affiliate_enabled=true`。
- 后端：补 setting service 测试，验证缺失设置时 `IsAffiliateEnabled()` 返回 `true`。
- 运行相关前后端定向测试。

## 风险

- 新迁移会把已有 settings 表的 `affiliate_enabled` 更新为 `true`；部署前应确认运营上允许公网邀请返利立即开启。
- 隐藏注册页 promo 输入后，用户不能手动输入注册优惠码；但后端和后台管理能力仍保留。
