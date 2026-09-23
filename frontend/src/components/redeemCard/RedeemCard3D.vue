<template>
  <div
    ref="host"
    class="rc3d-host"
    :class="{ 'rc3d-copied': copied }"
    :style="{ aspectRatio: `${CARD_WIDTH} / ${CARD_HEIGHT}` }"
    role="button"
    tabindex="0"
    :aria-label="ariaLabel"
    @pointerdown="startDrag"
    @keydown.enter.prevent="flip"
    @keydown.space.prevent="flip"
  >
    <div class="rc3d-scene" :style="{ transform: `scale(${scale})` }">
      <div ref="shadowEl" class="rc3d-shadow" :class="`rc3d-shadow-${data.theme}`"></div>
      <div ref="cardEl" class="rc3d-card">
        <div
          v-for="(seg, i) in edges"
          :key="i"
          class="rc3d-edge"
          :style="{
            width: `${seg.length}px`,
            height: `${CARD_THICKNESS}px`,
            transform: `translate3d(${seg.x - seg.length / 2}px, ${seg.y - CARD_THICKNESS / 2}px, 0) rotateZ(${seg.angle}deg) rotateX(90deg)`,
            background: `linear-gradient(${seg.back}, ${seg.front})`
          }"
        ></div>
        <div class="rc3d-face" :style="{ transform: `translateZ(${CARD_THICKNESS / 2}px)` }">
          <RedeemCardFace :data="data" side="front" />
          <div class="rc3d-glare"><div ref="frontGlare" class="rc3d-glare-band"></div></div>
        </div>
        <div class="rc3d-face" :style="{ transform: `rotateY(180deg) translateZ(${CARD_THICKNESS / 2}px)` }">
          <RedeemCardFace :data="data" side="back" :stamp="stamp" />
          <div class="rc3d-glare"><div ref="backGlare" class="rc3d-glare-band"></div></div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import RedeemCardFace from './RedeemCardFace.vue'
import {
  CARD_HEIGHT,
  CARD_THICKNESS,
  CARD_WIDTH,
  cardEdgeSegments,
  type RedeemCardData
} from './redeemCardModel'

// 3D 兑换卡：卡面按设计尺寸（1712 × 1080，20px/mm）排版，整个场景再按容器宽度缩放。
// 厚度 0.6mm = 12px，侧边由沿圆角轮廓立起的小条拼成。
// 交互：拖动旋转，松手后带惯性、最后回正到最近的一面；轻点翻面；
// copyable 时，背面朝前轻点兑换码会复制它（兑换码发光一下表示已复制，不弹文字）。
const props = withDefaults(
  defineProps<{
    data: RedeemCardData
    stamp?: string
    copyable?: boolean
    ariaLabel?: string
  }>(),
  { stamp: '', copyable: false, ariaLabel: '兑换卡，拖动旋转，轻点翻面' }
)

const edges = cardEdgeSegments()

const host = ref<HTMLElement | null>(null)
const cardEl = ref<HTMLElement | null>(null)
const shadowEl = ref<HTMLElement | null>(null)
const frontGlare = ref<HTMLElement | null>(null)
const backGlare = ref<HTMLElement | null>(null)
const scale = ref(0.3)
const copied = ref(false)

// 入场时从斜侧转进来。
const state = {
  rx: 14,
  ry: -32,
  vx: 0,
  vy: 0,
  targetRy: 0,
  dragging: false,
  moved: false,
  pointerId: -1,
  startX: 0,
  startY: 0,
  startRx: 0,
  startRy: 0,
  startTime: 0,
  lastX: 0,
  lastY: 0,
  lastTime: 0,
  downTarget: null as EventTarget | null,
  float: 0
}

const MAX_TILT = 40
const MAX_SPIN = 1.4 // 度/毫秒
const TAP_SLOP = 6
const TAP_TIME = 400

let frameId = 0
let lastFrame = 0
let resizeObserver: ResizeObserver | null = null
let copiedTimer: ReturnType<typeof setTimeout> | undefined
let reducedMotion = false

function clamp(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value))
}

function nearestFace(ry: number) {
  return Math.round(ry / 180) * 180
}

function showingBack() {
  const a = ((state.ry % 360) + 360) % 360
  return a > 90 && a < 270
}

function flip() {
  state.vx = 0
  state.vy = 0
  state.targetRy = nearestFace(state.ry) + 180
}

function startDrag(event: PointerEvent) {
  if (event.pointerType === 'mouse' && event.button !== 0) return
  if (!host.value) return
  state.dragging = true
  state.moved = false
  state.pointerId = event.pointerId
  state.startX = state.lastX = event.clientX
  state.startY = state.lastY = event.clientY
  state.startRx = state.rx
  state.startRy = state.ry
  state.startTime = state.lastTime = performance.now()
  state.downTarget = event.target
  state.vx = 0
  state.vy = 0
  window.addEventListener('pointermove', onPointerMove)
  window.addEventListener('pointerup', onPointerUp)
  window.addEventListener('pointercancel', onPointerCancel)
}

function onPointerMove(event: PointerEvent) {
  if (!state.dragging || event.pointerId !== state.pointerId || !host.value) return
  const dx = event.clientX - state.startX
  const dy = event.clientY - state.startY
  if (!state.moved && Math.hypot(dx, dy) > TAP_SLOP) state.moved = true
  if (!state.moved) return
  const rect = host.value.getBoundingClientRect()
  const prevRx = state.rx
  const prevRy = state.ry
  // 拖过一整张卡宽正好转半圈。
  state.ry = state.startRy + (dx / Math.max(rect.width, 1)) * 180
  state.rx = clamp(state.startRx - (dy / Math.max(rect.height, 1)) * 90, -MAX_TILT, MAX_TILT)
  const now = performance.now()
  const dt = Math.max(now - state.lastTime, 1)
  state.vy = state.vy * 0.5 + ((state.ry - prevRy) / dt) * 0.5
  state.vx = state.vx * 0.5 + ((state.rx - prevRx) / dt) * 0.5
  state.lastX = event.clientX
  state.lastY = event.clientY
  state.lastTime = now
}

function stopListening() {
  window.removeEventListener('pointermove', onPointerMove)
  window.removeEventListener('pointerup', onPointerUp)
  window.removeEventListener('pointercancel', onPointerCancel)
}

function onPointerUp(event: PointerEvent) {
  if (event.pointerId !== state.pointerId) return
  stopListening()
  state.dragging = false
  const isTap = !state.moved && performance.now() - state.startTime < TAP_TIME
  if (!isTap) {
    // 停顿后才松手就不给惯性。
    if (performance.now() - state.lastTime > 80) {
      state.vx = 0
      state.vy = 0
    }
    state.vy = clamp(state.vy, -MAX_SPIN, MAX_SPIN)
    state.vx = clamp(state.vx, -MAX_SPIN, MAX_SPIN)
    state.targetRy = nearestFace(state.ry)
    return
  }
  const target = state.downTarget as Element | null
  const onCard = !!target && !!cardEl.value?.contains(target)
  if (!onCard) return
  if (props.copyable && showingBack() && target?.closest('.rc-code-panel')) {
    void copyCode()
    return
  }
  flip()
}

function onPointerCancel(event: PointerEvent) {
  if (event.pointerId !== state.pointerId) return
  stopListening()
  state.dragging = false
  state.targetRy = nearestFace(state.ry)
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
  copied.value = true
  clearTimeout(copiedTimer)
  copiedTimer = setTimeout(() => {
    copied.value = false
  }, 1200)
}

function render(now: number) {
  const dt = lastFrame ? Math.min(now - lastFrame, 48) : 16
  lastFrame = now

  if (!state.dragging) {
    const spinning = Math.abs(state.vy) > 0.02 || Math.abs(state.vx) > 0.02
    if (spinning) {
      state.ry += state.vy * dt
      state.rx = clamp(state.rx + state.vx * dt, -MAX_TILT, MAX_TILT)
      const decay = Math.pow(0.994, dt)
      state.vy *= decay
      state.vx *= decay
      if (Math.abs(state.vy) <= 0.02 && Math.abs(state.vx) <= 0.02) {
        state.vx = 0
        state.vy = 0
        state.targetRy = nearestFace(state.ry)
      }
    } else {
      const k = 1 - Math.pow(0.993, dt)
      state.ry += (state.targetRy - state.ry) * k
      state.rx += (0 - state.rx) * k
    }
  }

  // 静止时轻轻浮动，拖动时渐停。
  const floatTarget = state.dragging || reducedMotion ? 0 : 1
  state.float += (floatTarget - state.float) * (1 - Math.pow(0.99, dt))
  const t = now / 1000
  const f = state.float
  const rx = state.rx + Math.sin(t * 0.8) * 4 * f
  const ry = state.ry + Math.sin(t * 0.55 + 1.1) * 7 * f
  const lift = Math.sin(t * 0.9) * 14 * f

  if (cardEl.value) {
    cardEl.value.style.transform = `translate3d(0, ${lift.toFixed(2)}px, 0) rotateX(${rx.toFixed(3)}deg) rotateY(${ry.toFixed(3)}deg)`
  }
  if (shadowEl.value) {
    const tilt = Math.min(1, Math.abs(Math.sin((ry * Math.PI) / 180)))
    shadowEl.value.style.transform = `translateY(${(-lift * 0.4).toFixed(2)}px) scaleX(${(1 - tilt * 0.35).toFixed(3)})`
    shadowEl.value.style.opacity = (0.9 - lift / 80).toFixed(3)
  }
  // 高光随水平转角扫过卡面。
  const a = ((ry % 360) + 360) % 360
  const frontAngle = a > 180 ? a - 360 : a
  const backAngle = a - 180
  if (frontGlare.value) frontGlare.value.style.transform = `translateX(${(frontAngle * 1.6).toFixed(2)}%) skewX(-16deg)`
  if (backGlare.value) backGlare.value.style.transform = `translateX(${(-backAngle * 1.6).toFixed(2)}%) skewX(-16deg)`

  frameId = requestAnimationFrame(render)
}

onMounted(() => {
  reducedMotion = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false
  if (reducedMotion) {
    state.rx = 0
    state.ry = 0
  }
  if (host.value) {
    const update = () => {
      if (host.value) scale.value = host.value.clientWidth / CARD_WIDTH
    }
    update()
    resizeObserver = new ResizeObserver(update)
    resizeObserver.observe(host.value)
  }
  frameId = requestAnimationFrame(render)
})

onBeforeUnmount(() => {
  cancelAnimationFrame(frameId)
  resizeObserver?.disconnect()
  stopListening()
  clearTimeout(copiedTimer)
})

defineExpose({ startDrag, flip })
</script>

<style scoped>
.rc3d-host {
  position: relative;
  width: 100%;
  touch-action: none;
  cursor: grab;
  outline: none;
  -webkit-tap-highlight-color: transparent;
}
.rc3d-host:active {
  cursor: grabbing;
}
.rc3d-host:focus-visible {
  box-shadow: 0 0 0 3px rgba(57, 211, 83, 0.45);
  border-radius: 12px;
}
.rc3d-scene {
  position: absolute;
  left: 0;
  top: 0;
  width: 1712px;
  height: 1080px;
  transform-origin: 0 0;
  perspective: 4200px;
}
.rc3d-card {
  position: absolute;
  inset: 0;
  transform-style: preserve-3d;
  will-change: transform;
}
.rc3d-face {
  position: absolute;
  inset: 0;
  border-radius: 64px;
  -webkit-backface-visibility: hidden;
  backface-visibility: hidden;
}
.rc3d-edge {
  position: absolute;
  left: 0;
  top: 0;
}
.rc3d-glare {
  position: absolute;
  inset: 0;
  border-radius: 64px;
  overflow: hidden;
  pointer-events: none;
}
.rc3d-glare-band {
  position: absolute;
  top: -10%;
  bottom: -10%;
  left: 20%;
  width: 60%;
  background: linear-gradient(
    90deg,
    rgba(255, 255, 255, 0) 0%,
    rgba(255, 255, 255, 0.07) 38%,
    rgba(255, 255, 255, 0.16) 50%,
    rgba(255, 255, 255, 0.07) 62%,
    rgba(255, 255, 255, 0) 100%
  );
  will-change: transform;
}
.rc3d-shadow {
  position: absolute;
  left: 12%;
  right: 12%;
  bottom: -110px;
  height: 120px;
  border-radius: 50%;
  filter: blur(46px);
  pointer-events: none;
}
.rc3d-shadow-dark {
  background: rgba(0, 0, 0, 0.75);
}
.rc3d-shadow-light {
  background: rgba(13, 17, 23, 0.28);
}
.rc3d-copied :deep(.rc-code-panel) {
  box-shadow: 0 0 0 6px #39d353, 0 0 60px rgba(57, 211, 83, 0.55);
  transition: box-shadow 0.15s ease-out;
}
</style>
