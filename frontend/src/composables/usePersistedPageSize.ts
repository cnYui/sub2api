import { getConfiguredTableDefaultPageSize, normalizeTablePageSize } from '@/utils/tablePreferences'

const STORAGE_KEY = 'table-page-size'
const LEGACY_SOURCE_KEY = 'table-page-size-source'

function getDefaultPageSize(fallback: number): number {
  return normalizeTablePageSize(getConfiguredTableDefaultPageSize() || fallback)
}

export function getPersistedPageSize(fallback = getConfiguredTableDefaultPageSize()): number {
  if (typeof window !== 'undefined') {
    try {
      const legacySource = window.localStorage.getItem(LEGACY_SOURCE_KEY)
      if (legacySource !== null) {
        // 旧来源标记会让本地偏好覆盖后端默认，必须丢弃。
        window.localStorage.removeItem(STORAGE_KEY)
        window.localStorage.removeItem(LEGACY_SOURCE_KEY)
        return getDefaultPageSize(fallback)
      }

      const stored = window.localStorage.getItem(STORAGE_KEY)
      if (stored !== null) {
        const parsed = Number(stored)
        if (Number.isFinite(parsed)) {
          return normalizeTablePageSize(parsed)
        }
      }
    } catch (error) {
      console.warn('Failed to read persisted page size:', error)
    }
  }
  return getDefaultPageSize(fallback)
}

export function setPersistedPageSize(size: number): void {
  if (typeof window === 'undefined') return
  try {
    window.localStorage.setItem(STORAGE_KEY, String(size))
    window.localStorage.removeItem(LEGACY_SOURCE_KEY)
  } catch (error) {
    console.warn('Failed to persist page size:', error)
  }
}
