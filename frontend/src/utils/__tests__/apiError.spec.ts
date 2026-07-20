import { describe, expect, it } from 'vitest'
import { extractApiErrorCode, extractApiErrorMessage } from '@/utils/apiError'

describe('规范错误解析', () => {
  it('读取 OpenAI 嵌套错误的英文提示与规范代码', () => {
    const error = {
      response: {
        data: {
          error: {
            message: 'The upstream service is rate limited. Please retry later.',
            sub2api_code: 'UPSTREAM_RATE_LIMITED',
          },
        },
      },
    }

    expect(extractApiErrorMessage(error, 'fallback')).toBe('The upstream service is rate limited. Please retry later.')
    expect(extractApiErrorCode(error)).toBe('UPSTREAM_RATE_LIMITED')
  })
})
