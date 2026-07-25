# 内层 latest duplicate sub2api-account 检查结果

## 范围

- 操作对象：本地内层 latest Sub2API，`http://127.0.0.1:18086`。
- 待导入文件：`C:/Users/yui/Downloads/sub2api-account-20260724231026.json`。
- 对比文件：`C:/Users/yui/Downloads/sub2api-account-20260724131644.json`。
- 未触碰公网 Nginx、Cloudflare、公网容器、外层 Sub2API 用户/套餐/流量卡/计费事实。

## 检查结果

- 待导入文件账号数：10。
- 账号均为：
  - `platform=openai`
  - `type=oauth`
  - `plan_type=free`
- 待导入文件与已导入文件对比：
  - 10 个邮箱完全一致。
  - refresh token 完全一致。
  - access token 完全一致。
- 内层 latest DB 已存在同名/同邮箱账号 10 个：
  - `id=185..194`
  - 均为 `status=active`
  - 均为 `schedulable=true`

## 处理结论

- 本轮未重复导入，避免创建同名重复账号。
- 本轮未跑模型测试。
- 内层 latest OpenAI OAuth 账号总数保持 194。
- 当前 `active/schedulable` 保持 136。

