# 18084 候选环境新建用户结果

## 背景

用户要求在当前最新前端页面对应的 `18084` 候选环境中创建 `1038686518@qq.com`，并使用用户指定的默认密码，使该账号可以直接通过登录页登录。

## 目标环境

- 前端/后端候选入口：`http://127.0.0.1:18084`
- 应用容器：`sub2api-candidate`
- 数据库容器：`sub2api-candidate-postgres`
- 数据库：`sub2api`

## 执行内容

- 已确认 `sub2api-candidate` 映射 `127.0.0.1:18084 -> 8080/tcp`。
- 已确认 `sub2api-candidate` 连接 `sub2api-candidate-postgres`。
- 已确认创建前 `1038686518@qq.com` 在候选库中不存在。
- 已使用与后端兼容的 bcrypt 哈希写入 `users.password_hash`。
- 已在一个事务中插入：
  - `users` 普通用户记录
  - `auth_identities` 的 email 登录身份记录

## 结果

- 新用户 ID：`48`
- Email：`1038686518@qq.com`
- Username：`1038686518@qq.com`
- Role：`user`
- Status：`active`
- Signup source：`email`
- Deleted at：空
- 候选库 active 用户数：从 `44` 增加到 `45`
- Email 身份记录：
  - `provider_type=email`
  - `provider_key=email`
  - `provider_subject=1038686518@qq.com`
  - `metadata.source=manual_candidate_18084_create_user`

## 验证

- 数据库查询确认 `users` 和 `auth_identities` 记录存在。
- 通过 `POST http://127.0.0.1:18084/api/v1/auth/login` 验证登录成功：
  - HTTP 状态：`200`
  - 响应 `code=0`
  - 响应 `message=success`
  - 返回用户：`id=48`、`email=1038686518@qq.com`、`role=user`、`status=active`
  - 响应包含 `access_token` 和 `refresh_token`

## 注意

- 本次只修改 `sub2api-candidate-postgres` 候选数据库。
- 未修改生产库 `sub2api-postgres`。
- 未创建 API Key。
- 未绑定套餐。
- 未调整余额。
- 未修改支付配置。
- 文档未记录 token、数据库运行密钥或密码哈希。
