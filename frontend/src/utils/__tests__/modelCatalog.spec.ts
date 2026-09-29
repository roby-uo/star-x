import { describe, expect, it } from 'vitest'
import { catalogModelEndpoint, catalogModelType, isAudioModel } from '../modelCatalog'

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
    expect(catalogModelEndpoint('Videos', 'openai', 'other-video')).toBe('暂未适配')
  })
})
