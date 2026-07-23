# 内层 latest 追加 agent identity 账号计划

## 背景

- 附件：`C:\Users\yui\Downloads\sub2api-agentIdentity-alive (1).json`。
- 文件类型：Sub2API 账号数据导出包，顶层包含 `accounts/proxies/type/version/exported_at`。
- 账号数量：5 个。
- 账号类型：`platform=openai`、`type=oauth`，包含 agent identity 凭证字段。
- 当前目标运行态：内层 latest Sub2API 本地容器：
  - `sub2api-upstream-latest`，入口 `http://127.0.0.1:18086`
  - `sub2api-upstream-postgres`
  - `sub2api-upstream-redis`
- 当前内层 OpenAI 分组：`groups.id=2 / internal-openai-upstream`。
- 当前内层已有 5 个 OpenAI OAuth 账号，均 active/schedulable 且绑定分组 2。

## 目标

- 将附件中的 5 个 OpenAI agent identity 账号追加到内层 latest 账号池。
- 导入后绑定到 `internal-openai-upstream` 分组。
- 验证新账号 active/schedulable，并用管理测试接口做最小可用性验证。
- 不打印、不写入文档完整 access token、refresh token、agent private key、内部转发 Key、JWT。

## 操作边界

- 只改内层 latest 本地运行态。
- 不修改外层定制版 Sub2API 用户、计费、套餐、流量卡事实。
- 不触碰公网 Nginx、Cloudflare、公网容器或远程数据库。
- 使用内层管理 API 优先；只读 SQL 用于前后核对。

## 备份

- 备份表：
  - `accounts`
  - `account_groups`
  - `proxies`
- 备份文件：`backups/20260722-194856-upstream-latest-agent-identity-preimport.sql`
- 备份后验证文件存在、非空、可读。

## 导入步骤

1. 读取内层 latest 管理账号登录信息到进程内存，不输出明文密码或 token。
2. 调用 `POST http://127.0.0.1:18086/api/v1/auth/login` 获取临时管理 token。
3. 调用 `POST /api/v1/admin/accounts/data`：
   - payload 使用附件原始 JSON。
   - `skip_default_group_bind=true`，与前端导入行为一致。
4. 根据附件账号 name 查询新建账号 ID。
5. 调用正式批量更新接口，将新账号绑定到 `group_ids=[2]`，同时保持 active/schedulable。
6. 用只读 SQL 核对账号总数、分组绑定、状态。
7. 对新账号执行管理测试接口；如个别失败，只记录账号 ID 与错误摘要，不删除账号。

## 回滚边界

- 如果导入接口整体失败且未创建账号：不需要回滚。
- 如果部分创建失败：保留已创建账号，记录失败项，等待人工决定是否删除。
- 如果需要回滚：按本次导入的 5 个账号 name/ID 删除或软删除新账号，并清理对应 `account_groups`，再根据备份核对。

## 验证标准

- 导入接口返回 `account_created=5`、`account_failed=0`。
- 内层 OpenAI OAuth 账号从 5 个变为 10 个。
- 新增 5 个账号均：
  - `platform=openai`
  - `type=oauth`
  - `status=active`
  - `schedulable=true`
  - 绑定 `group_id=2`
- 管理测试接口对新增账号返回 200，或至少能定位失败账号和原因。
