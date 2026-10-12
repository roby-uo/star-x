import type { SubscriptionPlan } from '@/types/payment'
import { USD_TO_CNY_RATE } from '@/utils/currency'

/** Backend supplied cash price avoids treating legacy display-only currency as ledger units. */
export function planPriceCny(plan: SubscriptionPlan, original = false, legacyRate = 0): number {
  const normalized = original ? plan.original_price_cny : plan.price_cny
  if (normalized != null && Number.isFinite(normalized)) return normalized
  const cashPrice = original ? plan.cash_original_price_cny : plan.cash_price_cny
  if (cashPrice != null && Number.isFinite(cashPrice)) return cashPrice
  const amount = original ? plan.original_price ?? 0 : plan.price
  // Historical currency was display-only, so it must never change the cash price.
  return legacyRate > 0 ? amount * legacyRate : amount
}

export function planPaymentAmount(plan: SubscriptionPlan, paymentCurrency: string, legacyRate = 0, original = false): number {
  const cashPrice = original ? plan.cash_original_price_cny : plan.cash_price_cny
  if (paymentCurrency === 'USD') {
    if (cashPrice != null) return cashPrice / USD_TO_CNY_RATE
    // All old plans keep their pre-existing USD checkout price, regardless of display label.
    return original ? plan.original_price ?? 0 : plan.price
  }
  return planPriceCny(plan, original, legacyRate)
}
