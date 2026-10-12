import type { ProcurementPrice } from '@/api/admin/billingManagement'
import { emptyPrice } from './billingManagement'

// Dated reference only; saving and future price updates remain explicit.
export const MINIMAX_COST_TEMPLATE_SOURCE = 'https://platform.minimax.cn/docs/guides/pricing-paygo · 2026-10-12 参考'
export function minimaxChinaCostTemplate(): ProcurementPrice[] {
  const token = (models: string[], input: number, output: number, read: number, write: number, min = 0, max: number | null = null, tier = ''): ProcurementPrice => ({ ...emptyPrice(), models, input_price: input, output_price: output, cache_read_price: read, cache_write_price: write, min_tokens: min, max_tokens: max, service_tier: tier })
  const video = (model: string, resolution: string, price: number): ProcurementPrice => ({ ...emptyPrice(), models: [model], modality: 'video', billing_unit: 'per_second', resolution, unit_price: price })
  return [
    token(['MiniMax-M3'], 2.1, 8.4, .42, 0, 0, 512000),
    token(['MiniMax-M3'], 4.2, 16.8, .84, 0, 512000),
    token(['MiniMax-M3'], 3.15, 12.6, .63, 0, 0, 512000, 'priority'),
    token(['MiniMax-M3'], 6.3, 25.2, 1.26, 0, 512000, null, 'priority'),
    token(['MiniMax-M2.7'], 2.1, 8.4, .42, 2.625),
    token(['MiniMax-M2.7-highspeed'], 4.2, 16.8, .42, 2.625),
    token(['MiniMax-M2.5', 'MiniMax-M2.1', 'MiniMax-M2'], 2.1, 8.4, .21, 2.625),
    token(['MiniMax-M2.5-highspeed', 'MiniMax-M2.1-highspeed'], 4.2, 16.8, .21, 2.625),
    video('MiniMax-H3', '768P', .5), video('MiniMax-H3', '2K', .8),
    video('MiniMax-H3-Max', '480P', .33), video('MiniMax-H3-Max', '768P', .5)
  ]
}
export function appendMissingMiniMaxPrices(existing: ProcurementPrice[]): ProcurementPrice[] {
  const conflicts = (row: ProcurementPrice, other: ProcurementPrice, model: string) => row.modality === other.modality && (!other.resolution || row.resolution.toLowerCase() === other.resolution.toLowerCase()) && (!other.quality || row.quality.toLowerCase() === other.quality.toLowerCase()) && (row.service_tier || '') === (other.service_tier || '') && other.models.some(pattern => pattern.endsWith('*') ? model.toLowerCase().startsWith(pattern.slice(0, -1).toLowerCase()) : pattern.toLowerCase() === model.toLowerCase()) && (row.max_tokens == null || other.min_tokens < row.max_tokens) && (other.max_tokens == null || row.min_tokens < other.max_tokens)
  const missing = minimaxChinaCostTemplate().map(row => ({ ...row, models: row.models.filter(model => !existing.some(other => conflicts(row, other, model))) })).filter(row => row.models.length)
  return [...existing, ...missing]
}
