/**
 * 兑换卡公开接口：用户打开 /card/<token> 时取卡面数据，无需登录。
 */

import { buildApiUrl } from './url'
import type { RedeemCardData } from '@/components/redeemCard/redeemCardModel'

export interface PublicRedeemCard extends RedeemCardData {
  code_status: string
}

export interface PublicRedeemCardError {
  status: number
}

// 不走 apiClient：它会带上本地保存的登录令牌，令牌过期时接口回 401，
// 拦截器随即跳转登录页。收卡的人不需要登录，这里直接匿名请求。
export async function getPublicRedeemCard(token: string): Promise<PublicRedeemCard> {
  let response: Response
  try {
    response = await fetch(buildApiUrl(`/redeem-cards/${encodeURIComponent(token)}`), {
      credentials: 'omit',
      headers: { Accept: 'application/json' }
    })
  } catch {
    throw { status: 0 } satisfies PublicRedeemCardError
  }
  const body = (await response.json().catch(() => null)) as { code?: number; data?: PublicRedeemCard } | null
  if (!response.ok || !body || body.code !== 0 || !body.data) {
    throw { status: response.ok ? 500 : response.status } satisfies PublicRedeemCardError
  }
  return body.data
}
