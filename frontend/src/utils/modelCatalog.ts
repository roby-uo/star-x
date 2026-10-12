import { miniMaxVideoSpec } from './minimaxVideo'

export type CatalogModelType = 'Chat' | 'Images' | 'Videos'

// Shared with generated client import scripts so legacy catalogs use the same fallback.
export const catalogImageModelPattern = /^(?:(?:doubao-)?seedream-|gpt-image(?:-|$)|dall-e-|imagen[-.]|flux[-.])/i
export const catalogVideoModelPattern = /^(?:minimax-h3(?:-max)?$|(?:doubao-)?seedance[-.]|kling[-.]|sora(?:[-.]|$)|veo(?:[-.]|$))/i
export const catalogAudioModelPattern = /audio|tts|whisper/i

export function isAudioModel(model: string): boolean {
  return catalogAudioModelPattern.test(model)
}

export function catalogModelType(model: string, billingMode?: string | null): CatalogModelType {
  if (billingMode === 'chat' || billingMode === 'text') return 'Chat'
  if (billingMode === 'image') return 'Images'
  if (billingMode === 'video') return 'Videos'
  if (catalogImageModelPattern.test(model)) return 'Images'
  if (catalogVideoModelPattern.test(model)) return 'Videos'
  return 'Chat'
}

export function catalogModelEndpoint(type: CatalogModelType, platform?: string, model?: string): string {
  if (type === 'Images') return '/v1/images/generations'
  if (type === 'Videos') {
    if (platform === 'grok') return '/v1/videos/generations'
    return platform === 'openai' && miniMaxVideoSpec(model) ? '/v2/video_generation' : '暂未适配'
  }
  if (platform === 'anthropic' || (platform === 'antigravity' && /claude/i.test(model || ''))) return '/v1/messages'
  if (platform === 'gemini' || platform === 'antigravity') return `/v1beta/models/${encodeURIComponent(model || '{model}')}:generateContent`
  return '/v1/chat/completions'
}

/** A model-family label for browsing; it does not assert upstream account ownership. */
export function catalogModelProvider(model: string, platform?: string): string {
  if (/minimax|hailuo/i.test(model)) return 'MiniMax'
  if (/deepseek/i.test(model)) return 'DeepSeek'
  if (/doubao|seedream|seedance/i.test(model)) return '火山引擎'
  if (/^glm|chatglm/i.test(model)) return '智谱'
  if (/qwen|qwq/i.test(model)) return '通义千问'
  if (/claude/i.test(model)) return 'Anthropic'
  if (/gemini|imagen|veo/i.test(model)) return 'Google'
  if (/grok/i.test(model)) return 'xAI'
  if (/^gpt|^o[134](?:-|$)|^sora|chatgpt/i.test(model)) return 'OpenAI'
  return ({ anthropic: 'Anthropic', gemini: 'Google', grok: 'xAI', antigravity: 'Antigravity' } as Record<string, string>)[platform || ''] || '兼容模型'
}
