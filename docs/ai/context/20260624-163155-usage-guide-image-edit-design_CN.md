# /usage-guide 图生图调用说明改造设计

时间：2026-06-24 16:31 JST

## 背景

- 当前普通用户「使用方法」页面 `/usage-guide` 里，「生图方法」栏目仍展示旧版纯生图调用示例：`POST /v1/images/generations`。
- 2026-06-24 已完成 Sub2API -> CLIProxyAPI 公网图生图链路调查与实测，确认真实可用入口是 `POST https://aaccx.pw/v1/images/edits`，兼容 `https://api.aaccx.pw/v1/images/edits`。
- 用户要求按现有栏目结构和语法风格，把真实图生图调用方法补进页面，并继续完成分支、提交、推送、PR、review、合并与同步流程。

## 目标

- 只改普通用户 `/usage-guide` 前端内容，让用户能直接照着页面完成真实图生图调用。
- 保持现有页面布局、栏目切换方式、卡片结构和代码块样式不变。
- 不暴露内网端口、管理员分组、后台接口或任何敏感凭证。

## 非目标

- 不改后端 API、计费、订阅、路由和鉴权逻辑。
- 不新增新的教程视图或新的页面交互。
- 不处理截图中出现但当前源码尚未实现的 `Trae 接入` 栏目。

## 设计决策

### 1. 保持 `guideTopics -> sections` 数据结构

- 继续复用 `UsageGuideView.vue` 里现有的 `guideTopics` 和 `sections` 数据驱动方式。
- 不引入新组件，不改模板分支结构，避免扩大改动面。

### 2. 把「生图方法」调整为真实图生图说明

- 栏目标题保持「生图方法」，避免影响现有导航文案。
- 栏目说明改为强调“现有 API Key + OpenAI 兼容图生图接口 + 图片额度扣费”。
- 内容结构继续保持三个 section：
  - 可用范围
  - 接口与扣费
  - 请求示例

### 3. 请求示例改为 `POST /v1/images/edits`

- 示例使用已经实测通过的公网链路：
  - `https://api.aaccx.pw/v1/images/edits`
  - 模型 `gpt-image-2`
  - `Authorization: Bearer sk-xxxx`
- 为了兼容用户最常见的两类接入方式，示例中同时说明：
  - JSON 方式：适合公网图片 URL
  - multipart 方式：适合本地文件上传
- 仍沿用现有代码块写法，不新增复杂说明格式。

### 4. 价格与安全边界保持现状

- 继续展示 1K / 2K / 4K 三档价格：
  - `$0.10 / 张`
  - `$0.20 / 张`
  - `$0.40 / 张`
- 页面仍只展示普通用户需要知道的信息，不写：
  - `groups.id`
  - `127.0.0.1:18080`
  - CLIProxyAPI 内网地址
  - 明文 API Key

## 实施范围

- `frontend/src/views/user/UsageGuideView.vue`
- `frontend/src/views/user/__tests__/UsageGuideView.spec.ts`

## 验证计划

- 运行 `frontend` 下与 `UsageGuideView` 相关的测试。
- 运行前端 `typecheck`。
- 运行前端 `build`。

## Git 流程约束

- 新分支从当前工作区状态切出，分支名使用 `codex/` 前缀。
- 仅提交本次相关文件，避免把现有无关 `deploy/*` 未跟踪文件带入提交。
- 推送目标优先使用个人 fork 远端 `personal`，不要误推上游 `origin`。
