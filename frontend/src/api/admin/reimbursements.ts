/**
 * Admin Reimbursement API endpoints
 * Handles reimbursement request review, PDF upload and LLM parser config
 */

import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'
import type { ReimbursementParseResult, ReimbursementRequest, ReimbursementStatus } from '../reimbursement'

export interface AdminReimbursementRequest extends ReimbursementRequest {
  user_id: number
  user_email: string
  username: string
  raw_text: string
  pdf_size: number
  pdf_uploaded_at: string | null
  handled_by: number | null
  notified_at: string | null
}

export interface ReimbursementLLMConfigView {
  base_url: string
  model: string
  timeout_ms: number
  api_key_configured: boolean
  api_key_masked: string
  api_key_source: 'settings' | 'env' | ''
}

export interface ReimbursementLLMConfigUpdate {
  base_url?: string
  model?: string
  timeout_ms?: number
  api_key?: string
  clear_api_key?: boolean
}

export interface ReimbursementConfigTestResult {
  ok: boolean
  latency_ms: number
  model: string
  result: ReimbursementParseResult
}

export interface AdminReimbursementListFilters {
  status?: ReimbursementStatus | ''
  search?: string
  sort_by?: string
  sort_order?: 'asc' | 'desc'
}

export async function list(
  page: number = 1,
  pageSize: number = 20,
  filters?: AdminReimbursementListFilters,
  options?: { signal?: AbortSignal }
): Promise<PaginatedResponse<AdminReimbursementRequest>> {
  const { data } = await apiClient.get<PaginatedResponse<AdminReimbursementRequest>>(
    '/admin/reimbursement/requests',
    {
      params: {
        page,
        page_size: pageSize,
        ...filters
      },
      signal: options?.signal
    }
  )
  return data
}

export async function getById(id: number): Promise<AdminReimbursementRequest> {
  const { data } = await apiClient.get<AdminReimbursementRequest>(`/admin/reimbursement/requests/${id}`)
  return data
}

// 后端 LLM 超时上限 120s，实例默认 30s 会在解析完成前先断掉请求，留 10s 余量
const PARSE_REQUEST_TIMEOUT_MS = 130000
// PDF 最大 20MB，慢网络下 30s 传不完；上传失败无法重试到一半，给足余量
const UPLOAD_REQUEST_TIMEOUT_MS = 180000

// 实例默认头是 application/json，必须显式覆盖才能让浏览器自动补 multipart boundary
export async function uploadPdf(id: number, file: File): Promise<AdminReimbursementRequest> {
  const formData = new FormData()
  formData.append('file', file)
  const { data } = await apiClient.post<AdminReimbursementRequest>(
    `/admin/reimbursement/requests/${id}/pdf`,
    formData,
    { headers: { 'Content-Type': 'multipart/form-data' }, timeout: UPLOAD_REQUEST_TIMEOUT_MS }
  )
  return data
}

export async function downloadPdf(id: number): Promise<Blob> {
  const response = await apiClient.get<Blob>(`/admin/reimbursement/requests/${id}/pdf`, {
    responseType: 'blob'
  })
  return response.data
}

export async function getConfig(): Promise<ReimbursementLLMConfigView> {
  const { data } = await apiClient.get<ReimbursementLLMConfigView>('/admin/reimbursement/config')
  return data
}

export async function updateConfig(payload: ReimbursementLLMConfigUpdate): Promise<ReimbursementLLMConfigView> {
  const { data } = await apiClient.put<ReimbursementLLMConfigView>('/admin/reimbursement/config', payload)
  return data
}

export async function testConfig(text?: string): Promise<ReimbursementConfigTestResult> {
  const payload: { text?: string } = {}
  if (text) payload.text = text
  const { data } = await apiClient.post<ReimbursementConfigTestResult>('/admin/reimbursement/config/test', payload, {
    timeout: PARSE_REQUEST_TIMEOUT_MS
  })
  return data
}

export const reimbursementsAPI = {
  list,
  getById,
  uploadPdf,
  downloadPdf,
  getConfig,
  updateConfig,
  testConfig
}

export default reimbursementsAPI
