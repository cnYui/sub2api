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
export const CARD_PX_PER_MM = 20
// 站长要求卡片厚度 0.6mm（3D 模板的厚度滑杆最小值）。
export const CARD_THICKNESS_MM = 0.6

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

// 卡面上一块区域（设计像素，左上角为原点），3D 卡用它判断轻点是否落在兑换码面板上。
export interface FaceRegion {
  x: number
  y: number
  width: number
  height: number
  radius: number
}

export function regionContains(region: FaceRegion, x: number, y: number): boolean {
  return x >= region.x && x <= region.x + region.width && y >= region.y && y <= region.y + region.height
}

