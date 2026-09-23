// 兑换卡：站长设计的实体卡（85.6 × 54 mm），黑色版与白色版两套配色。
// 设计稿按 20px/mm 绘制，卡面组件始终按 1712 × 1080 的设计尺寸排版，再整体缩放。

export type RedeemCardTheme = 'dark' | 'light'
export type RedeemCardSide = 'front' | 'back'

export interface RedeemCardContent {
  amount: string
  plan: string
  tokens: string
  valid_until: string
  serial: string
  heatmap_seed: number
}

// 所有卡片共用的卡面信息，后台「兑换卡」页可改，改一次所有已发出的卡片都会更新。
export interface RedeemCardProfile {
  owner_label: string
  owner_name: string
  owner_lines: string[]
  steps: string[]
  left_qr_image: string
  left_qr_caption: string
  right_qr_image: string
  right_qr_caption: string
}

export interface RedeemCardData {
  theme: RedeemCardTheme
  code: string
  content: RedeemCardContent
  profile: RedeemCardProfile
}

export const CARD_WIDTH = 1712
export const CARD_HEIGHT = 1080
export const CARD_RADIUS = 64
export const CARD_PX_PER_MM = 20
// 站长要求卡片厚度 0.6mm。
export const CARD_THICKNESS_MM = 0.6
export const CARD_THICKNESS = CARD_THICKNESS_MM * CARD_PX_PER_MM

// 二维码没有上传自定义图片时，使用站点邮件里同一套图片。
export const DEFAULT_QR_IMAGES = {
  left: '/email/qr-wechat.png',
  right: '/email/qr-site.png'
}

export const DEFAULT_HEATMAP_SEED = 7

export function defaultRedeemCardProfile(): RedeemCardProfile {
  return {
    owner_label: '主理人',
    owner_name: '悠一',
    owner_lines: ['去探索 · TOP100 探索者', 'NEXIS · Cobuilder'],
    steps: ['登录进入 aaccx.pw/login', '进入兑换页面输入兑换码', '额度即时到账'],
    left_qr_image: '',
    left_qr_caption: '微信群 · 天才程序员聚集地',
    right_qr_image: '',
    right_qr_caption: '官网 · aaccx.pw/login'
  }
}

export function emptyRedeemCardContent(): RedeemCardContent {
  return {
    amount: '',
    plan: '',
    tokens: '',
    valid_until: '',
    serial: '',
    heatmap_seed: DEFAULT_HEATMAP_SEED
  }
}

const HEATMAP_GREENS: Record<RedeemCardTheme, string[]> = {
  dark: ['#161b22', '#0e4429', '#006d32', '#26a641', '#39d353'],
  light: ['#ebedf0', '#9be9a8', '#40c463', '#30a14e', '#216e39']
}

// 与设计稿里的生成逻辑逐行一致：同一个种子在后台预览和用户的卡片上画出同一张热力图。
export function heatmapColors(seed: number, theme: RedeemCardTheme): string[] {
  const safeSeed = Number(seed) || DEFAULT_HEATMAP_SEED
  let s = safeSeed >>> 0
  const rnd = () => {
    s = (s + 0x6d2b79f5) | 0
    let t = Math.imul(s ^ (s >>> 15), 1 | s)
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
  const greens = HEATMAP_GREENS[theme]
  const cells: string[] = []
  for (let w = 0; w < 52; w++) {
    const wave = 0.3 + 0.7 * Math.abs(Math.sin(w / 5.3 + safeSeed))
    for (let d = 0; d < 7; d++) {
      let level = 0
      if (rnd() < wave * 0.8) level = 1 + Math.floor(rnd() * 4 * wave)
      cells.push(greens[Math.min(4, level)])
    }
  }
  return cells
}

// 兑换码按 4 位一组显示，每行最多 4 组。
// 只做视觉分组、不插入任何字符，也不改大小写：兑换时按原文精确匹配，照卡片抄写必须能兑换成功。
export function codeLines(code: string): string[][] {
  const text = code.trim()
  const groups = text.match(/.{1,4}/g) ?? []
  const lines: string[][] = []
  for (let i = 0; i < groups.length; i += 4) {
    lines.push(groups.slice(i, i + 4))
  }
  return lines.length ? lines : [[]]
}

export function codeFontSize(lineCount: number): number {
  if (lineCount <= 1) return 112
  if (lineCount === 2) return 92
  return 72
}

// 选中兑换码时的卡面预填值，管理员可以再改。只用到兑换码自身和它绑定的套餐档位。
export interface RedeemCodeForCard {
  id: number
  type: string
  value: number
  expires_at?: string | null
  validity_days?: number
  group?: { name?: string } | null
  balance_package_plan?: {
    name: string
    price_cny: number
    weekly_credit_usd: number
    validity_days: number
    refresh_count: number
  } | null
}

function trimNumber(value: number): string {
  return Number.isInteger(value) ? String(value) : value.toFixed(2)
}

export function contentFromRedeemCode(code: RedeemCodeForCard, heatmapSeed = DEFAULT_HEATMAP_SEED): RedeemCardContent {
  const content = emptyRedeemCardContent()
  content.heatmap_seed = heatmapSeed
  content.serial = String(code.id).padStart(4, '0')
  content.valid_until = code.expires_at ? code.expires_at.slice(0, 10) : '长期有效'
  const plan = code.balance_package_plan
  switch (code.type) {
    case 'balance_package':
      if (plan) {
        content.amount = `¥${trimNumber(plan.price_cny)}`
        content.plan = `${plan.name} · ${plan.validity_days} 天`
        content.tokens = `每期 $${trimNumber(plan.weekly_credit_usd)} × ${plan.refresh_count} 期`
      }
      break
    case 'subscription':
      content.plan = `${code.group?.name ?? '订阅'} · ${code.validity_days ?? 0} 天`
      break
    case 'balance':
      content.amount = `$${trimNumber(code.value)}`
      content.plan = '余额充值'
      break
    default:
      content.amount = trimNumber(code.value)
  }
  return content
}

export function formatSerial(serial: string): string {
  const value = serial.trim()
  return value ? `No.${value}` : ''
}

const CODE_STATUS_STAMPS: Record<string, string> = {
  used: '已兑换',
  expired: '已过期',
  disabled: '已停用'
}

// 兑换码不能再用时，卡片背面盖一个章；未使用或状态未知不盖。
export function codeStatusStamp(status: string | null | undefined): string {
  return CODE_STATUS_STAMPS[status ?? ''] ?? ''
}

// 卡面描边是 135° 渐变：色标 0% / 50% / 100%。
const BORDER_STOPS: Array<[number, [number, number, number]]> = [
  [0, [0x2b, 0x8a, 0x99]],
  [0.5, [0x3a, 0x3f, 0x6e]],
  [1, [0x8e, 0x2f, 0x72]]
]

function toHex(value: number): string {
  return Math.round(Math.min(255, Math.max(0, value))).toString(16).padStart(2, '0')
}

// 卡面 (x, y) 处的描边颜色。135° 渐变在 W×H 的盒子上恰好是 t = (x + y) / (W + H)。
// shade < 1 时整体压暗，侧边用它做出比卡面略深的「截面」。
export function borderColorAt(x: number, y: number, shade = 1): string {
  const t = Math.min(1, Math.max(0, (x + y) / (CARD_WIDTH + CARD_HEIGHT)))
  let i = 0
  while (i < BORDER_STOPS.length - 2 && t > BORDER_STOPS[i + 1][0]) i++
  const [t0, c0] = BORDER_STOPS[i]
  const [t1, c1] = BORDER_STOPS[i + 1]
  const k = (t - t0) / (t1 - t0)
  return `#${c0.map((v, j) => toHex((v + (c1[j] - v) * k) * shade)).join('')}`
}

// 3D 卡片的侧边：沿圆角矩形外轮廓切成若干竖直的小条，每条立在卡面上、高度即卡片厚度。
// x / y 是小条中心在卡面上的位置，angle 是小条沿轮廓的走向（度）。
// front / back 分别是这一点在正面、背面的描边颜色（背面翻转过，同一点左右对调），
// 侧边从背面颜色过渡到正面颜色，转到任何角度都和两面的描边接得上。
export interface CardEdgeSegment {
  x: number
  y: number
  length: number
  angle: number
  front: string
  back: string
}

// 相邻小条略微重叠，避免斜着看时露出缝。
const EDGE_OVERLAP = 1

export function cardEdgeSegments(options: { sideSegments?: number; cornerSegments?: number; shade?: number } = {}): CardEdgeSegment[] {
  const W = CARD_WIDTH
  const H = CARD_HEIGHT
  const R = CARD_RADIUS
  const cornerSegments = options.cornerSegments ?? 8
  const sideLength = options.sideSegments ?? 66
  const shade = options.shade ?? 0.82
  const segments: CardEdgeSegment[] = []
  const push = (x: number, y: number, length: number, angle: number) => {
    segments.push({
      x,
      y,
      length: length + EDGE_OVERLAP,
      angle,
      front: borderColorAt(x, y, shade),
      back: borderColorAt(W - x, y, shade)
    })
  }
  const line = (x1: number, y1: number, x2: number, y2: number) => {
    const total = Math.hypot(x2 - x1, y2 - y1)
    const n = Math.max(1, Math.round(total / sideLength))
    const angle = (Math.atan2(y2 - y1, x2 - x1) * 180) / Math.PI
    for (let i = 0; i < n; i++) {
      const t = (i + 0.5) / n
      push(x1 + (x2 - x1) * t, y1 + (y2 - y1) * t, total / n, angle)
    }
  }
  const corner = (cx: number, cy: number, fromDeg: number) => {
    const step = 90 / cornerSegments
    const chord = 2 * R * Math.sin(((step / 2) * Math.PI) / 180)
    for (let i = 0; i < cornerSegments; i++) {
      const deg = fromDeg + step * (i + 0.5)
      const rad = (deg * Math.PI) / 180
      push(cx + R * Math.cos(rad), cy + R * Math.sin(rad), chord, deg + 90)
    }
  }
  // 顺时针：上边、右上角、右边、右下角、下边、左下角、左边、左上角（y 轴向下）。
  line(R, 0, W - R, 0)
  corner(W - R, R, 270)
  line(W, R, W, H - R)
  corner(W - R, H - R, 0)
  line(W - R, H, R, H)
  corner(R, H - R, 90)
  line(0, H - R, 0, R)
  corner(R, R, 180)
  return segments
}
