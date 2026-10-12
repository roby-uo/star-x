import type { Account } from '@/types'
import type { BillingManagementConfig, ProcurementPrice, ProcurementProfile } from '@/api/admin/billingManagement'
import { USD_TO_CNY_RATE } from '@/utils/currency'

export const purchaseLabels = { payg: '按量付费', subscription: '包月 / 订阅', credit_pack: '预付额度包', hybrid: '固定费 + 按量' }
export const unitLabels = { per_million_tokens: '百万 tokens', per_image: '张', per_second: '秒', per_request: '次' }
export const modalityLabels = { chat: '文本', image: '图像', video: '视频' }
export const newID = () => crypto.randomUUID()
export function emptyPrice(): ProcurementPrice {
  return { id: newID(), models: [], modality: 'chat', billing_unit: 'per_million_tokens', resolution: '', quality: '', service_tier: '', min_tokens: 0, max_tokens: null, input_price: null, output_price: null, cache_read_price: null, cache_write_price: null, image_input_price: null, image_output_price: null, unit_price: null }
}
export function emptyProfile(): ProcurementProfile {
  return { id: newID(), enabled: true, name: '', provider: '', account_ids: [], purchase_mode: 'payg', currency: 'CNY', discount_multiplier: 1, period_cost: null, credit_amount: null, valid_from: null, valid_until: null, source: '', notes: '', prices: [] }
}
export function profileIssue(profile: ProcurementProfile): string {
  if (!profile.name.trim() || !profile.provider.trim()) return '请填写采购方案名称和供应商。'
  if (!Number.isFinite(profile.discount_multiplier) || profile.discount_multiplier <= 0) return '采购折扣必须大于 0；原价填写 1。'
  if (profile.valid_from && profile.valid_until && new Date(profile.valid_until) <= new Date(profile.valid_from)) return '结束时间须晚于开始时间。'
  if (profile.purchase_mode === 'hybrid' && (profile.period_cost == null || !Number.isFinite(profile.period_cost) || profile.period_cost < 0)) return '请填写有效的周期实付费用。'
  if (profile.period_cost != null && (!Number.isFinite(profile.period_cost) || profile.period_cost < 0)) return '周期实付费用须为非负数。'
  if (profile.purchase_mode === 'credit_pack' && (profile.period_cost == null || profile.period_cost <= 0 || profile.credit_amount == null || profile.credit_amount <= 0)) return '额度包须填写实付金额和供应商额度。'
  for (const row of profile.prices) {
    if (!row.models.length || row.models.some(model => !model.trim())) return '每条成本规则至少填写一个模型。'
    if (!Number.isInteger(row.min_tokens) || row.min_tokens < 0 || (row.max_tokens != null && (!Number.isInteger(row.max_tokens) || row.max_tokens <= row.min_tokens))) return '输入用量区间须为有效整数，且上限大于下限。'
    const values = row.billing_unit === 'per_million_tokens' ? [row.input_price, row.output_price, row.cache_read_price, row.cache_write_price] : [row.unit_price]
    if (values.some(value => value == null || !Number.isFinite(value) || value < 0)) return '请完整填写成本单价；不收费的项目明确填写 0。'
  }
  return ''
}
export function profileState(profile: ProcurementProfile, now = Date.now()): string {
  if (!profile.enabled) return '停用'
  if (profile.valid_until && new Date(profile.valid_until).getTime() <= now) return '已过期'
  if (profile.valid_from && new Date(profile.valid_from).getTime() > now) return '待生效'
  if (!profile.account_ids.length) return '模板'
  return '生效中'
}
export function profileAccounts(profile: ProcurementProfile, accounts: Account[]): string {
  return profile.account_ids.map(id => accounts.find(account => account.id === id)?.name || `账号 #${id}`).join('、')
}
export function costCoverage(config: BillingManagementConfig, accounts: Account[]): number {
  const covered = new Set(config.cost_profiles.filter(profile => profileState(profile) === '生效中').flatMap(profile => profile.account_ids))
  return accounts.filter(account => covered.has(account.id)).length
}
export function money(value: number | null | undefined, currency = 'CNY'): string {
  if (value == null || !Number.isFinite(value)) return '待配置'
  if (!['CNY', 'USD'].includes(currency)) return '待核对'
  const amount = currency === 'USD' ? value * USD_TO_CNY_RATE : value
  const digits = amount !== 0 && Math.abs(amount) < 0.0000005 ? Math.min(12, Math.max(6, Math.ceil(-Math.log10(Math.abs(amount))) + 2)) : 6
  return new Intl.NumberFormat('zh-CN', { style: 'currency', currency: 'CNY', maximumFractionDigits: digits }).format(amount)
}

export function procurementToCny(value: number | null | undefined, currency: string): number | null {
  return value == null ? null : value * (currency === 'USD' ? USD_TO_CNY_RATE : 1)
}
export function procurementFromCny(value: number | null, currency: string): number | null {
  return value == null ? null : value / (currency === 'USD' ? USD_TO_CNY_RATE : 1)
}
export function changeProfileCurrency(profile: ProcurementProfile, previous: string): void {
  const factor = (previous === 'USD' ? USD_TO_CNY_RATE : 1) / (profile.currency === 'USD' ? USD_TO_CNY_RATE : 1)
  for (const field of ['period_cost', 'credit_amount'] as const) if (profile[field] != null) profile[field] *= factor
  for (const row of profile.prices) for (const field of ['input_price', 'output_price', 'cache_read_price', 'cache_write_price', 'image_input_price', 'image_output_price', 'unit_price', 'input_image_unit_price'] as const) if (row[field] != null) row[field] *= factor
}
