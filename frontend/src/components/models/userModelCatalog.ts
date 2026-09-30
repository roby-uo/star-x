import type { UserAvailableModel } from '@/api/channels'
import { catalogModelProvider, catalogModelType, isAudioModel, type CatalogModelType } from '@/utils/modelCatalog'

export interface UserCatalogItem {
  key: string
  name: string
  platform: string
  type: CatalogModelType
  groups: UserAvailableModel[]
}

/** A model is one catalog item; its group-specific access remains separate. */
export function aggregateUserModels(models: UserAvailableModel[]): UserCatalogItem[] {
  const items = new Map<string, UserCatalogItem>()
  for (const model of models) {
    if (isAudioModel(model.name)) continue
    const type = catalogModelType(model.name, model.policy?.kind || model.pricing?.billing_mode)
    const key = JSON.stringify([model.platform, model.name, type])
    const item = items.get(key) || { key, name: model.name, platform: model.platform, type, groups: [] }
    if (!item.groups.some(group => group.group_id === model.group_id)) item.groups.push(model)
    items.set(key, item)
  }
  return [...items.values()].sort((a, b) => a.name.localeCompare(b.name))
}

export const catalogTypeLabels: Record<CatalogModelType, string> = { Chat: '文本', Images: '图像', Videos: '视频' }

export function modelSupplier(name: string, platform: string): string {
  return catalogModelProvider(name, platform)
}

export function protocolLabel(platform: string): string {
  return ({ openai: 'OpenAI 兼容', anthropic: 'Anthropic', gemini: 'Gemini', antigravity: 'Antigravity', grok: 'OpenAI 兼容' } as Record<string, string>)[platform] || platform
}

export function buildTextRequest(model: UserAvailableModel, prompt: string): { path: string; body: Record<string, unknown>; headers: Record<string, string> } {
  if (model.platform === 'anthropic' || (model.platform === 'antigravity' && /claude/i.test(model.name))) {
    return { path: '/v1/messages', headers: { 'anthropic-version': '2023-06-01' }, body: { model: model.name, messages: [{ role: 'user', content: prompt }], max_tokens: 256 } }
  }
  if (model.platform === 'gemini' || model.platform === 'antigravity') {
    return { path: `/v1beta/models/${encodeURIComponent(model.name)}:generateContent`, headers: {}, body: { contents: [{ role: 'user', parts: [{ text: prompt }] }], generationConfig: { maxOutputTokens: 256 } } }
  }
  return { path: '/v1/chat/completions', headers: {}, body: { model: model.name, messages: [{ role: 'user', content: prompt }], ...(/^gpt-[56](?:\.|-|$)/i.test(model.name) ? { max_completion_tokens: 256 } : { max_tokens: 256 }) } }
}

export function parseTextResponse(result: unknown): string {
  if (!result || typeof result !== 'object') return ''
  const data = result as { choices?: { message?: { content?: unknown } }[]; content?: { type?: string; text?: string }[]; candidates?: { content?: { parts?: { text?: string }[] } }[] }
  const chat = data.choices?.[0]?.message?.content
  if (typeof chat === 'string') return chat
  if (Array.isArray(chat)) return chat.map(part => part?.text || '').join('')
  if (Array.isArray(data.content)) return data.content.filter(part => part.type === 'text').map(part => part.text || '').join('')
  return data.candidates?.[0]?.content?.parts?.map(part => part.text || '').join('') || ''
}
