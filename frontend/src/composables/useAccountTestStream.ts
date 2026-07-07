export interface AccountTestStreamEvent {
  type: string
  text?: string
  model?: string
  success?: boolean
  error?: string
  image_url?: string
  mime_type?: string
}

export interface AccountTestRequestBodyInput {
  modelId: string
  prompt: string
  supportsImageTest: boolean
  mode?: 'default' | 'compact'
}

export function buildAccountTestRequestBody(input: AccountTestRequestBodyInput) {
  const body: {
    model_id: string
    prompt: string
    mode?: 'default' | 'compact'
  } = {
    model_id: input.modelId,
    prompt: input.supportsImageTest ? input.prompt.trim() : ''
  }

  if (input.mode) {
    body.mode = input.mode
  }

  return body
}

export function parseAccountTestSSEChunk(
  buffer: string,
  chunk: string,
  onParseError?: (error: unknown) => void
) {
  const nextBuffer = buffer + chunk
  const lines = nextBuffer.split('\n')
  const events: AccountTestStreamEvent[] = []
  const rest = lines.pop() || ''

  for (const line of lines) {
    if (!line.startsWith('data: ')) {
      continue
    }
    const jsonStr = line.slice(6).trim()
    if (!jsonStr) {
      continue
    }
    try {
      events.push(JSON.parse(jsonStr))
    } catch (error) {
      onParseError?.(error)
    }
  }

  return {
    events,
    buffer: rest
  }
}
