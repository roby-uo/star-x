import { describe, expect, it } from 'vitest'
import type { UserAvailableModel } from '@/api/channels'
import { aggregateUserModels, buildTextRequest, parseTextResponse } from '../userModelCatalog'

const model = (group_id: number, platform = 'openai', name = 'shared-model'): UserAvailableModel => ({ name, platform, group_id, group_name: `Group ${group_id}`, rate_multiplier: 1 })

describe('user model catalog', () => {
  it('merges group visibility without losing group-specific policies or mixing protocols', () => {
    const standard = { ...model(1, 'openai', 'MiniMax-H3'), policy: { platform: 'openai', model: 'MiniMax-H3', kind: 'video' as const, state: 'published' as const, resolutions: ['768P'] } }
    const vip = { ...standard, group_id: 2, group_name: 'VIP', policy: { ...standard.policy, resolutions: ['768P', '2K'] } }
    const items = aggregateUserModels([standard, vip, model(3, 'anthropic', 'MiniMax-H3'), standard])
    expect(items).toHaveLength(2)
    expect(items.find(item => item.platform === 'openai')?.groups).toEqual([standard, vip])
    expect(items.find(item => item.platform === 'openai')?.type).toBe('Videos')
  })

  it('classifies custom image policy names and removes audio', () => {
    expect(aggregateUserModels([{ ...model(1), policy: { platform: 'openai', model: 'shared-model', kind: 'image', state: 'published' } }, model(1, 'openai', 'tts-1')]).map(item => item.type)).toEqual(['Images'])
  })

  it('uses the selected platform native request and extracts its response', () => {
    expect(buildTextRequest(model(1, 'anthropic'), '你好')).toMatchObject({ path: '/v1/messages', body: { max_tokens: 256 } })
    expect(buildTextRequest(model(1, 'gemini', 'gemini-3'), '你好')).toMatchObject({ path: '/v1beta/models/gemini-3:generateContent' })
    expect(buildTextRequest(model(1, 'openai', 'gpt-5.5'), '你好').body).toHaveProperty('max_completion_tokens', 256)
    expect(parseTextResponse({ choices: [{ message: { content: 'Chat answer' } }] })).toBe('Chat answer')
    expect(parseTextResponse({ content: [{ type: 'text', text: 'Claude answer' }] })).toBe('Claude answer')
    expect(parseTextResponse({ candidates: [{ content: { parts: [{ text: 'Gemini answer' }] } }] })).toBe('Gemini answer')
  })
})
