import { describe, expect, it } from 'vitest'
import { appendMissingMiniMaxPrices, minimaxChinaCostTemplate } from '../minimaxCostTemplate'
import { emptyPrice, profileIssue, emptyProfile } from '../billingManagement'

describe('MiniMax procurement reference template', () => {
  it('uses complete CNY units and independent standard/priority context bands', () => {
    const prices = minimaxChinaCostTemplate()
    expect(profileIssue({ ...emptyProfile(), name: 'MiniMax', provider: 'MiniMax', prices })).toBe('')
    const low = prices.find(row => row.models.includes('MiniMax-M3') && row.max_tokens === 512000 && row.service_tier === '')!
    const priority = prices.find(row => row.models.includes('MiniMax-M3') && row.max_tokens === 512000 && row.service_tier === 'priority')!
    expect(low.input_price).toBe(2.1)
    expect(priority.input_price).toBeCloseTo(low.input_price! * 1.5)
    const h3 = prices.find(row => row.models.includes('MiniMax-H3') && row.resolution === '768P')!
    expect(h3.unit_price! * 4).toBe(2)
    const max = prices.find(row => row.models.includes('MiniMax-H3-Max') && row.resolution === '480P')!
    expect(max.unit_price! * 5).toBeCloseTo(1.65)
  })
  it('preserves a custom generic cost and adds only uncovered models from shared rows', () => {
    const generic = { ...emptyPrice(), models: ['MiniMax-H3*'], modality: 'video' as const, billing_unit: 'per_second' as const, unit_price: .123 }
    const custom = { ...emptyPrice(), models: ['MiniMax-M2.5'], input_price: 99, output_price: 99, cache_read_price: 0, cache_write_price: 0 }
    const result = appendMissingMiniMaxPrices([generic, custom])
    expect(result[0]).toBe(generic)
    expect(result[1]).toBe(custom)
    expect(result.filter(row => row.models.some(model => model.startsWith('MiniMax-H3')))).toEqual([generic])
    expect(result.filter(row => row.models.includes('MiniMax-M2.5'))).toEqual([custom])
    expect(result.some(row => row.models.includes('MiniMax-M2.1'))).toBe(true)
    expect(appendMissingMiniMaxPrices(result)).toEqual(result)
  })
})
