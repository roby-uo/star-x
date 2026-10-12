import { describe, expect, it } from 'vitest'
import type { SubscriptionPlan } from '@/types/payment'
import { planPaymentAmount, planPriceCny } from '../planPricing'

const plan: SubscriptionPlan = {
  id: 1, group_id: 1, name: 'Example', description: '', price: 100,
  validity_days: 30, validity_unit: 'days', features: [], for_sale: true, sort_order: 0,
}

describe('subscription CNY checkout price', () => {
  it('uses the backend cash price once even for legacy display labels', () => {
    expect(planPriceCny({ ...plan, currency: 'USD', price_cny: 100 })).toBe(100)
    expect(planPriceCny({ ...plan, currency: 'CNY' })).toBe(100)
    expect(planPriceCny({ ...plan, currency: 'USD' })).toBe(100)
    expect(planPriceCny({ ...plan, currency: 'USD', cash_price_cny: 620 })).toBe(620)
  })

  it('keeps legacy empty-currency checkout behavior', () => {
    expect(planPriceCny(plan, false, 0)).toBe(100)
    expect(planPriceCny(plan, false, 7)).toBe(700)
    expect(planPaymentAmount(plan, 'USD', 7)).toBe(100)
  })

  it('converts a native CNY plan only when the actual payment channel is USD', () => {
    expect(planPaymentAmount({ ...plan, currency: 'CNY', cash_price_cny: 100 }, 'CNY', 7)).toBe(100)
    expect(planPaymentAmount({ ...plan, currency: 'CNY', cash_price_cny: 100 }, 'USD', 7)).toBeCloseTo(100 / 6.2, 12)
  })
})
