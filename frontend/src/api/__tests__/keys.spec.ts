import { beforeEach, describe, expect, it, vi } from 'vitest'

const { post } = vi.hoisted(() => ({
  post: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    post,
  },
}))

import { create } from '@/api/keys'

describe('keys api', () => {
  beforeEach(() => {
    post.mockReset()
    post.mockResolvedValue({ data: { id: 1, name: 'Auto Key' } })
  })

  it('creates user keys without sending group_id', async () => {
    await create(
      'Auto Key',
      'custom-secret',
      ['127.0.0.1'],
      ['10.0.0.1'],
      12,
      30,
      { rate_limit_5h: 1, rate_limit_1d: 2, rate_limit_7d: 3 }
    )

    expect(post).toHaveBeenCalledWith('/keys', {
      name: 'Auto Key',
      custom_key: 'custom-secret',
      ip_whitelist: ['127.0.0.1'],
      ip_blacklist: ['10.0.0.1'],
      quota: 12,
      expires_in_days: 30,
      rate_limit_5h: 1,
      rate_limit_1d: 2,
      rate_limit_7d: 3,
    })
  })
})
