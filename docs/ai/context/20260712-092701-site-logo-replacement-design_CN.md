# 全站品牌图标替换设计

## 背景

当前网站在侧栏“天才程序员小站”前、认证页、首页、API Key 用量页、法律文档页和浏览器 favicon 展示同一品牌图标。用户要求用 `/Users/wujianxiang/Downloads/2273.JPG` 的头像替换现有蓝色 Sub2API 图标，并自动完成尺寸适配、缩放与压缩。

源图为 640×640 JPEG，体积 87,042 字节。现有 `frontend/public/logo.png` 为非正方形 PNG，体积 149,928 字节。页面品牌位最大为 56×56 CSS 像素，其他位置主要为 36×36、40×40 和 favicon。

## 引用审计

品牌图标最终由以下位置使用：

- `frontend/index.html`：默认 favicon 使用 `/logo.png`。
- `frontend/src/components/layout/AppSidebar.vue`：桌面与移动侧栏品牌图标。
- `frontend/src/components/layout/AuthLayout.vue`：登录、注册、验证码和第三方登录相关页面。
- `frontend/src/views/HomeView.vue`：默认首页导航图标。
- `frontend/src/views/KeyUsageView.vue`：公开 API Key 用量页导航图标。
- `frontend/src/views/public/LegalDocumentView.vue`：公开法律文档页导航图标。
- `frontend/src/App.vue`：后台配置 `site_logo` 非空时动态覆盖 favicon。

上述 Vue 页面都采用 `siteLogo || '/logo.png'`。运行态 18084 注入配置当前为 `site_logo=""`，因此实际展示由 `/logo.png` 决定；后台自定义 Logo 能力继续保留。

## 方案比较

### 方案 A：替换为 256×256 压缩 PNG，推荐

保持现有 `/logo.png` 契约，只替换单个静态资源。源图本身已经是正方形，不额外裁切人物；缩放到 256×256 足以覆盖 56px 的高分屏展示，并通过调色板压缩和元数据剥离降低体积。

优点：单资源覆盖全站，不改组件，不改变后台自定义 Logo 逻辑，兼容 favicon 和现有后端静态文件测试。

### 方案 B：使用 512×512 PNG

保留更多像素，但当前最大展示尺寸不需要 512px，文件体积更高，下载收益较差。

### 方案 C：切换为 WebP 并修改所有引用

文件可能更小，但需要修改 favicon、组件 fallback、静态资源测试和 Content-Type 契约。对一个小尺寸品牌头像属于不必要的扩散改动。

## 最终设计

- 从源 JPEG 生成 256×256 PNG。
- 不改变原始正方形构图，不做人物检测或二次裁脸。
- 使用高质量 Lanczos 缩放、去除 EXIF/ICC 等非必要元数据，并压缩为调色板 PNG。
- 目标文件保持正方形、PNG 格式，体积不超过 100 KiB。
- 继续使用唯一资源路径 `/logo.png`，不修改页面组件和后台 `site_logo` 覆盖能力。
- 不修改数据库、Redis、Nginx、容器或运行态设置，不部署 18084。

## 测试策略

1. 新增资源契约测试，读取 PNG IHDR，要求 `frontend/public/logo.png` 为 256×256 且不超过 100 KiB。
2. 先运行测试，确认旧图因尺寸和体积不符合而失败。
3. 生成新图后重跑测试，确认资源契约通过。
4. 搜索全部 `siteLogo`、`site_logo` 和 `/logo.png` 引用，确认所有品牌位置仍统一走该资源，且没有遗漏的旧品牌图片文件。
5. 运行前端目标测试、类型检查、生产构建和后端嵌入静态资源测试。
6. 启动本地前端，通过浏览器检查侧栏、认证页、首页及 favicon，并覆盖桌面和移动视口。

## 工作区约束

- 当前分支为 `main`。
- 实施前已有用户修改 `backend/resources/certs/tls.crt`，本次不得覆盖、回退或提交该文件。
- 用户已明确授权自动决定裁切、尺寸和压缩策略。
