# 全站品牌图标替换实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**目标：** 将网站所有默认品牌位置统一替换为用户提供的头像图标，并生成适合小尺寸展示的压缩资源。

**架构：** 保持现有 `/logo.png` 单资源契约，所有页面继续通过 `siteLogo || '/logo.png'` 共享默认品牌资源。只替换静态图片并增加资源约束测试，不修改组件结构或运行态设置。

**技术栈：** Vue 3、TypeScript、Vitest、ImageMagick、Vite、Go embedded frontend。

---

### 任务 1：RED - 固化默认品牌资源约束

**文件：**
- 新建：`frontend/src/__tests__/brandLogoAsset.spec.ts`

- [ ] 读取 `frontend/public/logo.png` 的二进制内容，验证 PNG signature。
- [ ] 从 IHDR 读取宽高，要求均为 256。
- [ ] 断言文件体积不超过 `100 * 1024` 字节。
- [ ] 运行测试并确认旧资源因实际尺寸和体积不符合而失败：

```bash
cd frontend
pnpm vitest run src/__tests__/brandLogoAsset.spec.ts
```

### 任务 2：GREEN - 生成压缩头像资源

**文件：**
- 修改：`frontend/public/logo.png`

- [ ] 使用 ImageMagick 从源 JPEG 生成临时 PNG：

```bash
magick /Users/wujianxiang/Downloads/2273.JPG \
  -auto-orient \
  -resize 256x256! \
  -filter Lanczos \
  -strip \
  -colors 256 \
  PNG8:/tmp/sub2api-logo.png
```

- [ ] 用 `apply_patch` 删除旧 `frontend/public/logo.png`，再通过 `apply_patch` 可支持的二进制替换流程写入新资源；不得使用 shell 重定向直接编辑仓库文件。
- [ ] 验证生成文件为 256×256 PNG、体积不超过 100 KiB，并目视检查人物构图没有变形或明显色带。
- [ ] 重跑资源约束测试，确认通过。

### 任务 3：统一引用审计

**文件：**
- 不修改生产代码。

- [ ] 搜索 `siteLogo`、`site_logo`、`/logo.png` 和仓库中的 PNG/JPEG/WebP/SVG 品牌资源。
- [ ] 确认侧栏、认证页、首页、Key 用量页、法律文档页和 favicon 全部使用同一默认资源。
- [ ] 确认 18084 当前 `site_logo` 为空，不会覆盖新默认图标。

### 任务 4：自动化验证

**文件：**
- 不新增生产文件。

- [ ] 运行资源测试和相邻视觉源码测试：

```bash
cd frontend
pnpm vitest run \
  src/__tests__/brandLogoAsset.spec.ts \
  src/components/layout/__tests__/AppSidebar.spec.ts \
  src/components/layout/__tests__/AuthLayout.visual.spec.ts \
  src/views/__tests__/HomeView.spec.ts \
  src/views/__tests__/KeyUsageView.spec.ts
```

- [ ] 运行前端类型检查与生产构建：

```bash
cd frontend
pnpm typecheck
pnpm build
```

- [ ] 运行后端嵌入静态资源测试：

```bash
cd backend
go test ./internal/web
```

- [ ] 回到仓库根目录运行 `git diff --check`。

### 任务 5：浏览器视觉验证

**文件：**
- 不修改生产文件。

- [ ] 启动前端开发服务器，使用未被占用的本地端口。
- [ ] 在桌面视口检查首页、登录页和登录后侧栏，确认头像清晰、比例正确、无裁切或布局偏移。
- [ ] 在移动视口检查相同品牌位置，确认 36px/40px/56px 展示不模糊且不溢出。
- [ ] 检查浏览器 favicon 已使用新头像。

### 任务 6：长期上下文与结果记录

**文件：**
- 修改：`AGENTS.md`
- 新建：`docs/ai/context/20260712-092701-site-logo-replacement-result_CN.md`

- [ ] 在 `AGENTS.md` 的“最高优先级定论”顶部增加本次资源规格、引用范围、验证结果和未部署说明。
- [ ] 结果文档记录 RED/GREEN 证据、最终尺寸/体积、全站引用清单、测试命令和未触碰范围。
- [ ] 运行 `git status --short` 与 `git ls-files --others --exclude-standard docs/ai/context`，确认历史文件未被覆盖，用户已有证书改动未被触碰。
