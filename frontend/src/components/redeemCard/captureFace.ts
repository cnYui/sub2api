// 把卡面 DOM 截成 3D 卡的贴图。截图库只在这里按需加载，平面预览用不到它。
import html2canvas from 'html2canvas-pro'
import { CARD_HEIGHT, CARD_WIDTH, type FaceRegion } from './redeemCardModel'

// RedeemCard3D 给视口外的卡面容器打的标记，onclone 靠它把克隆出来的那份挪回原点。
const CAPTURE_ATTR = 'data-rc-capture'

// 卡面圆角外是透明的，而 3D 端面的圆角比贴图大一点点，透明像素在显卡里是黑色，
// 会在四个角露出黑点。先垫一层与描边相同的 135° 渐变，角上就和倒角、侧边连成一片。
function underlayRim(canvas: HTMLCanvasElement) {
  const ctx = canvas.getContext('2d')
  if (!ctx) return
  const w = canvas.width
  const h = canvas.height
  // CSS 135° 渐变线：过中心、方向 (1, 1)，长度让左上角和右下角正好是 0% 和 100%。
  const gradient = ctx.createLinearGradient((w - h) / 4, (h - w) / 4, (3 * w + h) / 4, (w + 3 * h) / 4)
  gradient.addColorStop(0, '#2b8a99')
  gradient.addColorStop(0.5, '#3a3f6e')
  gradient.addColorStop(1, '#8e2f72')
  ctx.save()
  ctx.globalCompositeOperation = 'destination-over'
  ctx.fillStyle = gradient
  ctx.fillRect(0, 0, w, h)
  ctx.restore()
}

export async function captureFace(face: HTMLElement, scale: number): Promise<HTMLCanvasElement> {
  const canvas = await html2canvas(face, {
    backgroundColor: null,
    scale,
    logging: false,
    useCORS: true,
    width: CARD_WIDTH,
    height: CARD_HEIGHT,
    windowWidth: CARD_WIDTH,
    windowHeight: CARD_HEIGHT * 2,
    // 只克隆卡面和它的祖先（外加 <head> 里的样式），后台整页 DOM 不必拷一份。
    ignoreElements: (node) =>
      !(node.tagName === 'HEAD' || node.parentElement?.tagName === 'HEAD' || node.contains(face) || face.contains(node)),
    onclone: (_doc, cloned) => {
      const container = cloned.closest<HTMLElement>(`[${CAPTURE_ATTR}]`)
      if (container) {
        container.style.left = '0px'
        container.style.top = '0px'
      }
    }
  })
  underlayRim(canvas)
  return canvas
}

// 兑换码面板在卡面上的位置（设计像素），用来判断轻点是否落在兑换码上、以及绿光画在哪。
export function measureRegion(face: HTMLElement, selector: string): FaceRegion | null {
  const target = face.querySelector<HTMLElement>(selector)
  if (!target) return null
  const outer = face.getBoundingClientRect()
  const rect = target.getBoundingClientRect()
  return {
    x: rect.left - outer.left,
    y: rect.top - outer.top,
    width: rect.width,
    height: rect.height,
    radius: parseFloat(getComputedStyle(target).borderTopLeftRadius) || 0
  }
}
