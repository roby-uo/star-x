export type ModelTestEndpoint = 'Chat' | 'Responses' | 'Images'

export function parseModelTestModels(payload: unknown): string[] {
  if (!payload || typeof payload !== 'object' || !('data' in payload) || !Array.isArray(payload.data)) return []
  const ids = payload.data
    .map((item: unknown) => item && typeof item === 'object' && 'id' in item ? item.id : null)
    .filter((id: unknown): id is string => typeof id === 'string' && Boolean(id.trim()))
  return [...new Set(ids)].sort((a, b) => a.localeCompare(b))
}

export function buildModelTestRequest(endpoint: ModelTestEndpoint, model: string): { path: string; body: Record<string, unknown> } {
  if (endpoint === 'Images') {
    return {
      path: '/v1/images/generations',
      body: { model, prompt: 'A single black dot on a white background', size: '1024x1024', n: 1 }
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
