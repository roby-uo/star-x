import { describe, expect, it } from 'vitest'
import { miniMaxVideoSpec } from '../minimaxVideo'
import { defaultModelPolicy } from '@/types/modelService'

describe('MiniMax video policy defaults', () => {
  it('keeps H3 and H3-Max resolution, duration and price defaults separate', () => {
    const standard = defaultModelPolicy('MiniMax-H3', 'openai', 'video')
    const max = defaultModelPolicy('MiniMax-H3-Max', 'openai', 'video')
    expect(standard).toMatchObject({ resolutions: ['768P', '2K'], min_duration: 4, max_duration: 15, prices: { '768P': 0.08, '2K': 0.13 } })
    expect(max).toMatchObject({ resolutions: ['480P', '768P'], min_duration: 5, max_duration: 15, prices: { '480P': 0.05, '768P': 0.08 } })
    expect(max.modes).toEqual(['text', 'first_frame', 'first_last_frame'])
    expect(max.prices).not.toHaveProperty('2K')
  })

  it('does not infer adapter support from a similar name or another platform', () => {
    for (const name of ['MiniMax-H3-Max-preview', 'minimax-h3-max', 'MiniMax-H3-Context-IR', 'other-video']) expect(miniMaxVideoSpec(name)).toBeUndefined()
    expect(defaultModelPolicy('MiniMax-H3-Max', 'anthropic', 'video').resolutions).toBeUndefined()
    expect(defaultModelPolicy('other-video', 'openai', 'video').resolutions).toBeUndefined()
  })
})
