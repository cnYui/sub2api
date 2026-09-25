<template>
  <div
    class="rc3d-host"
    :class="{ 'rc3d-ready': ready }"
    role="button"
    tabindex="0"
    :aria-label="ariaLabel"
    @keydown.enter.prevent="flip"
    @keydown.space.prevent="flip"
  >
    <div ref="stageHost" class="rc3d-stage"></div>

    <!-- 不支持 WebGL2 或 3D 初始化失败时退回平面卡：轻点翻面，背面轻点兑换码复制。 -->
    <div v-if="fallback" class="rc3d-fallback">
      <div class="rc3d-fallback-card" :class="{ 'rc3d-copied': copied }" @click="onFallbackTap">
        <RedeemCardScaled :data="data" :side="fallbackSide" :stamp="stamp" />
      </div>
    </div>

    <!-- 截贴图用的两面卡面，按设计尺寸排在视口外面；读屏软件不读它。 -->
    <div class="rc3d-capture" data-rc-capture aria-hidden="true">
      <div ref="frontFace"><RedeemCardFace :data="data" side="front" /></div>
      <div ref="backFace"><RedeemCardFace :data="data" side="back" :stamp="stamp" /></div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import RedeemCardFace from './RedeemCardFace.vue'
import RedeemCardScaled from './RedeemCardScaled.vue'
import { regionContains, type FaceRegion, type RedeemCardData, type RedeemCardSide } from './redeemCardModel'
import type { CardHit, CardStage } from './cardStage'
import type { captureFace as CaptureFace, measureRegion as MeasureRegion } from './captureFace'

// 3D 兑换卡，场景照站长的 three.js 模板（见 cardStage.ts）。
// 交互：拖动旋转、滚轮/双指缩放，开场缓慢自转、一碰就停；轻点卡片翻面；
// copyable 时轻点背面的兑换码会复制它，面板亮一下绿光，不弹文字。
const props = withDefaults(
  defineProps<{
    data: RedeemCardData
    stamp?: string
    copyable?: boolean
    zoomable?: boolean
    ariaLabel?: string
  }>(),
  { stamp: '', copyable: false, zoomable: true, ariaLabel: '兑换卡，拖动旋转，轻点翻面' }
)

const stageHost = ref<HTMLElement | null>(null)
const frontFace = ref<HTMLElement | null>(null)
const backFace = ref<HTMLElement | null>(null)
const ready = ref(false)
const fallback = ref(false)
const fallbackSide = ref<RedeemCardSide>('front')
const copied = ref(false)

let stage: CardStage | null = null
let capture: { captureFace: typeof CaptureFace; measureRegion: typeof MeasureRegion } | null = null
let codeRegion: FaceRegion | null = null
let unmounted = false
let captureRun = 0
let captureTimer: ReturnType<typeof setTimeout> | undefined
let copiedTimer: ReturnType<typeof setTimeout> | undefined

function supportsWebGL2(): boolean {
  try {
    const gl = document.createElement('canvas').getContext('webgl2')
    // 探测用的上下文立即释放，浏览器同时存活的 WebGL 上下文有上限。
    gl?.getExtension('WEBGL_lose_context')?.loseContext()
    return !!gl
  } catch {
    return false
  }
}

function useFallback(error?: unknown) {
  if (error) console.error('[redeem-card] 3D 卡片初始化失败，改用平面卡', error)
  stage?.dispose()
  stage = null
  ready.value = false
  fallback.value = true
}

async function refreshTextures() {
  if (!stage || !capture) return
  const { captureFace, measureRegion } = capture
  const run = ++captureRun
  await nextTick()
  const front = frontFace.value?.querySelector<HTMLElement>('.rc-face')
  const back = backFace.value?.querySelector<HTMLElement>('.rc-face')
  if (!front || !back) return
  await document.fonts?.ready
  if (!stage || run !== captureRun) return
  const frontCanvas = await captureFace(front, stage.textureScale)
  const backCanvas = await captureFace(back, stage.textureScale)
  // 截图期间数据又变了（后台连续输入）就丢掉这一轮，等下一轮。
  if (!stage || run !== captureRun) return
  codeRegion = measureRegion(back, '.rc-code-panel')
  stage.setFaces(frontCanvas, backCanvas)
  stage.setCodeRegion(codeRegion)
  ready.value = true
}

function scheduleRefresh() {
  if (!stage) return
  clearTimeout(captureTimer)
  captureTimer = setTimeout(() => {
    refreshTextures().catch(useFallback)
  }, 250)
}

watch(() => [props.data, props.stamp], scheduleRefresh, { deep: true })

function onTap(hit: CardHit | null) {
  if (!hit) return
  if (props.copyable && hit.side === 'back' && codeRegion && regionContains(codeRegion, hit.x, hit.y)) {
    void copyCode()
    return
  }
  stage?.flip()
}

function flip() {
  if (stage) stage.flip()
  else if (fallback.value) fallbackSide.value = fallbackSide.value === 'front' ? 'back' : 'front'
}

function onFallbackTap(event: MouseEvent) {
  const target = event.target as Element | null
  if (props.copyable && fallbackSide.value === 'back' && target?.closest('.rc-code-panel')) {
    void copyCode()
    return
  }
  flip()
}

async function copyCode() {
  const text = props.data.code
  let ok = false
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(text)
      ok = true
    }
  } catch {
    ok = false
  }
  if (!ok) {
    const area = document.createElement('textarea')
    area.value = text
    area.setAttribute('readonly', '')
    area.style.position = 'fixed'
    area.style.opacity = '0'
    document.body.appendChild(area)
    area.select()
    try {
      ok = document.execCommand('copy')
    } catch {
      ok = false
    }
    document.body.removeChild(area)
  }
  if (!ok) return
  stage?.flashCode()
  copied.value = true
  clearTimeout(copiedTimer)
  copiedTimer = setTimeout(() => {
    copied.value = false
  }, 1200)
}

onMounted(async () => {
  if (!supportsWebGL2()) {
    useFallback()
    return
  }
  try {
    // three.js 和截图库都按需加载，平面预览和其它页面不背这两个包。
    const [stageModule, captureModule] = await Promise.all([import('./cardStage'), import('./captureFace')])
    if (unmounted || !stageHost.value) return
    capture = captureModule
    stage = stageModule.createCardStage(stageHost.value, {
      zoomable: props.zoomable,
      reducedMotion: window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false,
      onTap
    })
    await refreshTextures()
  } catch (error) {
    if (!unmounted) useFallback(error)
  }
})

onBeforeUnmount(() => {
  unmounted = true
  captureRun++
  clearTimeout(captureTimer)
  clearTimeout(copiedTimer)
  stage?.dispose()
  stage = null
})

defineExpose({ flip })
</script>

<style scoped>
.rc3d-host {
  position: relative;
  width: 100%;
  height: 100%;
  outline: none;
  -webkit-tap-highlight-color: transparent;
}
.rc3d-host:focus-visible {
  box-shadow: inset 0 0 0 3px rgba(57, 211, 83, 0.45);
}
.rc3d-stage {
  position: absolute;
  inset: 0;
  opacity: 0;
  transition: opacity 0.5s ease;
}
.rc3d-ready .rc3d-stage {
  opacity: 1;
}
.rc3d-fallback {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  container-type: size;
}
.rc3d-fallback-card {
  width: 92%;
  width: min(92cqw, calc(92cqh * 1712 / 1080));
  cursor: pointer;
}
.rc3d-copied :deep(.rc-code-panel) {
  box-shadow: 0 0 0 6px #39d353, 0 0 60px rgba(57, 211, 83, 0.55);
  transition: box-shadow 0.15s ease-out;
}
.rc3d-capture {
  position: fixed;
  left: -100000px;
  top: 0;
  width: 1712px;
  pointer-events: none;
}
</style>
