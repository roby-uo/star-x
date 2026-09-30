import { describe, expect, it } from 'vitest'
import { catalogModelEndpoint, catalogModelProvider, catalogModelType, isAudioModel } from '../modelCatalog'

describe('model catalog classification', () => {
  it('recognizes configured image models and MiniMax H3 videos', () => {
    expect(catalogModelType('doubao-seedream-5-0-pro-260628')).toBe('Images')
    expect(catalogModelType('custom-image', 'image')).toBe('Images')
    expect(catalogModelType('MiniMax-H3')).toBe('Videos')
    expect(catalogModelType('MiniMax-H3-Max')).toBe('Videos')
  })

  it('keeps ordinary models in the available chat category', () => {
    expect(catalogModelType('deepseek-flash')).toBe('Chat')
    expect(isAudioModel('gpt-4o-audio-preview')).toBe(true)
    expect(catalogModelEndpoint('Images')).toBe('/v1/images/generations')
    expect(catalogModelEndpoint('Videos', 'grok')).toBe('/v1/videos/generations')
    expect(catalogModelEndpoint('Videos', 'openai', 'MiniMax-H3')).toBe('/v2/video_generation')
    expect(catalogModelEndpoint('Videos', 'openai', 'MiniMax-H3-Max')).toBe('/v2/video_generation')
    expect(catalogModelEndpoint('Videos', 'openai', 'MiniMax-H3-Max-preview')).toBe('暂未适配')
    expect(catalogModelEndpoint('Videos', 'anthropic', 'MiniMax-H3-Max')).toBe('暂未适配')
    expect(catalogModelEndpoint('Videos', 'openai', 'other-video')).toBe('暂未适配')
  })

  it('uses native text request endpoints for each protocol', () => {
    expect(catalogModelEndpoint('Chat', 'anthropic', 'claude-sonnet')).toBe('/v1/messages')
    expect(catalogModelEndpoint('Chat', 'gemini', 'gemini-2.5-pro')).toBe('/v1beta/models/gemini-2.5-pro:generateContent')
    expect(catalogModelEndpoint('Chat', 'antigravity', 'claude-sonnet')).toBe('/v1/messages')
    expect(catalogModelEndpoint('Chat', 'antigravity', 'gemini-2.5-pro')).toBe('/v1beta/models/gemini-2.5-pro:generateContent')
    expect(catalogModelEndpoint('Chat', 'openai', 'deepseek-flash')).toBe('/v1/chat/completions')
  })

  it('keeps model family separate from its compatible protocol', () => {
    expect(catalogModelProvider('MiniMax-H3', 'openai')).toBe('MiniMax')
    expect(catalogModelProvider('doubao-seedream-5-0-pro-260628', 'openai')).toBe('火山引擎')
    expect(catalogModelProvider('deepseek-flash', 'openai')).toBe('DeepSeek')
    expect(catalogModelProvider('custom', 'openai')).toBe('兼容模型')
  })
})
