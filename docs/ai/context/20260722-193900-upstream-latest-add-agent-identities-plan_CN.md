# 内层 latest Sub2API 追加 Agent Identity 计划

时间：2026-07-22 19:39

## 目标

- 将 `C:\Users\yui\Downloads\sub2api-agentIdentity-alive.json` 中的 4 个 OpenAI agent identity 账号添加到内层 latest Sub2API。
- 目标实例为本机 `http://127.0.0.1:18086`。
- 保持外层定制版 Sub2API 继续负责用户 Key、套餐、流量卡和计费。

## 执行方式

- 使用内层 latest Sub2API 正式管理导入接口：`POST /api/v1/admin/accounts/data`。
- 使用本地管理账号登录后携带 token 调用接口；不输出、不记录 token。
- 导入 payload 使用附件原始导出结构，`skip_default_group_bind=false`，让新账号绑定默认/现有 OpenAI 分组。

## 验证

- 导入接口返回创建数量与错误数量。
- 数据库确认新增账号数量、平台、类型、状态、调度状态和分组绑定。
- 用内层内部转发 Key 调用 `GET /v1/models` 和低成本 `POST /v1/responses` 验证调度仍可用。

## 安全边界

- 不回显、不记录 agent identity 具体凭证内容。
- 不改公网 Nginx、Cloudflare、公网数据库、公网容器。
- 不重启外层计费实例，除非调度缓存验证显示必须刷新。
