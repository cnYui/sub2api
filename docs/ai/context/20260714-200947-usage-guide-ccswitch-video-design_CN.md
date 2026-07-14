# /usage-guide CCSwitch 视频教程设计

## 背景

用户提供 `/Users/wujianxiang/Downloads/112_raw.MP4`，内容为使用 CCSwitch 接入中转站的完整教程。目标是在现有登录后页面 `/usage-guide` 中增加独立视频栏目，让用户优先按视频排查连接失败和断连问题。

原视频时长约 116 秒，分辨率为 2476x1440，编码为 H.264，文件约 43.1 MB。原文件不直接进入仓库，需要先压缩为适合网页播放的版本。

## 已确认方案

- 从本地 `main` 创建分支 `codex/add-ccswitch-usage-guide-video`。
- 在现有 `/usage-guide` 中新增独立栏目「CCSwitch 视频教程」，不新增路由，也不改现有 Codex 八步图文教程。
- 使用本地静态视频，不依赖第三方视频托管。
- 视频采用 1080p H.264 MP4，保留配置界面文字可读性。
- 页面使用原生 `<video>` 控件，不自动播放，使用 `preload="metadata"`，并提供从视频提取的封面图。

## 视频处理

- 输入：`/Users/wujianxiang/Downloads/112_raw.MP4`。
- 输出：`frontend/public/usage-guide/ccswitch-relay-connection-guide.mp4`。
- 封面：`frontend/public/usage-guide/ccswitch-relay-connection-guide-poster.webp`。
- 目标编码：H.264 High Profile、YUV 4:2:0、AAC 音频、30 fps。
- 目标尺寸：最长边限制在 1920，保持原始宽高比，不放大。
- 压缩策略：CRF 27 左右、慢速预设，并启用 `faststart`，兼顾界面文字清晰度和仓库体积。
- 输出后使用 `ffprobe` 校验编码、分辨率、时长和文件大小，并人工抽查画面文字清晰度与音画同步。

## 页面结构

扩展 `UsageGuideView.vue` 的栏目联合类型，增加专用 `video` 类型，避免把视频伪装成普通文本段落或步骤图片。视频栏目包含标题、说明、视频地址和封面地址。

页面文案：

- 栏目标题：「CCSwitch 视频教程」
- 栏目说明：「完整演示使用 CCSwitch 接入中转站，解决绝大多数连接失败和断连问题。」
- 视频标题：「使用 CCSwitch 接入中转站」

视频播放器宽度跟随内容区域，设置稳定宽高比和最大宽度；移动端占满可用宽度。沿用现有浅色、深色主题，不新增装饰性卡片或自动播放行为。

## 失败与兼容处理

- 浏览器不支持视频时显示简短回退文案，并提供视频文件链接。
- `controls` 允许用户暂停、拖动、全屏和调节音量。
- `preload="metadata"` 避免进入页面后立即下载完整视频。
- MP4 使用 H.264/AAC 和 `yuv420p`，覆盖主流桌面与移动浏览器。

## 测试与验收

- 先更新 `UsageGuideView.spec.ts`，验证第六个栏目、`video` 类型、播放器属性、文案和静态资源存在。
- 运行目标 Vitest、前端类型检查和前端构建。
- 运行 `git diff --check`。
- 启动本地前端，在桌面和移动视口检查栏目导航、封面、播放器尺寸、控件和播放行为。
- 使用 `ffprobe` 确认视频为 1080p 以内的 H.264 MP4、支持渐进播放，且体积显著小于原始 43.1 MB。

## 范围边界

- 不修改后端、数据库、Redis、Nginx、容器或公网运行态。
- 不删除或替换现有截图教程。
- 不新增外部视频服务、埋点、字幕系统或自定义播放器。
- 本轮只完成本地分支实现与验证，不部署、不推送。
