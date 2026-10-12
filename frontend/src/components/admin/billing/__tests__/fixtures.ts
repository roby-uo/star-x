import type { Account, AdminGroup } from '@/types'
import type { BillingManagementConfig, BillingPreview, ProcurementProfile } from '@/api/admin/billingManagement'

export function profile(overrides: Partial<ProcurementProfile> = {}): ProcurementProfile {
  return {
    id: 'cost-example', enabled: true, name: 'API 采购', provider: 'MiniMax 中国站', account_ids: [1], purchase_mode: 'payg', currency: 'CNY', discount_multiplier: 1,
    period_cost: null, credit_amount: null, valid_from: null, valid_until: null, source: '合同 001', notes: '',
    prices: [{ id: 'price-example', models: ['MiniMax-M3'], modality: 'chat', billing_unit: 'per_million_tokens', resolution: '', quality: '', service_tier: '', min_tokens: 0, max_tokens: null, input_price: 2, output_price: 8, cache_read_price: 0, cache_write_price: 0, image_input_price: null, image_output_price: null, unit_price: null }],
    ...overrides,
  }
}

export function account(id = 1, type = 'apikey'): Account {
  return { id, name: `上游 ${id}`, platform: 'openai', type, status: 'active', group_ids: [7], configured_models: { 'MiniMax-M3': 'MiniMax-M3' } } as Account
}

export function group(): AdminGroup {
  return { id: 7, name: '普通用户', platform: 'openai', status: 'active', rate_multiplier: 1.4 } as AdminGroup
}

export function config(overrides: Partial<BillingManagementConfig> = {}): BillingManagementConfig {
  return { version: 4, simple_billing_enabled: true, api_sale_multiplier: 1.4, usd_to_cny_rate: 6.2, exchange_rate_source: '固定站内换算', exchange_rate_effective_at: '2026-10-01T00:00:00Z', cost_profiles: [profile()], ...overrides }
}

export function preview(overrides: Partial<BillingPreview> = {}): BillingPreview {
  return {
    cost: { status: 'estimated', profile_id: 'cost-example', currency: 'CNY', amount: 2, amount_cny: 2, period_cost: null, exchange_rate: null, config_version: 4, reason: '按采购单价估算' },
    sale: { status: 'estimated', currency: 'USD', amount: 0.448, multiplier: 1.4, source: 'model_service', reason: '按已保存售价计算' },
    comparison: { income_cny: null, margin_cny: null, reason: '额度扣费不等于充值现金收入' }, warnings: [],
    ...overrides,
  }
}

export function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason: unknown) => void
  const promise = new Promise<T>((ok, fail) => { resolve = ok; reject = fail })
  return { promise, resolve, reject }
}
