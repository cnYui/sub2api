import { beforeEach, describe, expect, it, vi } from 'vitest'

const { post } = vi.hoisted(() => ({
  post: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    post,
  },
}))

import { ackTrafficCreditExhaustionEvents } from '@/api/user'

describe('user traffic credit exhaustion api', () => {
  beforeEach(() => {
    post.mockReset()
    post.mockResolvedValue({ data: undefined })
  })

  it('posts event ids to the backend ack endpoint', async () => {
    await ackTrafficCreditExhaustionEvents([7, 9])

    expect(post).toHaveBeenCalledWith('/user/traffic-credit-exhaustion-events/ack', {
      event_ids: [7, 9],
    })
  })
})

