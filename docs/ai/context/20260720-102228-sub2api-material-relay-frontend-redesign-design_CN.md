# Sub2API 全前端 Material Relay 重设计

- **状态**：设计已由用户批准，待审计与实施
- **批准范围**：公开页、认证页、用户端、管理端、通用组件
- **批准目标**：信息效率、品牌辨识度、交互手感同等重要
- **基线提交**：`2408ba98`

## 设计结论

采用 `C · Material Relay` 作为全前端的视觉基线：通透、流动、层级清晰。半透明材质只用于侧栏、顶栏、弹层等浮动层；表格、表单、统计和正文使用高可读实色表面，避免玻璃卡片叠加造成信息模糊。

视觉不追求单一深色或渐变风格。浅色以冷白、石墨灰和清晰蓝为主，深色使用中性黑灰，保留一套可验证的语义色阶。普通卡片圆角收敛为 `6px/8px`，浮层可以更大；禁止卡片套卡片。

## 应用壳

- 桌面端保留左侧导航与轻量顶栏，主内容保持高信息密度。
- 移动端使用抽屉导航和内容内操作区，不将桌面双栏压缩到手机宽度。
- 页面标题、筛选器和主操作形成固定层级。
- 保留现有导航数据、权限、feature flag、路由和 API 行为。

## 组件边界

1. `frontend/tailwind.config.js`：统一 `primary`、`accent`、`dark` 语义色阶，字体、阴影、圆角和动效 token。
2. `frontend/src/style.css` 与新增动效 token 文件：统一基础表面、按钮、输入框、下拉菜单、对话框、侧栏和 reduced-motion 行为。
3. `frontend/src/components/layout/AppLayout.vue`、`AppSidebar.vue`、`AppHeader.vue`：只承担页面骨架、导航和顶栏交互。
4. `frontend/src/components/layout/TablePageLayout.vue`、`frontend/src/components/common/BaseDialog.vue`、`Toast.vue`、`Input.vue`、`EmptyState.vue`：统一功能组件的视觉和反馈契约。
5. `frontend/src/views/HomeView.vue`、`frontend/src/components/layout/AuthLayout.vue`：先统一公开/认证体验。
6. 用户端和管理端页面：保留数据加载、路由、权限和错误语义，只替换页面标题区、表面层级、空状态、筛选器和局部布局。

不新增重复的 Button/Card 类组件，不重写计费、支付、用量、权限或 API 逻辑。

## 动效契约

动效只服务于反馈、状态指示和空间连续性。使用以下 vocabulary：

- `Press / Tap feedback`：按钮按下 `transform: scale(0.97)`，`160ms`。
- `Origin-aware animation`：菜单、Popover、Tooltip 从触发点展开。
- `Continuity transition`：侧栏、筛选区和状态切换保持空间连续性。
- `Stagger`：只用于偶发的首次加载或分组进入，间隔 `30–60ms`，不得阻塞交互。

统一 easing：

```css
--ease-out: cubic-bezier(0.23, 1, 0.32, 1);
--ease-in-out: cubic-bezier(0.77, 0, 0.175, 1);
--ease-drawer: cubic-bezier(0.32, 0.72, 0, 1);
```

统一时长：按钮 `160ms`，小型 Popover/Dropdown `180ms`，Drawer `280ms`，Toast 进入 `220ms`、退出 `160ms`。只动画 `transform` 和 `opacity`，不动画布局属性。路由切换、键盘命令和高频表格更新不做装饰性动画。

`prefers-reduced-motion: reduce` 下删除位移、缩放和弹性，保留短促透明度反馈；hover 动效只放在 `@media (hover: hover) and (pointer: fine)` 内。对话框继续维护焦点进入、Escape 关闭和关闭后焦点恢复，Toast 继续提供 `aria-live` 和错误引用复制。

## 实施阶段

1. 设计 token、全局基础类和应用壳。
2. 公开页与认证页。
3. 用户端 Dashboard、Key、用量、订阅、支付和个人资料。
4. 管理端 Dashboard、运维、用户、渠道、支付、设置和风险控制页面。

每个阶段都必须能独立运行和验证，业务 API 契约保持不变。

## 验收标准

- `pnpm typecheck`、`pnpm lint:check` 和 `pnpm test:run` 通过。
- Playwright 截图覆盖桌面 `1440×900`、移动 `390×844`，至少包含公开页、认证页、用户 Dashboard、管理 Dashboard。
- 慢放检查确认 Popover 来源正确、Toast 可中断、Drawer 方向一致且无布局抖动。
- `prefers-reduced-motion` 下无位移/缩放动画，仍有可理解的透明度反馈。
- `review-animations` 不得发现 `transition: all`、`ease-in` UI 进入、`scale(0)`、非 GPU 布局动画或未处理的高频动画。

## 非目标与风险

- 本轮不改变后端、数据库、支付状态机、计费来源和 API 错误契约。
- 当前页面存在大量直接写入 Tailwind 颜色和 `transition-all`；全局 token 只能覆盖共享类，页面层仍需按阶段清理。
- `visualThemeSource.spec.ts` 当前假设中性黑白主题；引入语义色阶后必须改为验证禁止的视觉反模式，而不是锁死旧颜色。
- 管理端超大单文件视图不在第一阶段拆分，避免视觉重构引入无关行为回归。
