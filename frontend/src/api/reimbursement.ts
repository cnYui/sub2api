/**
 * Reimbursement / invoice information API endpoints (user side)
 */

import { apiClient } from './client'
import type { PaginatedResponse } from '@/types'

export interface ReimbursementFields {
  company_name: string | null
  tax_id: string | null
  bank_account: string | null
  bank_name: string | null
  address: string | null
  amount: number | null
}

export type ReimbursementFieldKey = keyof ReimbursementFields

export interface ReimbursementParseResult {
  fields: ReimbursementFields
  missing: ReimbursementFieldKey[]
  complete: boolean
  notes: string | null
}

export type ReimbursementStatus = 'pending' | 'completed'

export interface ReimbursementRequest {
  id: number
  company_name: string
  tax_id: string
  bank_account: string
  bank_name: string
  address: string
  amount: number
  status: ReimbursementStatus
  pdf_available: boolean
  pdf_file_name: string
  created_at: string
  updated_at: string
  completed_at: string | null
}

export interface ReimbursementSubmitPayload {
  company_name: string
  tax_id: string
  bank_account: string
  bank_name: string
  address: string
  amount: number
  raw_text?: string
}

// 六个字段的固定顺序，与服务端 missing 数组顺序一致
export const REIMBURSEMENT_FIELD_KEYS: ReimbursementFieldKey[] = [
  'company_name',
  'tax_id',
  'bank_account',
  'bank_name',
  'address',
  'amount'
]

export function emptyReimbursementFields(): ReimbursementFields {
  return {
    company_name: null,
    tax_id: null,
    bank_account: null,
    bank_name: null,
    address: null,
    amount: null
  }
}

// 后端 LLM 超时上限 120s，实例默认 30s 会在解析完成前先断掉请求，留 10s 余量
const PARSE_REQUEST_TIMEOUT_MS = 130000

export async function parse(text: string, previous?: ReimbursementFields): Promise<ReimbursementParseResult> {
  const payload: { text: string; previous?: ReimbursementFields } = { text }
  if (previous) payload.previous = previous
  const { data } = await apiClient.post<ReimbursementParseResult>('/reimbursement/parse', payload, {
    timeout: PARSE_REQUEST_TIMEOUT_MS
  })
  return data
}

export async function submit(payload: ReimbursementSubmitPayload): Promise<ReimbursementRequest> {
  const { data } = await apiClient.post<ReimbursementRequest>('/reimbursement/requests', payload)
  return data
}

export async function listMine(page: number, pageSize: number): Promise<PaginatedResponse<ReimbursementRequest>> {
  const { data } = await apiClient.get<PaginatedResponse<ReimbursementRequest>>('/reimbursement/requests', {
    params: { page, page_size: pageSize }
  })
  return data
}

export async function getById(id: number): Promise<ReimbursementRequest> {
  const { data } = await apiClient.get<ReimbursementRequest>(`/reimbursement/requests/${id}`)
  return data
}

// 后端出错时返回的 JSON 错误体也会被包成 Blob，调用方需按 blob.type 判断
export async function downloadPdf(id: number): Promise<Blob> {
  const response = await apiClient.get<Blob>(`/reimbursement/requests/${id}/pdf`, { responseType: 'blob' })
  return response.data
}

export const reimbursementAPI = {
  parse,
  submit,
  listMine,
  getById,
  downloadPdf
}

export default reimbursementAPI
