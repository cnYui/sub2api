# /usage-guide CCSwitch 视频教程实施计划

> **执行要求：** 使用 `executing-plans` 在当前会话逐项执行；实现阶段使用 `test-driven-development`，完成前使用 `verification-before-completion`。

**目标：** 压缩用户提供的 CCSwitch 教程视频，在 `/usage-guide` 新增独立视频栏目，并在验证通过后合并到本地 `main`、推送到 `personal/main`。

**架构：** 视频和封面作为 `frontend/public/usage-guide/` 下的稳定静态资源，由现有 Vite 构建复制到产物。`UsageGuideView.vue` 的栏目联合类型新增 `video` 分支，模板使用原生播放器，不引入播放器依赖或新路由。

**技术栈：** Vue 3、TypeScript、Vitest、Vite、FFmpeg/ffprobe、浏览器视觉检查、Git。

---

## 文件职责

- 新建 `frontend/public/usage-guide/ccswitch-relay-connection-guide.mp4`：压缩后的网页视频。
- 新建 `frontend/public/usage-guide/ccswitch-relay-connection-guide-poster.webp`：播放器加载前封面。
- 修改 `frontend/src/views/user/UsageGuideView.vue`：新增视频栏目类型、内容和响应式播放器。
- 修改 `frontend/src/views/user/__tests__/UsageGuideView.spec.ts`：验证栏目契约、播放器属性和静态资源。
- 新建 `docs/ai/context/20260714-201226-usage-guide-ccswitch-video-result_CN.md`：记录压缩结果、验证结果和集成状态。
- 修改 `AGENTS.md`：追加最终长期记忆，不修改既有历史条目。

### 任务 1：先建立失败的页面与资源契约测试

**文件：**

- 修改：`frontend/src/views/user/__tests__/UsageGuideView.spec.ts`

- [ ] **步骤 1：新增公开资源目录定位**

```ts
const publicUsageGuideDir = resolve(currentDir, '../../../../public/usage-guide')
```

- [ ] **步骤 2：把栏目数量和视频栏目写入现有栏目测试**

将栏目测试改为六个栏目，并增加：

```ts
expect(source).toContain("id: 'ccswitch-video'")
expect(source).toContain("title: 'CCSwitch 视频教程'")
```

- [ ] **步骤 3：新增视频播放器契约测试**

```ts
it('CCSwitch 视频教程使用本地视频、封面和按需加载播放器', () => {
  const source = readFileSync(viewPath, 'utf8')
  const expectedTokens = [
    "kind: 'video'",
    'data-test="usage-guide-video"',
    'controls',
    'playsinline',
    'preload="metadata"',
    'ccswitch-relay-connection-guide.mp4',
    'ccswitch-relay-connection-guide-poster.webp',
    '使用 CCSwitch 接入中转站',
    '解决 99% 常见的连接不上、断连问题',
  ]

  for (const token of expectedTokens) {
    expect(source, `缺少 CCSwitch 视频教程信息：${token}`).toContain(token)
  }
  expect(source).not.toContain('autoplay')
})
```

- [ ] **步骤 4：新增静态资源存在测试**

```ts
it('CCSwitch 视频和封面资源都存在', () => {
  expect(existsSync(resolve(publicUsageGuideDir, 'ccswitch-relay-connection-guide.mp4'))).toBe(true)
  expect(existsSync(resolve(publicUsageGuideDir, 'ccswitch-relay-connection-guide-poster.webp'))).toBe(true)
})
```

- [ ] **步骤 5：运行目标测试确认失败**

```bash
pnpm -C frontend test:run src/views/user/__tests__/UsageGuideView.spec.ts
```

预期：新增测试因视频栏目和两个静态资源尚不存在而失败；原有测试继续通过。

### 任务 2：压缩视频并生成封面

**文件：**

- 新建：`frontend/public/usage-guide/ccswitch-relay-connection-guide.mp4`
- 新建：`frontend/public/usage-guide/ccswitch-relay-connection-guide-poster.webp`

- [ ] **步骤 1：创建静态资源目录**

```bash
mkdir -p frontend/public/usage-guide
```

- [ ] **步骤 2：生成 1080p H.264 网页视频**

```bash
ffmpeg -y -i /Users/wujianxiang/Downloads/112_raw.MP4 \
  -map 0:v:0 -map 0:a? \
  -vf "scale='min(1920,iw)':-2:flags=lanczos,fps=30" \
  -c:v libx264 -preset slow -crf 27 -profile:v high -pix_fmt yuv420p \
  -movflags +faststart -c:a aac -b:a 96k -ac 2 \
  frontend/public/usage-guide/ccswitch-relay-connection-guide.mp4
```

预期：输出分辨率不超过 1920 宽，时长约 116 秒，文件显著小于原始 43.1 MB。

- [ ] **步骤 3：生成 WebP 封面**

```bash
ffmpeg -y -ss 00:00:03 \
  -i frontend/public/usage-guide/ccswitch-relay-connection-guide.mp4 \
  -frames:v 1 -vf "scale=1280:-2:flags=lanczos" \
  -c:v libwebp -quality 82 \
  frontend/public/usage-guide/ccswitch-relay-connection-guide-poster.webp
```

- [ ] **步骤 4：检查媒体规格和文件大小**

```bash
ffprobe -v error \
  -show_entries format=duration,size,bit_rate:stream=codec_name,codec_type,width,height,pix_fmt,r_frame_rate \
  -of json frontend/public/usage-guide/ccswitch-relay-connection-guide.mp4
du -h /Users/wujianxiang/Downloads/112_raw.MP4 frontend/public/usage-guide/ccswitch-relay-connection-guide.mp4 frontend/public/usage-guide/ccswitch-relay-connection-guide-poster.webp
```

预期：视频为 H.264、音频为 AAC、像素格式为 `yuv420p`、帧率 30 fps、宽度 1920、高度按比例计算。

### 任务 3：实现视频栏目

**文件：**

- 修改：`frontend/src/views/user/UsageGuideView.vue`
- 测试：`frontend/src/views/user/__tests__/UsageGuideView.spec.ts`

- [ ] **步骤 1：增加视频栏目类型**

```ts
| {
    id: string
    title: string
    description: string
    kind: 'video'
    video: { title: string; src: string; poster: string }
  }
```

- [ ] **步骤 2：新增 CCSwitch 视频栏目数据**

在 `guideTopics` 的 Codex 栏目后加入：

```ts
{
  id: 'ccswitch-video',
  title: 'CCSwitch 视频教程',
  description: '完整演示使用 CCSwitch 接入中转站，解决 99% 常见的连接不上、断连问题。',
  kind: 'video',
  video: {
    title: '使用 CCSwitch 接入中转站',
    src: '/usage-guide/ccswitch-relay-connection-guide.mp4',
    poster: '/usage-guide/ccswitch-relay-connection-guide-poster.webp',
  },
},
```

- [ ] **步骤 3：增加播放器模板分支**

```vue
<section v-else-if="activeTopic.kind === 'video'" class="usage-guide-video-section">
  <h3 class="usage-guide-video-title">{{ activeTopic.video.title }}</h3>
  <video
    data-test="usage-guide-video"
    class="usage-guide-video"
    :src="activeTopic.video.src"
    :poster="activeTopic.video.poster"
    controls
    playsinline
    preload="metadata"
  >
    当前浏览器无法播放视频，请
    <a :href="activeTopic.video.src">打开视频文件</a>。
  </video>
</section>
```

原来的普通栏目分支改为 `v-else`。

- [ ] **步骤 4：增加稳定响应式样式**

```css
.usage-guide-video-section { width: 100%; max-width: 960px; }
.usage-guide-video-title { margin: 0 0 14px; color: #111827; font-size: 18px; font-weight: 700; }
.dark .usage-guide-video-title { color: #f8fafc; }
.usage-guide-video {
  display: block;
  width: 100%;
  aspect-ratio: 16 / 9;
  border-radius: 8px;
  background: #000;
  object-fit: contain;
}
```

- [ ] **步骤 5：运行目标测试确认通过**

```bash
pnpm -C frontend test:run src/views/user/__tests__/UsageGuideView.spec.ts
```

预期：`UsageGuideView.spec.ts` 全部通过。

- [ ] **步骤 6：提交功能改动**

只暂存视频、封面、页面和目标测试，提交信息为：

```bash
git commit -m "feat: add CCSwitch usage guide video"
```

### 任务 4：完整验证和结果记录

**文件：**

- 新建：`docs/ai/context/20260714-201226-usage-guide-ccswitch-video-result_CN.md`
- 修改：`AGENTS.md`

- [ ] **步骤 1：运行前端验证**

```bash
pnpm -C frontend test:run src/views/user/__tests__/UsageGuideView.spec.ts
pnpm -C frontend typecheck
pnpm -C frontend build
git diff --check
```

预期：全部通过。

- [ ] **步骤 2：验证后端嵌入构建**

```bash
go test -C backend -count=1 -tags=embed ./internal/web
```

预期：嵌入前端资源的 Go 测试通过。

- [ ] **步骤 3：启动前端并做桌面、移动端视觉检查**

```bash
pnpm -C frontend dev --host 127.0.0.1
```

检查第六栏目、封面、非自动播放、播放/暂停/拖动/全屏，以及桌面和移动视口无溢出、重叠或文字截断。

- [ ] **步骤 4：新增结果文档和最终记忆**

结果文档记录原始/压缩体积、媒体规格、测试命令、视觉检查结果、提交哈希和未部署说明。`AGENTS.md` 只追加最终结论。

- [ ] **步骤 5：提交文档记录**

只暂存本任务新增结果文档和可精确分离的 `AGENTS.md` 条目，提交信息为：

```bash
git commit -m "docs: record CCSwitch usage guide video result"
```

### 任务 5：合并本地 main 并同步 personal/main

- [ ] **步骤 1：确认功能分支提交范围**

```bash
git status --short --branch
git log --oneline main..HEAD
git diff --stat main...HEAD
```

预期：只包含本任务文件；用户既有未提交文件仍未被暂存。

- [ ] **步骤 2：切换本地 main 并快进合并**

```bash
git switch main
git merge --ff-only codex/add-ccswitch-usage-guide-video
```

- [ ] **步骤 3：在 main 上复验**

```bash
pnpm -C frontend test:run src/views/user/__tests__/UsageGuideView.spec.ts
pnpm -C frontend typecheck
git diff --check
```

- [ ] **步骤 4：确认分叉并推送 personal/main**

```bash
git fetch personal main
git rev-list --left-right --count personal/main...main
git push personal main:main
```

不得推送 `origin`。

- [ ] **步骤 5：确认本地和 personal/main 一致**

```bash
git rev-parse main
git rev-parse personal/main
git rev-list --left-right --count personal/main...main
```

预期：两个哈希一致，ahead/behind 为 `0 0`。
