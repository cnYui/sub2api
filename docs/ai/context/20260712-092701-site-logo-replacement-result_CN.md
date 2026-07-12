# 全站品牌图标替换结果

## 结果

已将用户提供的 `/Users/wujianxiang/Downloads/2273.JPG` 转换为网站新的默认品牌图标，并替换 `frontend/public/logo.png`。

最终资源规格：

- 格式：PNG，8-bit 调色板，252 色。
- 尺寸：256×256。
- 体积：37,570 字节。
- 原资源：580×560，149,928 字节。
- 体积减少：约 74.9%。
- 处理方式：保留源图 640×640 原始正方形构图，不二次裁切；使用 Lanczos 缩放、自动方向校正、元数据剥离和 256 色压缩。

## 全站引用范围

以下品牌位置继续统一使用 `/logo.png`，无需修改组件：

- `frontend/index.html`：favicon。
- `frontend/src/components/layout/AppSidebar.vue`：登录后的桌面/移动侧栏。
- `frontend/src/components/layout/AuthLayout.vue`：登录、注册及认证相关页面。
- `frontend/src/views/HomeView.vue`：默认首页。
- `frontend/src/views/KeyUsageView.vue`：公开 Key 用量页。
- `frontend/src/views/public/LegalDocumentView.vue`：公开法律文档页。

`frontend/src/App.vue` 仍保留后台 `site_logo` 非空时动态覆盖 favicon 的能力。只读检查 18084 当前公开设置为 `site_name=天才程序员小站`、`site_logo=""`，因此运行态不会用旧自定义 Logo 覆盖新默认资源。

## TDD 证据

新增 `frontend/src/__tests__/brandLogoAsset.spec.ts`，直接解析 PNG signature 和 IHDR，并约束默认 Logo 为 256×256、体积不超过 100 KiB。

RED：

- 旧图宽度实际为 580，期望 256，测试失败。
- 旧图体积实际为 149,928 字节，超过 102,400 字节，测试失败。

GREEN：

- 替换后资源测试 2/2 通过。
- ImageMagick 复核为 `PNG 256x256 252 colors 37570B`。

## 自动化验证

以下命令通过：

```bash
cd frontend
pnpm vitest run \
  src/__tests__/brandLogoAsset.spec.ts \
  src/components/layout/__tests__/AppSidebar.spec.ts \
  src/components/layout/__tests__/AuthLayout.visual.spec.ts \
  src/views/__tests__/HomeView.spec.ts \
  src/views/__tests__/KeyUsageView.spec.ts
```

结果：5 个测试文件、14 个测试全部通过。

```bash
cd frontend
pnpm typecheck
pnpm build
```

结果：类型检查退出 0；生产构建转换 869 个模块并退出 0。构建仍有项目既有的动态/静态导入与大 chunk 警告，本次未引入新的构建错误。

```bash
cd backend
go test -tags=embed ./internal/web
```

结果：通过；构建输出 `backend/internal/web/dist/logo.png` 与源资源同为 256×256、37,570 字节。

## 浏览器验证

本地前端以 `VITE_DEV_PROXY_TARGET=http://127.0.0.1:18084` 启动在 `http://127.0.0.1:5173/`。

- 首页：新图自然尺寸 256×256，实际显示约 37×37，`object-fit: contain`，favicon href 为 `/logo.png`。
- 登录页：新图自然尺寸 256×256，实际显示约 53×53，`object-fit: contain`，无拉伸或裁切。
- 响应式检查：首页没有横向溢出，头像保持正方形。
- Key 用量页与法律页的快速导航未在浏览器采样窗口内获得图片节点，因此不计作视觉通过证据；两处源码均明确使用 `siteLogo || '/logo.png'`，引用审计和相邻测试已覆盖。

## 未触碰范围

- 未部署或替换 18084 容器。
- 未修改数据库、Redis、Nginx、CLIProxyAPI 或运行态 `site_logo`。
- 未修改页面布局、品牌名称、后台自定义 Logo 能力或 API 行为。
- 未触碰工作区已有的 `backend/resources/certs/tls.crt` 用户修改。
- 未提交、推送或创建 PR。
