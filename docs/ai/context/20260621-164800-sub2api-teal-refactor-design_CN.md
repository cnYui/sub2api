# Sub2API 前端 Teal/Slate 重构设计

## 目标

将 upstream origin/main 的 Teal/Slate 主题引入当前本地分支，同时保留本地已有的合理定制（订阅页默认 tab、套餐卡片文案），形成统一的公网前端。

## 当前状态

| 分支 | HEAD | 主题 |
|---|---|---|
| 本地 main | c1ef7c2a | 灰黑白（gray primary, Playfair+Inter） |
| origin/main | 78f5c445 | Teal/Slate（teal primary, slate accent, Inter） |

**本地 main 领先 origin/main 的定制（26 commits ahead）：**
- 首页套餐卡片文案精简（¥29元/月，日限额 x刀，24点刷新）
- 订阅页默认展示订阅 tab（左侧订阅、右侧充值）
- nginx 公网路由配置已包含 `/purchase` 等白名单

**origin/main 带来的变更（26 commits）：**
- `tailwind.config.js`：primary 从 gray 改为 teal-50..teal-950，accent 改为 slate-900，dark 改为 slate
- `HomeView.vue`：新增 teal 渐变背景装饰（模糊圆圈 + 网格线）
- `UsageGuideView.vue`：已删除（534 行，含路由移除）
- `SettingsView.vue`（admin）：新增 657 行 Claude OAuth System Prompt Injection 设置项
- `Admin DashboardView.vue`：69 行变更（移除模型分布卡片）
- `vite.config.ts`：10 行变更（chunk 策略调整）
- 大量其他视图、i18n、测试文件的更新

## 冲突分析

### tailwind.config.js
本地：gray primary + gold accent + Playfair Display
上游：teal primary + slate accent + Inter only

**策略**：采用上游 teal/slate，不保留本地 gray。brand 配色（DEFAULT_SITE_NAME="天才程序员小站"）保留，但颜色体系切换为 teal。

### HomeView.vue
本地：有首页定制（套餐卡片展示）
上游：有 teal 渐变背景

**策略**：以 upstream teal 渐变装饰为基础，合并本地 hero 区「立即登录」按钮文案。teal 装饰背景 + 本地套餐卡片展示共存。

### SettingsView.vue
本地：原版（c3e0f2b9）
上游：在第 3841 行后追加 Claude OAuth System Prompt Injection 配置（657 行）

**策略**：采用上游追加版本（`++`），Claude OAuth 配置是功能增量，无冲突。

### UsageGuideView.vue
本地：存在（534 行）
上游：已删除

**策略**：采用上游（删除）。使用指南功能已通过其他方式（如 Dashboard 移除模型卡片，admin 新增使用教程入口）覆盖。

### router/index.ts
本地：有 `/usage-guide` 路由
上游：已移除 `/usage-guide` 路由

**策略**：采用上游（移除路由）。

## 执行计划

### 1. 创建工作分支
```bash
git checkout -b codex/teal-refactor-$(date +%Y%m%d)
```

### 2. 合并 origin/main
```bash
git fetch origin
git merge origin/main --no-ff -m "merge: 引入 origin/main teal/slate 主题及上游功能"
```

### 3. 冲突处理规则
| 文件 | 策略 |
|---|---|
| `tailwind.config.js` | 采用 upstream（teal/slate） |
| `frontend/src/views/HomeView.vue` | 采用 upstream（teal 装饰），手工合并本地 hero 文案 |
| `frontend/src/views/user/SettingsView.vue` | 采用 upstream（Claude OAuth 新增部分） |
| `frontend/src/views/user/UsageGuideView.vue` | 采用 upstream（删除） |
| `frontend/src/router/index.ts` | 采用 upstream（移除 usage-guide 路由） |
| 其他文件 | 优先 upstream，后端逻辑以 upstream 为准 |

### 4. 验证
- `cd frontend && pnpm vitest run`
- `cd frontend && pnpm build`
- 启动 `pnpm --dir frontend run dev` 视觉验收

### 5. 发布公网
- 构建新 Docker 镜像
- 只重建 sub2api app 容器
- 验证 https://aaccx.pw/purchase、https://api.aaccx.pw/v1/models

## 不执行事项

- 不改 nginx、Cloudflare Tunnel、数据库、Redis
- 不在 git 历史中记录完整 API Key、密码、token
- 不修改后端 API 路由、计费逻辑、支付接口
