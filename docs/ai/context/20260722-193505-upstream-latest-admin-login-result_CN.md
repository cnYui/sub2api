# 内层 latest Sub2API 管理账号同步结果

时间：2026-07-22 19:35

## 结果

- 已将内层 latest Sub2API 本地管理账号同步为用户指定邮箱：`xiaobianfuai@gmail.com`。
- 已同步管理账号密码；文档中不记录明文密码。
- 已更新内层 latest 本地数据库用户：
  - 容器：`sub2api-upstream-postgres`
  - 用户：`users.id=1`
  - 角色：`admin`
  - 状态：`active`
- 已更新内层 latest 本地部署环境文件：
  - `D:\CodeWorkSpace\sub2api-upstream-latest\deploy\.env`

## 验证

- `POST http://127.0.0.1:18086/api/v1/auth/login` 返回 200。
- 响应包含登录数据字段；未输出、未记录 token。

## 当前可访问地址

- 控制台首页：`http://127.0.0.1:18086`
- 账号管理：`http://127.0.0.1:18086/admin/accounts`

## 边界

- 本轮只改内层 latest Sub2API 的本地运行态与本地 `.env`。
- 未触碰公网 Nginx、Cloudflare、公网数据库、公网容器。
- 未记录 GPT 凭证、内部转发 Key、JWT 或会话 token。
