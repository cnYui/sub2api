# 登录与注册页循环世界地图背景实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**目标：** 仅在登录页和注册页增加始终为深色、水平无缝循环的紫色点阵世界地图背景，并保持其他认证页面与认证业务逻辑不变。

**架构：** 新增无业务依赖的 `WorldMapBackground` 装饰组件，使用由 `yui.web/custom.geo.json` 预渲染出的 1800×900 WebP 点阵纹理，通过 CSS transform 循环移动。`AuthLayout` 只提供显式布尔开关，`LoginView` 和 `RegisterView` 负责开启，其他调用方沿用默认关闭状态。

**技术栈：** Vue 3、TypeScript、Tailwind CSS、Vue Test Utils、Vitest、Vite、ImageMagick。

---

## 文件结构

- 新增 `frontend/src/components/layout/WorldMapBackground.vue`：渲染装饰背景、循环动画和减少动态效果降级。
- 新增 `frontend/src/assets/auth/world-map-dots.webp`：1800×900 透明点阵地图纹理，不在运行时加载 GeoJSON。
- 新增 `frontend/src/components/layout/__tests__/AuthLayout.spec.ts`：验证背景开关默认关闭和启用后的布局行为。
- 新增 `frontend/src/components/layout/__tests__/WorldMapBackground.visual.spec.ts`：验证地图资源、无障碍属性、周期参数和静态降级。
- 修改 `frontend/src/components/layout/AuthLayout.vue`：增加显式开关和深色背景状态。
- 修改 `frontend/src/components/layout/__tests__/AuthLayout.visual.spec.ts`：锁定仅登录、注册页面开启背景。
- 修改 `frontend/src/views/auth/LoginView.vue`：开启背景并调整外部页脚文字在固定深色底上的对比度。
- 修改 `frontend/src/views/auth/RegisterView.vue`：开启背景并调整外部页脚文字在固定深色底上的对比度。
- 新增 `docs/ai/context/20260717-103044-auth-world-map-background-result_CN.md`：记录实现结果和验证证据。

### Task 1：锁定认证布局的启用范围

**Files:**
- Create: `frontend/src/components/layout/__tests__/AuthLayout.spec.ts`
- Create: `frontend/src/components/layout/WorldMapBackground.vue`
- Modify: `frontend/src/components/layout/__tests__/AuthLayout.visual.spec.ts`
- Modify: `frontend/src/components/layout/AuthLayout.vue`
- Modify: `frontend/src/views/auth/LoginView.vue:2`
- Modify: `frontend/src/views/auth/RegisterView.vue:2`

- [ ] **Step 1：先写失败的布局行为测试**

创建 `AuthLayout.spec.ts`：

```ts
import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import AuthLayout from '@/components/layout/AuthLayout.vue'

const fetchPublicSettingsMock = vi.hoisted(() => vi.fn())

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    siteName: '天才程序员小站',
    siteLogo: '',
    cachedPublicSettings: { site_subtitle: 'AI API Gateway' },
    publicSettingsLoaded: true,
    fetchPublicSettings: fetchPublicSettingsMock,
  }),
}))

describe('AuthLayout', () => {
  it('默认不渲染世界地图背景', () => {
    const wrapper = mount(AuthLayout)

    expect(wrapper.find('[data-testid="auth-world-map-background"]').exists()).toBe(false)
  })

  it('显式启用后渲染地图并使用固定深色背景', () => {
    const wrapper = mount(AuthLayout, {
      props: { worldMapBackground: true },
    })

    expect(wrapper.find('[data-testid="auth-world-map-background"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="auth-layout"]').classes()).toContain('bg-[#0a0a12]')
    expect(wrapper.get('[data-testid="auth-brand-title"]').classes()).toContain('text-gray-100')
  })
})
```

在 `AuthLayout.visual.spec.ts` 增加页面范围测试：

```ts
const loginPath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../views/auth/LoginView.vue')
const registerPath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../views/auth/RegisterView.vue')
const forgotPasswordPath = resolve(
  dirname(fileURLToPath(import.meta.url)),
  '../../../views/auth/ForgotPasswordView.vue'
)

it('only enables the world map on login and registration pages', () => {
  expect(readFileSync(loginPath, 'utf8')).toContain('<AuthLayout world-map-background>')
  expect(readFileSync(registerPath, 'utf8')).toContain('<AuthLayout world-map-background>')
  expect(readFileSync(forgotPasswordPath, 'utf8')).not.toContain('world-map-background')
})
```

- [ ] **Step 2：运行测试并确认按预期失败**

Run:

```bash
cd frontend
npm run test:run -- src/components/layout/__tests__/AuthLayout.spec.ts src/components/layout/__tests__/AuthLayout.visual.spec.ts
```

Expected: FAIL；`AuthLayout` 尚未渲染测试标记，登录和注册页尚未传入 `world-map-background`。

- [ ] **Step 3：实现最小布局开关**

先创建最小 `WorldMapBackground.vue`：

```vue
<template>
  <div
    aria-hidden="true"
    data-testid="auth-world-map-background"
    class="pointer-events-none absolute inset-0 bg-[#0a0a12]"
  ></div>
</template>
```

在 `AuthLayout.vue` 中：

```ts
import WorldMapBackground from './WorldMapBackground.vue'

withDefaults(
  defineProps<{
    worldMapBackground?: boolean
  }>(),
  {
    worldMapBackground: false,
  }
)
```

将根节点和品牌内容改为显式状态：

```vue
<div
  data-testid="auth-layout"
  class="relative isolate flex min-h-screen items-center justify-center px-4 py-10"
  :class="
    worldMapBackground
      ? 'overflow-hidden bg-[#0a0a12] text-gray-100'
      : 'bg-white text-gray-900 dark:bg-dark-950 dark:text-gray-100'
  "
>
  <WorldMapBackground v-if="worldMapBackground" />
  <div class="relative z-10 w-full max-w-md">
```

品牌标题使用：

```vue
<h1
  data-testid="auth-brand-title"
  class="mb-2 font-display text-4xl font-semibold italic"
  :class="worldMapBackground ? 'text-gray-100' : 'text-gray-950 dark:text-gray-100'"
>
```

品牌副标题使用：

```vue
<p
  class="text-sm"
  :class="worldMapBackground ? 'text-gray-400' : 'text-gray-500 dark:text-gray-400'"
>
  {{ siteSubtitle }}
</p>
```

版权文字使用：

```vue
<div
  class="mt-8 text-center text-xs"
  :class="worldMapBackground ? 'text-gray-500' : 'text-gray-400 dark:text-gray-500'"
>
  &copy; {{ currentYear }} {{ siteName }}. All rights reserved.
</div>
```

登录、注册页分别把根调用改为：

```vue
<AuthLayout world-map-background>
```

登录页 footer 使用：

```vue
<template v-if="!backendModeEnabled" #footer>
  <p class="text-gray-400">
    {{ t('auth.dontHaveAccount') }}
    <router-link
      to="/register"
      class="font-medium text-primary-400 transition-colors hover:text-primary-300"
    >
      {{ t('auth.signUp') }}
    </router-link>
  </p>
</template>
```

注册页 footer 使用：

```vue
<template #footer>
  <p class="text-gray-400">
    {{ t('auth.alreadyHaveAccount') }}
    <router-link
      to="/login"
      class="font-medium text-primary-400 transition-colors hover:text-primary-300"
    >
      {{ t('auth.signIn') }}
    </router-link>
  </p>
</template>
```

卡片内表单颜色保持现状。

- [ ] **Step 4：运行测试并确认通过**

Run:

```bash
cd frontend
npm run test:run -- src/components/layout/__tests__/AuthLayout.spec.ts src/components/layout/__tests__/AuthLayout.visual.spec.ts
```

Expected: PASS，且没有 Vue warning。

- [ ] **Step 5：提交布局边界改动**

```bash
git add frontend/src/components/layout/AuthLayout.vue frontend/src/components/layout/WorldMapBackground.vue frontend/src/components/layout/__tests__/AuthLayout.spec.ts frontend/src/components/layout/__tests__/AuthLayout.visual.spec.ts frontend/src/views/auth/LoginView.vue frontend/src/views/auth/RegisterView.vue
git commit -m "feat: scope world map background to auth entry pages"
```

### Task 2：生成点阵纹理并实现无缝动画

**Files:**
- Create: `frontend/src/assets/auth/world-map-dots.webp`
- Create: `frontend/src/components/layout/__tests__/WorldMapBackground.visual.spec.ts`
- Modify: `frontend/src/components/layout/WorldMapBackground.vue`

- [ ] **Step 1：先写失败的地图组件视觉契约测试**

```ts
import { existsSync, readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const testDirectory = dirname(fileURLToPath(import.meta.url))
const componentPath = resolve(testDirectory, '../WorldMapBackground.vue')
const assetPath = resolve(testDirectory, '../../../assets/auth/world-map-dots.webp')
const source = readFileSync(componentPath, 'utf8')

describe('WorldMapBackground visual contract', () => {
  it('uses a local decorative world map texture', () => {
    expect(existsSync(assetPath)).toBe(true)
    expect(source).toContain("import worldMapDotsUrl from '@/assets/auth/world-map-dots.webp'")
    expect(source).toContain('aria-hidden="true"')
    expect(source).toContain('pointer-events-none')
  })

  it('scrolls one complete map period and respects reduced motion', () => {
    expect(source).toContain('width: calc(100% + 1800px)')
    expect(source).toContain('background-size: 1800px 900px')
    expect(source).toContain('animation: world-map-scroll 60s linear infinite')
    expect(source).toContain('translate3d(-1800px, 0, 0)')
    expect(source).toContain('@media (prefers-reduced-motion: reduce)')
    expect(source).toContain('animation: none')
  })
})
```

- [ ] **Step 2：运行测试并确认按预期失败**

Run:

```bash
cd frontend
npm run test:run -- src/components/layout/__tests__/WorldMapBackground.visual.spec.ts
```

Expected: FAIL；纹理文件不存在，组件尚无动画契约。

- [ ] **Step 3：从参考 GeoJSON 一次性生成 WebP 纹理**

在 `frontend` 目录运行以下生成程序。它只在开发时读取相邻的 `yui.web/custom.geo.json`，产物不保留 GeoJSON 运行时依赖：

```bash
node --input-type=module <<'NODE'
import { mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join, resolve } from 'node:path'
import { spawnSync } from 'node:child_process'

const sourcePath = resolve('../../yui.web/custom.geo.json')
const outputPath = resolve('src/assets/auth/world-map-dots.webp')
const width = 1800
const height = 900
const spacing = 10
const radius = 4.5
const geojson = JSON.parse(readFileSync(sourcePath, 'utf8'))

const polygonRings = geojson.features.flatMap(({ geometry }) => {
  if (geometry.type === 'Polygon') return [geometry.coordinates[0]]
  if (geometry.type === 'MultiPolygon') return geometry.coordinates.map((polygon) => polygon[0])
  return []
})

const polygons = polygonRings.map((ring) => ({
  ring,
  minLon: Math.min(...ring.map(([lon]) => lon)),
  maxLon: Math.max(...ring.map(([lon]) => lon)),
  minLat: Math.min(...ring.map(([, lat]) => lat)),
  maxLat: Math.max(...ring.map(([, lat]) => lat)),
}))

const pointInRing = (lon, lat, ring) => {
  let inside = false
  for (let index = 0, previous = ring.length - 1; index < ring.length; previous = index++) {
    const [currentLon, currentLat] = ring[index]
    const [previousLon, previousLat] = ring[previous]
    const crossesLatitude = currentLat > lat !== previousLat > lat
    if (!crossesLatitude) continue
    const boundaryLon =
      ((previousLon - currentLon) * (lat - currentLat)) / (previousLat - currentLat) + currentLon
    if (lon < boundaryLon) inside = !inside
  }
  return inside
}

const isLand = (lon, lat) =>
  polygons.some(
    ({ ring, minLon, maxLon, minLat, maxLat }) =>
      lon >= minLon && lon <= maxLon && lat >= minLat && lat <= maxLat && pointInRing(lon, lat, ring)
  )

const circles = []
for (let y = 0; y < height; y += spacing) {
  for (let x = 0; x < width; x += spacing) {
    const lon = (x / width) * 360 - 180
    const lat = 90 - (y / height) * 180
    if (isLand(lon, lat)) {
      circles.push(`<circle cx="${x}" cy="${y}" r="${radius}" fill="#54468c"/>`)
    }
  }
}

const temporarySvg = join(tmpdir(), `sub2api-world-map-${process.pid}.svg`)
const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${height}" viewBox="0 0 ${width} ${height}">${circles.join('')}</svg>`
mkdirSync(dirname(outputPath), { recursive: true })
writeFileSync(temporarySvg, svg)

const result = spawnSync(
  'magick',
  ['-background', 'none', temporarySvg, '-define', 'webp:lossless=true', outputPath],
  { stdio: 'inherit' }
)
rmSync(temporarySvg)
if (result.status !== 0) process.exit(result.status ?? 1)
NODE
```

验证纹理：

```bash
magick identify -format '%wx%h %[channels]\n' src/assets/auth/world-map-dots.webp
```

Expected: `1800x900`，并包含 alpha 通道。

- [ ] **Step 4：实现完整背景组件**

```vue
<template>
  <div
    aria-hidden="true"
    data-testid="auth-world-map-background"
    class="pointer-events-none absolute inset-0 overflow-hidden bg-[#0a0a12]"
  >
    <div
      class="world-map-track absolute inset-y-0 left-0"
      :style="{ backgroundImage: `url(${worldMapDotsUrl})` }"
    ></div>
  </div>
</template>

<script setup lang="ts">
import worldMapDotsUrl from '@/assets/auth/world-map-dots.webp'
</script>

<style scoped>
.world-map-track {
  width: calc(100% + 1800px);
  background-position: left center;
  background-repeat: repeat-x;
  background-size: 1800px 900px;
  animation: world-map-scroll 60s linear infinite;
  will-change: transform;
}

@keyframes world-map-scroll {
  from {
    transform: translate3d(0, 0, 0);
  }

  to {
    transform: translate3d(-1800px, 0, 0);
  }
}

@media (prefers-reduced-motion: reduce) {
  .world-map-track {
    animation: none;
    will-change: auto;
  }
}
</style>
```

- [ ] **Step 5：运行组件和布局测试并确认通过**

Run:

```bash
cd frontend
npm run test:run -- src/components/layout/__tests__/WorldMapBackground.visual.spec.ts src/components/layout/__tests__/AuthLayout.spec.ts src/components/layout/__tests__/AuthLayout.visual.spec.ts
```

Expected: PASS。

- [ ] **Step 6：提交地图资源和动画**

```bash
git add frontend/src/assets/auth/world-map-dots.webp frontend/src/components/layout/WorldMapBackground.vue frontend/src/components/layout/__tests__/WorldMapBackground.visual.spec.ts
git commit -m "feat: add looping world map auth background"
```

### Task 3：执行工程和视觉验证

**Files:**
- Verify: `frontend/src/components/layout/WorldMapBackground.vue`
- Verify: `frontend/src/components/layout/AuthLayout.vue`
- Verify: `frontend/src/views/auth/LoginView.vue`
- Verify: `frontend/src/views/auth/RegisterView.vue`

- [ ] **Step 1：运行目标测试、类型检查和生产构建**

```bash
cd frontend
npm run test:run -- src/components/layout/__tests__/WorldMapBackground.visual.spec.ts src/components/layout/__tests__/AuthLayout.spec.ts src/components/layout/__tests__/AuthLayout.visual.spec.ts src/views/auth/__tests__/RegisterView.spec.ts
npm run typecheck
npm run build
```

Expected: 全部 exit code 0，无失败测试和 TypeScript 错误。

- [ ] **Step 2：运行目标文件 ESLint 检查**

```bash
cd frontend
npx eslint src/components/layout/AuthLayout.vue src/components/layout/WorldMapBackground.vue src/components/layout/__tests__/AuthLayout.spec.ts src/components/layout/__tests__/AuthLayout.visual.spec.ts src/components/layout/__tests__/WorldMapBackground.visual.spec.ts src/views/auth/LoginView.vue src/views/auth/RegisterView.vue
```

Expected: exit code 0。

- [ ] **Step 3：启动本地前端开发服务**

```bash
cd frontend
npm run dev -- --host 127.0.0.1 --port 4174
```

Expected: Vite 输出 `http://127.0.0.1:4174/`；若端口占用则递增选择空闲端口。

- [ ] **Step 4：验证桌面和移动端页面**

检查：

- `/login` 与 `/register` 在亮色、深色主题下都显示深色紫色点阵世界地图。
- 观察至少一个完整地图接缝经过视口，确认没有黑色断层、跳帧或重复宽度错误。
- 1440×900 和 390×844 视口中，地图、品牌、卡片、页脚无不合理重叠。
- 输入框、密码显示按钮、登录/注册链接可以正常交互，背景不接收点击。
- 开启 `prefers-reduced-motion: reduce` 后地图保持可见但不移动。
- `/forgot-password` 不显示世界地图，证明默认关闭状态未扩散。

- [ ] **Step 5：检查最终差异**

```bash
git diff HEAD~2 --check
git status --short
git diff HEAD~2 -- frontend/src/components/layout frontend/src/views/auth/LoginView.vue frontend/src/views/auth/RegisterView.vue
```

Expected: 无空白错误；差异只包含计划范围内文件，用户已有后端测试改动未进入功能提交。

### Task 4：归档结果上下文

**Files:**
- Create: `docs/ai/context/20260717-103044-auth-world-map-background-result_CN.md`

- [ ] **Step 1：新增结果文档**

记录：

- 最终组件边界和页面范围。
- WebP 尺寸、文件大小和生成来源，不写入敏感信息。
- 测试、类型检查、构建、ESLint 和桌面/移动视觉验证结果。
- 未修改运行态容器、数据库、Redis、Nginx 或公网链路。
- 用户原有未提交文件保持未修改。

- [ ] **Step 2：检查未跟踪上下文文档**

```bash
git ls-files --others --exclude-standard docs/ai/context
```

Expected: 只列出本次 result 文档。

- [ ] **Step 3：提交结果文档**

```bash
git add docs/ai/context/20260717-103044-auth-world-map-background-result_CN.md
git diff --cached --check
git commit -m "docs: record auth world map background result"
```

- [ ] **Step 4：最终验证**

```bash
git status --short --branch
git log -4 --oneline --decorate
```

Expected: 只保留用户原有 `backend/internal/repository/migrations_schema_integration_test.go` 未提交改动，本次功能和上下文均已提交。
