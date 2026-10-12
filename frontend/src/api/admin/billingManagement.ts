import { apiClient } from '../client'

export type CostCurrency = 'CNY' | 'USD'
export type PurchaseMode = 'payg' | 'subscription' | 'credit_pack' | 'hybrid'
export type CostModality = 'chat' | 'image' | 'video'
export type CostUnit = 'per_million_tokens' | 'per_image' | 'per_second' | 'per_request'

export interface ProcurementPrice {
  operation?: '' | 'alpha_search'
  id: string
  models: string[]
  modality: CostModality
  billing_unit: CostUnit
  resolution: string
  quality: string
  service_tier: string
  min_tokens: number
  max_tokens: number | null
  input_price: number | null
  output_price: number | null
  cache_read_price: number | null
  cache_write_price: number | null
  image_input_price: number | null
  image_output_price: number | null
  unit_price: number | null
  input_image_unit_price?: number | null
}
export interface ProcurementProfile {
  id: string
  enabled: boolean
  name: string
  provider: string
  region?: string
  account_ids: number[]
  purchase_mode: PurchaseMode
  currency: CostCurrency
  discount_multiplier: number
  period_cost: number | null
  credit_amount: number | null
  valid_from: string | null
  valid_until: string | null
  source: string
  notes: string
  prices: ProcurementPrice[]
}
export interface BillingManagementConfig {
  version: number
  simple_billing_enabled?: boolean
  api_sale_multiplier?: number
  usd_to_cny_rate: number
  exchange_rate_source: string
  exchange_rate_effective_at: string | null
  cost_profiles: ProcurementProfile[]
}
export interface BillingPreviewInput {
  operation?: '' | 'alpha_search'
  profile_id?: string
  account_id?: number
  group_id?: number
  model: string
  modality: CostModality
  resolution?: string
  quality?: string
  input_tokens: number
  output_tokens: number
  cache_read_tokens: number
  cache_write_tokens: number
  image_input_tokens?: number
  image_output_tokens?: number
  image_count: number
  duration_seconds: number
  requests: number
  service_tier?: string
  input_image_count?: number
  image_scene?: string
}
export interface BillingCatalogPrice {
  label: string
  unit: CostUnit
  amount_cny: number
  resolution?: string
  quality?: string
  service_tier?: string
  min_tokens?: number
  max_tokens?: number | null
}
export interface BillingCatalogEntry {
  operation?: '' | 'alpha_search'
  key: string
  platform: string
  model: string
  upstream_model: string
  modality: CostModality
  account_ids: number[]
  group_id: number
  route: 'api' | 'account'
  source: 'manual' | 'official' | 'official_reference' | 'reference'
  provider: string
  region: string
  status: 'ready' | 'unknown'
  currency: 'CNY'
  config_version: number
  multiplier: number
  resolution?: string
  base_prices: BillingCatalogPrice[]
  reason: string
}
export interface BillingPreview {
  cost: {
    status: 'estimated' | 'unknown' | 'period_only'
    profile_id: string
    currency: CostCurrency
    amount: number | null
    amount_cny: number | null
    period_cost: number | null
    exchange_rate: number | null
    config_version: number
    reason: string
  }
  sale: { status: 'estimated' | 'unknown'; currency: 'USD'; amount: number | null; amount_cny?: number | null; multiplier: number | null; source: string; reason: string }
  comparison: { income_cny: number | null; margin_cny: number | null; reason: string }
  warnings: string[]
}

export const billingManagementAPI = {
  async catalog(): Promise<BillingCatalogEntry[]> {
    return (await apiClient.get<BillingCatalogEntry[]>('/admin/billing-management/catalog')).data
  },
  async getConfig(): Promise<BillingManagementConfig> {
    return (await apiClient.get<BillingManagementConfig>('/admin/billing-management/config')).data
  },
  async updateConfig(config: BillingManagementConfig): Promise<BillingManagementConfig> {
    return (await apiClient.put<BillingManagementConfig>('/admin/billing-management/config', config)).data
  },
  async preview(input: BillingPreviewInput): Promise<BillingPreview> {
    return (await apiClient.post<BillingPreview>('/admin/billing-management/preview', input)).data
  }
}
