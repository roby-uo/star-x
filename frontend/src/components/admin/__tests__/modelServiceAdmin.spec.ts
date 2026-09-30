import { describe, expect, it } from 'vitest'
import type { Channel, ChannelModelPricing } from '@/api/admin/channels'
import { configuredModelOpen, mergeModelPolicy, mergeModelPricing, modelPolicy, modelScopeSnapshot } from '../modelServiceAdmin'

const price = (models: string[], platform = 'openai'): ChannelModelPricing => ({
  id: 1, models, platform, billing_mode: 'token', input_price: 0.000001, output_price: 0.000002,
  cache_write_price: null, cache_read_price: null, image_input_price: null, image_output_price: null,
  per_request_price: null, intervals: [{ min_tokens: 0, max_tokens: 100, tier_label: '', input_price: 0.000003,
    output_price: null, cache_write_price: null, cache_read_price: null, per_request_price: null, sort_order: 0 }]
})
const channel = (): Channel => ({ id: 8, group_ids: [2, 1], status: 'active', model_pricing: [price(['alpha', 'beta'])],
  features_config: { other_flag: true, model_services: [{ model: 'ALPHA', platform: 'openai', kind: 'chat', state: 'published', groups: { '1': { enabled: true } } },
    { model: 'alpha', platform: 'anthropic', kind: 'chat', state: 'paused' }] } } as unknown as Channel)

describe('model service admin updates', () => {
  it('splits shared prices without changing other models, protocols, or intervals', () => {
    const shared = price(['alpha', 'beta'])
    const unrelated = price(['alpha'], 'anthropic')
    const input = [shared, unrelated]
    const next = mergeModelPricing(input, 'ALPHA', 'openai', { ...shared, input_price: 0.000009 })
    expect(next[0]).toEqual({ ...shared, models: ['beta'] })
    expect(next[1]).toBe(unrelated)
    expect(next[2]).toMatchObject({ models: ['ALPHA'], input_price: 0.000009, intervals: shared.intervals })
    expect(next[2]).not.toHaveProperty('id')
    expect(input[0]?.models).toEqual(['alpha', 'beta'])
  })

  it('preserves other configuration and policies when updating a published service', () => {
    const current = channel()
    const target = modelPolicy(current, 'alpha', 'openai')!
    const merged = mergeModelPolicy(current, { ...target, max_images: 3 })
    expect(merged.other_flag).toBe(true)
    expect(merged.model_services).toEqual([
      { model: 'alpha', platform: 'anthropic', kind: 'chat', state: 'paused' },
      { ...target, max_images: 3, state: 'published' }
    ])
  })

  it('detects concurrent scope changes while ignoring unrelated model changes', () => {
    const current = channel()
    const original = modelScopeSnapshot(current, 'alpha', 'openai')
    current.group_ids.reverse()
    current.model_pricing.push(price(['gamma']))
    expect(modelScopeSnapshot(current, 'alpha', 'openai')).toBe(original)
    current.model_pricing[0]!.input_price = 1
    expect(modelScopeSnapshot(current, 'alpha', 'openai')).not.toBe(original)
  })

  it('removes only the chosen model price when reverting to legacy pricing', () => {
    const current = channel()
    expect(mergeModelPricing(current.model_pricing, 'alpha', 'openai', null)).toEqual([
      { ...current.model_pricing[0], models: ['beta'] }
    ])
  })

  it('honors policy state and group authorization without treating legacy configuration as a draft', () => {
    const current = channel()
    expect(configuredModelOpen(current, 'alpha', 'openai', 1)).toBe(true)
    expect(configuredModelOpen(current, 'alpha', 'openai', 2)).toBe(false)
    expect(configuredModelOpen(current, 'gamma', 'openai', 1)).toBe(true)
    current.status = 'disabled'
    expect(configuredModelOpen(current, 'alpha', 'openai', 1)).toBe(false)
    expect(configuredModelOpen(current, 'gamma', 'openai', 1)).toBe(true)
  })

  it('checks exact, wildcard, mapped and upstream prices for restricted legacy channels', () => {
    const current = channel()
    current.features_config = {}
    current.restrict_models = true
    current.billing_model_source = 'requested'
    expect(configuredModelOpen(current, 'alpha', 'openai', 1)).toBe(true)
    expect(configuredModelOpen(current, 'gamma', 'openai', 1)).toBe(false)
    current.model_pricing.push(price(['gamma-*']))
    expect(configuredModelOpen(current, 'gamma-pro', 'openai', 1)).toBe(true)
    current.billing_model_source = 'channel_mapped'
    current.model_mapping = { openai: { 'external': 'alpha' } }
    expect(configuredModelOpen(current, 'external', 'openai', 1)).toBe(true)
    current.billing_model_source = 'upstream'
    expect(configuredModelOpen(current, 'external', 'openai', 1, 'beta')).toBe(true)
    expect(configuredModelOpen(current, 'external', 'openai', 1, 'missing')).toBe(false)
  })
})
