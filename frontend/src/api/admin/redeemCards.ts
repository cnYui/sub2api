/**
 * Admin redeem card API：为兑换码生成 3D 兑换卡分享链接，以及所有卡片共用的卡面信息。
 */

import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'
import type {
  RedeemCardContent,
  RedeemCardProfile,
  RedeemCardTheme
} from '@/components/redeemCard/redeemCardModel'

export interface AdminRedeemCard {
  id: number
  token: string
  redeem_code_id: number
  theme: RedeemCardTheme
  content: RedeemCardContent
  view_count: number
  last_viewed_at?: string | null
  created_at: string
  updated_at: string
  code: string
  code_type: string
  code_value: number
  code_status: string
}

export interface SaveRedeemCardRequest {
  redeem_code_id: number
  theme: RedeemCardTheme
  content: RedeemCardContent
}

export async function list(
  page: number = 1,
  pageSize: number = 20,
  search?: string
): Promise<PaginatedResponse<AdminRedeemCard>> {
  const { data } = await apiClient.get<PaginatedResponse<AdminRedeemCard>>('/admin/redeem-cards', {
    params: { page, page_size: pageSize, search: search || undefined }
  })
  return data
}

/** 兑换码还没有卡片时返回 null。 */
export async function getByCode(redeemCodeId: number): Promise<AdminRedeemCard | null> {
  const { data } = await apiClient.get<AdminRedeemCard | null>(`/admin/redeem-cards/by-code/${redeemCodeId}`)
  return data ?? null
}

/** 没有卡片就新建，有就更新卡面；链接（token）保持不变。 */
export async function save(payload: SaveRedeemCardRequest): Promise<AdminRedeemCard> {
  const { data } = await apiClient.post<AdminRedeemCard>('/admin/redeem-cards', payload)
  return data
}

/** 撤销分享链接，兑换码本身不受影响。 */
export async function remove(id: number): Promise<void> {
  await apiClient.delete(`/admin/redeem-cards/${id}`)
}

export async function getProfile(): Promise<RedeemCardProfile> {
  const { data } = await apiClient.get<RedeemCardProfile>('/admin/redeem-card-profile')
  return data
}

export async function updateProfile(profile: RedeemCardProfile): Promise<RedeemCardProfile> {
  const { data } = await apiClient.put<RedeemCardProfile>('/admin/redeem-card-profile', profile)
  return data
}

/** 发给用户的链接：/card/<token>。 */
export function shareUrl(token: string): string {
  return `${window.location.origin}/card/${token}`
}

export const redeemCardsAPI = {
  list,
  getByCode,
  save,
  remove,
  getProfile,
  updateProfile,
  shareUrl
}

export default redeemCardsAPI
