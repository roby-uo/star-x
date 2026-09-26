import { describe, expect, it } from 'vitest'
import { buildModelTestRequest, parseModelTestModels } from '../modelTest'

describe('model test requests', () => {
  it('uses the selected endpoint with bounded text output', () => {
    expect(buildModelTestRequest('Chat', 'deepseek-chat')).toMatchObject({ path: '/v1/chat/completions', body: { max_tokens: 32 } })
    expect(buildModelTestRequest('Chat', 'gpt-5.4')).toMatchObject({ path: '/v1/chat/completions', body: { max_completion_tokens: 32 } })
    expect(buildModelTestRequest('Responses', 'gpt-5.4')).toMatchObject({ path: '/v1/responses', body: { max_output_tokens: 64 } })
    expect(buildModelTestRequest('Images', 'gpt-image-2')).toMatchObject({ path: '/v1/images/generations', body: { n: 1 } })
  })

  it('uses model IDs returned by the key without hard-coded provider filtering', () => {
    expect(parseModelTestModels({ data: [{ id: 'seedream-new' }, { id: 'gpt-image-2' }, { id: 'seedream-new' }] }))
      .toEqual(['gpt-image-2', 'seedream-new'])
    expect(parseModelTestModels({ data: null })).toEqual([])
  })
})
