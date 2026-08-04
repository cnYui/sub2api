# 数据清空后重新开启注册

## 原因

- 2026-08-03 清空并重建 `18082` 业务数据时，`settings` 表也被清空。
- `registration_enabled` 不存在时，服务按安全默认值返回 `false`，因此注册页面显示“注册功能暂时关闭”。

## 修复

- 向 `sub2api-official-18082-postgres` 的 `settings` 表幂等写入 `registration_enabled=true`。
- 未修改源码默认值，也未触碰其他业务数据或既有工作区改动。

## 验证

- `GET http://127.0.0.1:18082/api/v1/settings/public` 返回 `registration_enabled=true`。
- 页面若仍显示旧状态，需要刷新浏览器以重新获取公开设置。
