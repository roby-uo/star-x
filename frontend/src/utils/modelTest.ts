export type ModelTestEndpoint = 'Chat' | 'Responses' | 'Images'

export const SEEDREAM_MODELS = [
  'doubao-seedream-5-0-pro-260628',
  'doubao-seedream-5-0-flash-260915',
  'doubao-seedream-5-0-260128',
  'doubao-seedream-4-5-251128',
  'doubao-seedream-4-0-250828'
] as const

export const isSeedreamModel = (model: string): boolean => /^(doubao-)?seedream-/i.test(model)

export function parseModelTestModels(payload: unknown): string[] {
  if (!payload || typeof payload !== 'object' || !('data' in payload) || !Array.isArray(payload.data)) return []
  const ids = payload.data
    .map((item: unknown) => item && typeof item === 'object' && 'id' in item ? item.id : null)
    .filter((id: unknown): id is string => typeof id === 'string' && Boolean(id.trim()))
  return [...new Set(ids)].sort((a, b) => a.localeCompare(b))
}

export function buildModelTestRequest(endpoint: ModelTestEndpoint, model: string, options: { prompt?: string; image?: string } = {}): { path: string; body: Record<string, unknown> } {
  if (endpoint === 'Images') {
    const prompt = options.prompt?.trim() || 'A single black dot on a white background'
    const image = options.image?.trim()
    return {
      path: '/v1/images/generations',
      body: isSeedreamModel(model)
        ? { model, prompt, size: '2K', response_format: 'url', ...(image ? { image } : {}) }
        : { model, prompt, size: '1024x1024', n: 1 }
    }
  }
  if (endpoint === 'Responses') {
    return { path: '/v1/responses', body: { model, input: 'Reply OK.', max_output_tokens: 64 } }
  }
  const outputLimit = /^gpt-[56](?:\.|-|$)/i.test(model) ? { max_completion_tokens: 32 } : { max_tokens: 32 }
  return {
    path: '/v1/chat/completions',
    body: { model, messages: [{ role: 'user', content: 'Reply OK.' }], ...outputLimit }
  }
}
