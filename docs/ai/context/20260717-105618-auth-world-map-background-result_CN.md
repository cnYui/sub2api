# 登录与注册页循环世界地图背景结果

## 结果

已在分支 `codex/auth-world-map-background` 完成登录与注册页循环世界地图背景。

- `LoginView`、`RegisterView` 显式开启地图背景。
- `AuthLayout` 新增默认关闭的 `worldMapBackground` 属性，其他认证页面保持原状。
- `WorldMapBackground` 使用本地点阵纹理和 CSS transform 实现 60 秒水平无缝循环，等价速度约为 30px/s。
- 亮色和深色主题下，登录与注册页外层背景都固定为 `#0a0a12`。
- 背景使用 `pointer-events: none` 和 `aria-hidden="true"`，不影响表单交互或辅助技术。
- `prefers-reduced-motion: reduce` 下停止动画，保留静态地图。

## 资源

- 新增 `frontend/src/assets/auth/world-map-dots.webp`。
- 来源为 `/Users/wujianxiang/CodeSpace/yui.web/custom.geo.json`，按参考实现的等矩形投影、10px 点距、4.5px 半径和 `#54468c` 颜色离线生成。
- 资源尺寸为 `1800×900`，通道为 `srgba`，文件大小为 8976 bytes。
- 运行时不下载、不解析 GeoJSON，也不逐帧执行陆地采样。

计划中的相对来源路径 `../../yui.web/custom.geo.json` 在隔离 worktree 下会解析到 `.worktrees/yui.web`。实际生成改用已确认的绝对参考路径；首次失败没有产生资产文件。

## TDD 证据

第一轮 RED：

- `AuthLayout.spec.ts` 失败于启用属性后未渲染地图。
- `AuthLayout.visual.spec.ts` 失败于登录和注册页未传入 `world-map-background`。

第一轮 GREEN：

- 2 个测试文件、5 个测试通过。
- 提交：`724bb392f feat: scope world map background to auth entry pages`。

第二轮 RED：

- `WorldMapBackground.visual.spec.ts` 失败于纹理文件不存在和动画契约缺失。

第二轮 GREEN：

- 3 个测试文件、7 个测试通过。
- 提交：`526779c17 feat: add looping world map auth background`。

## 工程验证

- 目标测试：4 个文件、10 个测试通过。
- `npm run typecheck`：exit code 0。
- `npm run build`：exit code 0，875 个模块完成转换，地图纹理进入构建产物；只有项目既有的动态/静态导入和 chunk 大小警告。
- 目标文件 ESLint：exit code 0。
- `git diff HEAD~2 --check`：exit code 0。
- 功能分支在归档 result 前保持 clean。

## 页面验证

本地地址：`http://127.0.0.1:4174/`。

桌面登录页：

- 只有一个认证布局、一个地图背景和一个表单卡片。
- 外层背景计算值为 `rgb(10, 10, 18)`。
- 地图纹理为 `repeat-x`，计算尺寸为 `1800px 900px`。
- 动画周期为 `60s`，300ms 内 transform 从约 `-505px` 移动到约 `-515px`。
- 轨道宽度为 3240px，等于 1440px 视口加一个 1800px 地图周期。
- 页面无水平溢出，卡片位于视口内。
- 背景 `pointer-events` 为 `none`，邮箱和密码输入可正常填写。

移动登录与注册页：

- 浏览器能力的最小实际宽度为 480px，无法直接设置到计划中的 390px。
- 在 480×844 下，两页背景均覆盖完整布局，卡片左右各保留 16px，页面无水平溢出。
- 布局使用 `w-full max-w-md` 和固定双侧 16px 内边距，窄于 480px 时卡片会继续收缩，不依赖固定卡片宽度。

范围验证：

- `/forgot-password` 的地图背景数量为 0，仍使用 `AuthLayout` 默认背景状态。
- 本地 Vite 未连接后端，公开设置请求失败，因此品牌 Logo/站点名的真实数据态未参与浏览器检查；相关颜色和条件渲染由 `AuthLayout.spec.ts` 的 mock 设置覆盖。
- in-app Browser 在当前 `devicePixelRatio=0.5` 环境中会重复拼接截图；DOM 计数确认页面没有重复节点，截图仅作视觉参考，不作为结构判定依据。

## 运行态

- 未修改数据库、Redis、容器、Nginx、Cloudflare Tunnel 或公网链路。
- 未部署、未替换候选环境。
- 主工作区原有 `backend/internal/repository/migrations_schema_integration_test.go` 未提交改动未被读取、修改或纳入功能分支。
