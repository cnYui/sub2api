# 内层 latest 批量追加 agent identity 账号计划

## 背景

- 本轮附件：
  - `C:\Users\yui\Downloads\20260722-204504-sub2-API.json`
  - `C:\Users\yui\Downloads\20260722-205243-sub2-API.json`
- 两份附件均为 Sub2API 账号数据导出包：`type=sub2api-data`、`version=1`。
- 第一份包含 3 个账号，第二份包含 5 个账号，合计 8 个账号。
- 账号类型均为 `platform=openai`、`type=oauth`，无代理。
- 目标运行态仍是内层 latest Sub2API 本地实例：
  - 控制台：`http://127.0.0.1:18086`
  - OpenAI 上游分组：`groups.id=2 / internal-openai-upstream`

## 目标

- 将两份附件中的 8 个 OpenAI agent identity 账号追加到内层 latest 账号池。
- 导入后把新增账号绑定到 `internal-openai-upstream` 分组。
- 验证新增账号均为 active/schedulable，并通过管理测试接口做最小可用性测试。

## 账号范围

- `zeliatiano82007+c2api4@outlook.com`
- `zeliatiano82007+c2api5@outlook.com`
- `zillahfrontczak5665@outlook.com`
- `dagmarlemings40041+c2api2@outlook.com`
- `cyrusjelsma6913+c2api5@outlook.com`
- `cruzbobier0957@outlook.com`
- `cristianborntreger51539@outlook.com`
- `cristymarston250031+c2api3@outlook.com`

## 操作边界

- 只改内层 latest 本地运行态。
- 不修改外层定制版 Sub2API 用户、套餐、流量卡和计费事实。
- 不触碰公网 Nginx、Cloudflare、公网容器或远程数据库。
- 不记录完整 access token、refresh token、agent private key、JWT、内部转发 Key。

## 导入步骤

1. 用内层 latest 本地管理账号登录 `http://127.0.0.1:18086`，token 仅在进程内使用。
2. 导出导入前账号数据快照，保存到 `backups/20260722-215541-upstream-latest-agent-identity-preimport-data.json`。
3. 分别调用 `POST /api/v1/admin/accounts/data` 导入两份附件，参数 `skip_default_group_bind=true`。
4. 通过账号名查询新增账号 ID。
5. 调用 `POST /api/v1/admin/accounts/bulk-update`，将新增账号绑定到 `group_ids=[2]`，并确保 `status=active`、`schedulable=true`。
6. 对新增账号逐个调用 `POST /api/v1/admin/accounts/:id/test`。
7. 写入结果文档并更新 `AGENTS.md` 顶部事实。

## 回滚边界

- 若导入接口整体失败且没有创建账号：不需要回滚。
- 若部分账号创建失败：保留已创建账号，记录失败项，由人工决定是否删除。
- 如需回滚：按本轮 8 个账号名/ID 删除新增账号及其 `account_groups` 绑定，再用导入前快照核对。
