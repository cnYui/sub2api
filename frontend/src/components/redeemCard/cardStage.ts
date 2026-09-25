// 3D 兑换卡舞台。几何、材质、灯光、取景和翻面动画逐项照搬站长的 Claude Design 模板
// （`兑换卡 3D.html` + `three-d-stage.js`，three.js 0.184）；模板的正反面是静态截图贴图，
// 这里换成运行时从卡面 DOM 截出来的画布，因为面值、兑换码、盖章每张卡都不一样。
import {
  BufferAttribute,
  CanvasTexture,
  Color,
  DataTexture,
  DirectionalLight,
  ExtrudeGeometry,
  HemisphereLight,
  MathUtils,
  Mesh,
  MeshBasicMaterial,
  MeshPhysicalMaterial,
  NeutralToneMapping,
  PCFShadowMap,
  PerspectiveCamera,
  PlaneGeometry,
  Raycaster,
  SRGBColorSpace,
  Scene,
  ShadowMaterial,
  Shape,
  Spherical,
  Vector2,
  Vector3,
  WebGLRenderer,
  type Texture
} from 'three'
import { OrbitControls } from 'three/addons/controls/OrbitControls.js'
import { CARD_HEIGHT, CARD_THICKNESS_MM, CARD_WIDTH, type FaceRegion } from './redeemCardModel'

// CR80 卡，单位米，与模板一致。
export const CARD_W = 0.0856
export const CARD_H = 0.054
export const CARD_THICKNESS_M = CARD_THICKNESS_MM / 1000
const CARD_R = 0.0032
// 卡面贴图边上那圈 3px（0.15mm）渐变描边，延续到同样宽的倒角和渐变侧边上。
const BEVEL = 0.00015
const CW = CARD_W - 2 * BEVEL
const CH = CARD_H - 2 * BEVEL
const GRAD = ['#2b8a99', '#3a3f6e', '#8e2f72'].map((hex) => new Color(hex))

// 几何分组的材质下标：正面、背面、侧边（含倒角）。
export const FRONT = 0
export const BACK = 1
export const EDGE = 2

function roundedRect(w: number, h: number, r: number): Shape {
  const s = new Shape()
  const x = -w / 2
  const y = -h / 2
  s.moveTo(x + r, y)
  s.lineTo(x + w - r, y)
  s.absarc(x + w - r, y + r, r, -Math.PI / 2, 0, false)
  s.lineTo(x + w, y + h - r)
  s.absarc(x + w - r, y + h - r, r, 0, Math.PI / 2, false)
  s.lineTo(x + r, y + h)
  s.absarc(x + r, y + h - r, r, Math.PI / 2, Math.PI, false)
  s.lineTo(x, y + r)
  s.absarc(x + r, y + r, r, Math.PI, Math.PI * 1.5, false)
  s.closePath()
  return s
}

// 挤出的圆角矩形：两个端面重新展 UV 贴卡面（背面左右翻回来，贴图不镜像），
// 侧边和倒角的顶点色是 青→紫→粉 的对角渐变，接上卡面描边。
export function cardGeometry(total: number): ExtrudeGeometry {
  const depth = Math.max(total - 2 * BEVEL, 0.0002)
  const g = new ExtrudeGeometry(roundedRect(CW, CH, CARD_R - BEVEL), {
    depth,
    curveSegments: 14,
    bevelEnabled: true,
    bevelThickness: BEVEL,
    bevelSize: BEVEL,
    bevelOffset: 0,
    bevelSegments: 3
  })
  g.translate(0, 0, -depth / 2)
  g.computeBoundingBox()
  const zMin = g.boundingBox!.min.z
  const zMax = g.boundingBox!.max.z
  const eps = BEVEL * 0.02
  const pos = g.attributes.position
  const uv = g.attributes.uv
  const colors = new Float32Array(pos.count * 3)
  const kind = new Uint8Array(pos.count / 3)
  const tmp = new Color()
  for (let tri = 0; tri < kind.length; tri++) {
    let front = true
    let back = true
    for (let k = 0; k < 3; k++) {
      const z = pos.getZ(tri * 3 + k)
      if (Math.abs(z - zMax) > eps) front = false
      if (Math.abs(z - zMin) > eps) back = false
    }
    kind[tri] = front ? FRONT : back ? BACK : EDGE
    for (let k = 0; k < 3; k++) {
      const i = tri * 3 + k
      const x = pos.getX(i)
      const y = pos.getY(i)
      if (kind[tri] !== EDGE) {
        const u = (x + CW / 2) / CW
        const v = (y + CH / 2) / CH
        uv.setXY(i, kind[tri] === BACK ? 1 - u : u, v)
      }
      const t = MathUtils.clamp(((x + CARD_W / 2) / CARD_W + (CARD_H / 2 - y) / CARD_H) / 2, 0, 1)
      if (t < 0.5) tmp.copy(GRAD[0]).lerp(GRAD[1], t * 2)
      else tmp.copy(GRAD[1]).lerp(GRAD[2], (t - 0.5) * 2)
      colors[i * 3] = tmp.r
      colors[i * 3 + 1] = tmp.g
      colors[i * 3 + 2] = tmp.b
    }
  }
  g.setAttribute('color', new BufferAttribute(colors, 3))
  g.clearGroups()
  let start = 0
  for (let tri = 1; tri <= kind.length; tri++) {
    if (tri === kind.length || kind[tri] !== kind[start]) {
      g.addGroup(start * 3, (tri - start) * 3, kind[start])
      start = tri
    }
  }
  return g
}

// 端面 UV → 卡面设计像素（1712 × 1080，左上角为原点）。背面的 UV 已经翻回来，两面同一个算法。
export function uvToFacePixel(uv: { x: number; y: number }): { x: number; y: number } {
  return { x: uv.x * CARD_WIDTH, y: (1 - uv.y) * CARD_HEIGHT }
}

export interface CardHit {
  side: 'front' | 'back' | 'edge'
  x: number
  y: number
}

export interface CardStageOptions {
  zoomable: boolean
  reducedMotion: boolean
  onTap: (hit: CardHit | null) => void
}

export interface CardStage {
  textureScale: number
  setFaces(front: HTMLCanvasElement, back: HTMLCanvasElement): void
  setCodeRegion(region: FaceRegion | null): void
  flashCode(): void
  flip(): void
  dispose(): void
}

const TAP_SLOP = 6
const TAP_TIME = 400
const FLIP_MS = 700
const GLOW_MS = 1200

function roundRectPath(ctx: CanvasRenderingContext2D, x: number, y: number, w: number, h: number, r: number) {
  ctx.moveTo(x + r, y)
  ctx.arcTo(x + w, y, x + w, y + h, r)
  ctx.arcTo(x + w, y + h, x, y + h, r)
  ctx.arcTo(x, y + h, x, y, r)
  ctx.arcTo(x, y, x + w, y, r)
  ctx.closePath()
}

export function createCardStage(host: HTMLElement, options: CardStageOptions): CardStage {
  const renderer = new WebGLRenderer({ antialias: true, alpha: true })
  renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2))
  renderer.shadowMap.enabled = true
  // 模板写的 PCFSoftShadowMap 在 r184 已废弃，渲染时会被换成 PCFShadowMap，这里直接用它。
  renderer.shadowMap.type = PCFShadowMap
  // 模板：中性色调映射，白卡的高光柔和压下来，不会过曝发灰。
  renderer.toneMapping = NeutralToneMapping
  renderer.toneMappingExposure = 1
  const canvas = renderer.domElement
  canvas.style.display = 'block'
  host.appendChild(canvas)

  const maxAnisotropy = renderer.capabilities.getMaxAnisotropy()
  // 模板贴图是设计稿的 2 倍（3424 × 2160），字才清楚；显卡放不下时退回 1 倍。
  const textureScale = renderer.capabilities.maxTextureSize >= CARD_WIDTH * 2 ? 2 : 1

  const scene = new Scene()
  const camera = new PerspectiveCamera(45, 1, 0.002, 20)
  const controls = new OrbitControls(camera, canvas)
  controls.enableDamping = true
  controls.dampingFactor = 0.08
  controls.minDistance = 0.015
  controls.maxDistance = 0.6
  // 页面上没有「正视」按钮，平移走了就回不来，所以只留旋转和缩放。
  controls.enablePan = false
  controls.enableZoom = options.zoomable
  controls.autoRotate = !options.reducedMotion
  controls.autoRotateSpeed = 1.2
  controls.cursorStyle = 'grab'

  // 灯光：模板把 three-d-stage 的默认值调暗到 半球光 0.7 / 主光 1.6 / 补光 0.4。
  scene.add(new HemisphereLight(0xffffff, 0xd8d2c4, 0.7))
  const key = new DirectionalLight(0xffffff, 1.6)
  key.position.set(4, 7, 5)
  key.castShadow = true
  key.shadow.mapSize.set(2048, 2048)
  key.shadow.bias = -0.0002
  const span = Math.hypot(CARD_W / 2, CARD_H / 2, CARD_THICKNESS_M / 2) * 3
  key.shadow.camera.left = -span
  key.shadow.camera.right = span
  key.shadow.camera.top = span
  key.shadow.camera.bottom = -span
  key.shadow.camera.updateProjectionMatrix()
  scene.add(key)
  const fill = new DirectionalLight(0xfff4e6, 0.4)
  fill.position.set(-5, 3, -4)
  scene.add(fill)

  // 卡片立在地面上，地面只接影子。
  const groundGeometry = new PlaneGeometry(200, 200)
  const groundMaterial = new ShadowMaterial({ opacity: 0.18 })
  const ground = new Mesh(groundGeometry, groundMaterial)
  ground.rotation.x = -Math.PI / 2
  ground.position.y = -CARD_H / 2
  ground.receiveShadow = true
  scene.add(ground)

  // 贴图截好之前先挂一张 1×1 白图，材质一开始就带 map，换贴图时不用重新编译着色器。
  const blank = new DataTexture(new Uint8Array([255, 255, 255, 255]), 1, 1)
  blank.colorSpace = SRGBColorSpace
  blank.needsUpdate = true
  const faceMaterial = (name: string) =>
    new MeshPhysicalMaterial({ name, map: blank, roughness: 0.55, metalness: 0, clearcoat: 0.25, clearcoatRoughness: 0.4 })
  const frontMaterial = faceMaterial('front')
  const backMaterial = faceMaterial('back')
  const edgeMaterial = new MeshPhysicalMaterial({
    name: 'edge_gradient',
    vertexColors: true,
    roughness: 0.45,
    metalness: 0.1,
    clearcoat: 0.5,
    clearcoatRoughness: 0.3
  })
  const geometry = cardGeometry(CARD_THICKNESS_M)
  const card = new Mesh(geometry, [frontMaterial, backMaterial, edgeMaterial])
  card.castShadow = true
  card.receiveShadow = true
  scene.add(card)

  // 复制兑换码后的绿光：贴在背面上方的一层透明平面，只画面板外圈，和平面版 .rc3d-copied 一样。
  const glowCanvas = document.createElement('canvas')
  glowCanvas.width = CARD_WIDTH
  glowCanvas.height = CARD_HEIGHT
  const glowTexture = new CanvasTexture(glowCanvas)
  glowTexture.colorSpace = SRGBColorSpace
  const glowGeometry = new PlaneGeometry(CW, CH)
  const glowMaterial = new MeshBasicMaterial({
    map: glowTexture,
    transparent: true,
    opacity: 0,
    depthWrite: false,
    toneMapped: false,
    polygonOffset: true,
    polygonOffsetFactor: -1,
    polygonOffsetUnits: -4
  })
  const glow = new Mesh(glowGeometry, glowMaterial)
  glow.rotation.y = Math.PI
  glow.position.z = geometry.boundingBox!.min.z
  glow.visible = false
  card.add(glow)
  let glowStart = -1

  let interacted = false
  let flipAnimation: { start: number; from: number; duration: number } | null = null
  const spherical = new Spherical()
  const offset = new Vector3()

  controls.addEventListener('start', () => {
    controls.autoRotate = false
    interacted = true
    flipAnimation = null
  })

  // 模板的取景：几乎正对卡面、略微偏右上，卡片尽量占满视口。
  function fitDistance() {
    const tanV = Math.tan((camera.fov * Math.PI) / 360)
    const tanH = tanV * camera.aspect
    return Math.max(((CARD_W / 2) * 1.12) / tanH, ((CARD_H / 2) * 1.3) / tanV)
  }

  function frame() {
    controls.target.set(0, 0, 0)
    camera.position.set(0.2, 0.14, 1).normalize().multiplyScalar(fitDistance())
    controls.update()
  }

  function resize() {
    const width = Math.max(1, host.clientWidth)
    const height = Math.max(1, host.clientHeight)
    const before = fitDistance()
    renderer.setSize(width, height)
    camera.aspect = width / height
    camera.updateProjectionMatrix()
    if (!interacted) {
      frame()
      return
    }
    // 用户转过以后只按新比例缩放距离，保留他看的角度。
    offset.copy(camera.position).sub(controls.target).multiplyScalar(fitDistance() / before)
    camera.position.copy(controls.target).add(offset)
    controls.update()
  }

  // 模板的「翻面」：相机绕卡片水平转半圈，700ms 缓入缓出。
  function flip() {
    controls.autoRotate = false
    interacted = true
    spherical.setFromVector3(offset.copy(camera.position).sub(controls.target))
    flipAnimation = { start: performance.now(), from: spherical.theta, duration: options.reducedMotion ? 0 : FLIP_MS }
  }

  function stepFlip(now: number) {
    if (!flipAnimation) return
    const k = flipAnimation.duration ? Math.min(1, (now - flipAnimation.start) / flipAnimation.duration) : 1
    const eased = k < 0.5 ? 2 * k * k : 1 - Math.pow(-2 * k + 2, 2) / 2
    spherical.theta = flipAnimation.from + Math.PI * eased
    camera.position.setFromSpherical(spherical).add(controls.target)
    if (k >= 1) flipAnimation = null
  }

  function stepGlow(now: number) {
    if (glowStart < 0) return
    const t = now - glowStart
    glowMaterial.opacity = t < 120 ? t / 120 : t < GLOW_MS - 300 ? 1 : Math.max(0, (GLOW_MS - t) / 300)
    if (t >= GLOW_MS) {
      glow.visible = false
      glowStart = -1
    }
  }

  const raycaster = new Raycaster()
  const pointer = new Vector2()

  function pick(clientX: number, clientY: number): CardHit | null {
    const rect = canvas.getBoundingClientRect()
    pointer.set(((clientX - rect.left) / rect.width) * 2 - 1, -((clientY - rect.top) / rect.height) * 2 + 1)
    raycaster.setFromCamera(pointer, camera)
    const hit = raycaster.intersectObject(card, false)[0]
    if (!hit) return null
    const index = hit.face?.materialIndex
    if ((index === FRONT || index === BACK) && hit.uv) {
      return { side: index === FRONT ? 'front' : 'back', ...uvToFacePixel(hit.uv) }
    }
    return { side: 'edge', x: 0, y: 0 }
  }

  // 轻点 = 按下到抬起没怎么动、也没超过 400ms；两指操作不算。拖动交给 OrbitControls。
  let tap: { id: number; x: number; y: number; time: number } | null = null
  let activePointers = 0
  const onPointerDown = (event: PointerEvent) => {
    activePointers++
    if (activePointers > 1 || (event.pointerType === 'mouse' && event.button !== 0)) {
      tap = null
      return
    }
    tap = { id: event.pointerId, x: event.clientX, y: event.clientY, time: performance.now() }
  }
  const onPointerMove = (event: PointerEvent) => {
    if (tap && event.pointerId === tap.id && Math.hypot(event.clientX - tap.x, event.clientY - tap.y) > TAP_SLOP) {
      tap = null
    }
  }
  const onPointerUp = (event: PointerEvent) => {
    activePointers = Math.max(0, activePointers - 1)
    if (!tap || event.pointerId !== tap.id) return
    const quick = performance.now() - tap.time < TAP_TIME
    tap = null
    if (quick) options.onTap(pick(event.clientX, event.clientY))
  }
  const onPointerCancel = () => {
    activePointers = Math.max(0, activePointers - 1)
    tap = null
  }
  canvas.addEventListener('pointerdown', onPointerDown)
  canvas.addEventListener('pointermove', onPointerMove)
  canvas.addEventListener('pointerup', onPointerUp)
  canvas.addEventListener('pointercancel', onPointerCancel)

  const observer = new ResizeObserver(resize)
  observer.observe(host)
  resize()

  renderer.setAnimationLoop((now: number) => {
    stepFlip(now)
    stepGlow(now)
    controls.update()
    renderer.render(scene, camera)
  })

  function faceTexture(source: HTMLCanvasElement): Texture {
    const texture = new CanvasTexture(source)
    texture.colorSpace = SRGBColorSpace
    texture.anisotropy = maxAnisotropy
    return texture
  }

  function releaseMap(texture: Texture | null) {
    if (texture && texture !== blank) texture.dispose()
  }

  return {
    textureScale,
    setFaces(front, back) {
      const previous = [frontMaterial.map, backMaterial.map]
      frontMaterial.map = faceTexture(front)
      backMaterial.map = faceTexture(back)
      previous.forEach(releaseMap)
    },
    setCodeRegion(region) {
      const ctx = glowCanvas.getContext('2d')
      if (!ctx) return
      ctx.clearRect(0, 0, glowCanvas.width, glowCanvas.height)
      if (region) {
        const { x, y, width, height, radius } = region
        ctx.save()
        ctx.shadowColor = 'rgba(57, 211, 83, 0.55)'
        ctx.shadowBlur = 60
        ctx.fillStyle = '#39d353'
        ctx.beginPath()
        roundRectPath(ctx, x - 6, y - 6, width + 12, height + 12, radius + 6)
        roundRectPath(ctx, x, y, width, height, radius)
        ctx.fill('evenodd')
        ctx.restore()
        // CSS 的外阴影不会画进盒子里面，这里把溢进面板的光晕擦掉。
        ctx.save()
        ctx.globalCompositeOperation = 'destination-out'
        ctx.beginPath()
        roundRectPath(ctx, x, y, width, height, radius)
        ctx.fill()
        ctx.restore()
      }
      glowTexture.needsUpdate = true
    },
    flashCode() {
      glowStart = performance.now()
      glow.visible = true
    },
    flip,
    dispose() {
      renderer.setAnimationLoop(null)
      observer.disconnect()
      canvas.removeEventListener('pointerdown', onPointerDown)
      canvas.removeEventListener('pointermove', onPointerMove)
      canvas.removeEventListener('pointerup', onPointerUp)
      canvas.removeEventListener('pointercancel', onPointerCancel)
      controls.dispose()
      releaseMap(frontMaterial.map)
      releaseMap(backMaterial.map)
      blank.dispose()
      glowTexture.dispose()
      for (const disposable of [geometry, groundGeometry, glowGeometry, frontMaterial, backMaterial, edgeMaterial, groundMaterial, glowMaterial]) {
        disposable.dispose()
      }
      renderer.dispose()
      // 后台来回切换预览会反复建舞台，主动释放 WebGL 上下文，免得超过浏览器上限。
      renderer.forceContextLoss()
      canvas.remove()
    }
  }
}
