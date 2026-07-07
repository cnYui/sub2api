import { describe, expect, it } from 'vitest'
import {
  buildAccountTestRequestBody,
  parseAccountTestSSEChunk
} from '../useAccountTestStream'

describe('useAccountTestStream', () => {
  it('跨 chunk 解析 SSE data 事件并保留未完成 buffer', () => {
    const first = parseAccountTestSSEChunk('', 'data: {"type":"content","text":"hel')
    expect(first.events).toEqual([])
    expect(first.buffer).toBe('data: {"type":"content","text":"hel')

    const second = parseAccountTestSSEChunk(
      first.buffer,
      'lo"}\n\ndata: {"type":"test_complete","success":true}\n\n'
    )

    expect(second.events).toEqual([
      { type: 'content', text: 'hello' },
      { type: 'test_complete', success: true }
    ])
    expect(second.buffer).toBe('')
  })

  it('生成图片测试与 OpenAI compact 测试请求体', () => {
    expect(
      buildAccountTestRequestBody({
        modelId: 'gpt-image-2',
        prompt: ' draw a cat ',
        supportsImageTest: true,
        mode: 'compact'
      })
    ).toEqual({
      model_id: 'gpt-image-2',
      prompt: 'draw a cat',
      mode: 'compact'
    })

    expect(
      buildAccountTestRequestBody({
        modelId: 'claude-sonnet-4',
        prompt: 'ignored',
        supportsImageTest: false
      })
    ).toEqual({
      model_id: 'claude-sonnet-4',
      prompt: ''
    })
  })
})
