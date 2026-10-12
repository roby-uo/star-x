<template>
  <AppLayout>
    <div class="mx-auto max-w-7xl space-y-5">
      <AdminWorkspaceTabs area="billing" />
      <header class="flex flex-wrap items-start justify-between gap-3">
        <div><h2 class="text-xl font-semibold text-gray-900 dark:text-white">售价规则</h2><p class="mt-1 text-sm text-gray-500">统一调整销售倍率，模型使用匹配的采购基准价。</p></div>
        <button type="button" class="btn btn-secondary" :disabled="loading || saving" aria-label="刷新计费配置" @click="load"><Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" /></button>
      </header>
      <p v-if="error" role="alert" class="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:bg-red-950/20">{{ error }}</p>
      <p v-if="notice" role="status" class="text-sm text-emerald-600">{{ notice }}</p>
      <div class="flex flex-wrap gap-x-7 gap-y-2 rounded-xl bg-primary-50 px-5 py-3 text-sm dark:bg-primary-950/30">
        <span class="font-medium text-primary-700 dark:text-primary-300">统一人民币</span><span class="text-gray-600 dark:text-gray-300">美元固定换算 <strong>6.2</strong></span><span class="text-gray-500">人民币原价保持不变</span>
      </div>
      <p v-if="loading" class="py-16 text-center text-sm text-gray-500">正在加载计费配置…</p>
      <template v-else-if="config">
        <p v-if="config.simple_billing_enabled === false" class="rounded-xl bg-amber-50 px-5 py-3 text-sm text-amber-800 dark:bg-amber-950/30 dark:text-amber-300">当前使用旧计费规则。保存 API 倍率后启用统一规则；价格未确认的按量上游需先补齐基准价。</p>
        <div class="grid gap-4 lg:grid-cols-2">
          <form class="rounded-xl border border-gray-200 bg-white p-5 dark:border-dark-700 dark:bg-dark-800" @submit.prevent="saveApiRate">
            <h3 class="font-semibold text-gray-900 dark:text-white">按量 API</h3>
            <div class="mt-4 flex flex-wrap items-center justify-between gap-3"><label for="api-sale-multiplier" class="text-sm text-gray-600 dark:text-gray-300">统一售价倍率</label><div class="flex items-center gap-2"><input id="api-sale-multiplier" v-model.number="apiMultiplier" required type="number" min="0.000001" step="any" class="input w-24" /><span class="text-sm text-gray-500">×</span><button type="submit" class="btn btn-primary" :disabled="saving || !apiRateChanged">{{ saving ? '保存中…' : '保存' }}</button></div></div>
            <p class="mt-3 text-xs text-gray-500">采购基准价 × 统一倍率；不叠加分组或独立媒体倍率。</p>
          </form>
          <form class="rounded-xl border border-gray-200 bg-white p-5 dark:border-dark-700 dark:bg-dark-800" @submit.prevent="saveGroupRate">
            <h3 class="font-semibold text-gray-900 dark:text-white">账号授权 / 订阅</h3>
            <div class="mt-4 space-y-3"><div class="flex items-center justify-between gap-3"><label for="billing-group" class="shrink-0 text-sm text-gray-600 dark:text-gray-300">分组</label><select id="billing-group" v-model.number="selectedGroupID" class="input w-full max-w-60" aria-label="选择账号计费分组"><option v-for="group in groups" :key="group.id" :value="group.id">{{ group.name }}</option></select></div><div class="flex items-center justify-between gap-3"><label for="account-group-multiplier" class="text-sm text-gray-600 dark:text-gray-300">售价倍率</label><div class="flex shrink-0 items-center gap-2"><input id="account-group-multiplier" v-model.number="groupMultiplier" type="number" min="0" step="any" required class="input w-24" aria-label="账号分组倍率" /><span class="text-sm text-gray-500">×</span><button type="submit" class="btn btn-secondary" :disabled="saving || !groupRateChanged">保存</button></div></div></div>
            <p class="mt-3 text-xs text-gray-500">参考基准价 × 所选分组倍率；包月实付独立记录。</p>
          </form>
        </div>
        <details :open="!!route.query.model || route.query.tab === 'prices'" class="rounded-xl border border-gray-200 bg-white p-5 dark:border-dark-700 dark:bg-dark-800">
          <summary class="cursor-pointer font-medium">按模型核对价格 <span class="ml-2 text-xs font-normal text-gray-500">{{ models.length }} 个模型</span></summary>
          <section class="mt-4 space-y-3" aria-label="模型基准价">
          <div class="flex flex-wrap items-center justify-between gap-3"><p class="text-xs text-gray-500">核对具体上游、规格与最终扣费</p><div class="flex flex-wrap gap-2"><input v-model="query" class="input w-full sm:w-64" placeholder="搜索模型或供应商" aria-label="搜索费率模型" /><select v-model="modality" class="input w-32" aria-label="费率模型类型"><option value="">全部类型</option><option value="Chat">文本</option><option value="Images">图像</option><option value="Videos">视频</option></select></div></div>
          <div class="overflow-x-auto rounded-xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
            <table class="w-full min-w-[880px] text-left text-sm"><thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-900"><tr><th class="px-5 py-3">模型与类型</th><th class="px-5 py-3">价格来源</th><th class="px-5 py-3">人民币基准价</th><th class="px-5 py-3">售价</th><th class="px-5 py-3 text-right">规格与试算</th></tr></thead><tbody>
              <tr v-for="row in filteredModels" :key="row.key" class="border-t border-gray-100 dark:border-dark-700"><td class="px-5 py-4"><p class="font-medium">{{ row.model }}</p><p class="mt-1 text-xs text-gray-500">{{ typeLabel(row.kind) }} · {{ row.provider }}</p></td><td class="px-5 py-4"><p class="text-xs" :class="catalogEntries(row).some(entry => entry.status !== 'ready') ? 'text-amber-600' : 'text-gray-600 dark:text-gray-300'">{{ sourceLabel(row) }}</p><p class="mt-1 text-xs text-gray-400">{{ row.accounts.length }} 个上游来源</p><details v-if="catalogEntries(row).some(entry => entry.status !== 'ready')" class="mt-2 max-w-52 text-xs text-gray-500"><summary class="cursor-pointer">查看待确认原因</summary><p v-for="reason in [...new Set(catalogEntries(row).filter(entry => entry.status !== 'ready').map(entry => entry.reason))]" :key="reason" class="mt-2">{{ reason }}</p></details></td><td class="max-w-xs px-5 py-4 text-xs">{{ priceLabel(row) }}</td><td class="max-w-xs px-5 py-4"><p class="text-xs text-primary-700 dark:text-primary-300">{{ priceLabel(row, true) }}</p><p class="mt-1 text-xs text-gray-400">{{ rateLabel(row) }}</p></td><td class="px-5 py-4 text-right"><button type="button" class="btn btn-secondary whitespace-nowrap text-xs" :aria-label="`查看 ${row.model} 规格与最终扣费`" @click="openPreview(row.model)">查看与试算</button></td></tr>
              <tr v-if="!filteredModels.length"><td colspan="5" class="px-5 py-12 text-center text-gray-500">{{ models.length ? '没有符合条件的模型。' : '先在上游管理登记模型。' }}</td></tr>
            </tbody></table>
          </div>
          <p class="text-xs text-gray-500">按供应商、地区、模型与规格匹配。模型名不能代表第三方采购价；最终价格以选定上游的预览为准。</p>
          </section>
        </details>
        <details :open="route.query.tab === 'costs'" class="rounded-xl border border-gray-200 bg-white p-5 dark:border-dark-700 dark:bg-dark-800">
          <summary class="cursor-pointer font-medium">价格来源与特殊供应商 <span class="ml-2 text-xs font-normal text-gray-500">{{ config.cost_profiles.length }} 份配置</span></summary>
          <div class="mt-4 space-y-3"><div class="flex flex-wrap items-center justify-between gap-3"><p class="text-xs text-gray-500">官方参考价自动匹配，第三方单价或合同折扣在这里补充。</p><button type="button" class="btn btn-secondary" :disabled="saving" @click="openCost()">添加基准价</button></div>
            <p v-if="!config.cost_profiles.length" class="rounded-lg bg-gray-50 p-4 text-sm text-gray-500 dark:bg-dark-900">暂无人工采购配置。无法匹配可靠价格的按量模型需先补齐单价。</p>
            <article v-for="profile in config.cost_profiles" :key="profile.id" class="flex flex-wrap items-center justify-between gap-3 border-t border-gray-100 py-4 dark:border-dark-700"><div class="min-w-0"><h4 class="text-sm font-medium">{{ profile.name }} <span class="ml-2 text-xs font-normal text-gray-500">{{ profileState(profile) }}</span></h4><p class="mt-1 text-xs text-gray-500">{{ profile.provider }} · {{ purchaseLabels[profile.purchase_mode] }} · {{ profile.prices.length }} 条规则</p><p class="mt-1 text-xs text-gray-400">{{ profileAccounts(profile, accounts) || '未关联 · 参考模板' }}<span v-if="profile.period_cost != null"> · 周期实付 {{ money(profile.period_cost, profile.currency) }}</span></p></div><div class="flex flex-wrap gap-2"><button type="button" class="btn btn-secondary text-xs" @click="openPreview(undefined, profile.id)">试算</button><button type="button" class="btn btn-secondary text-xs" @click="openCost(profile)">编辑</button><button type="button" class="text-xs text-gray-400 hover:text-red-600" :disabled="saving" @click="deleting = profile">删除</button></div></article>
          </div>
        </details>
        <details class="text-sm text-gray-500"><summary class="cursor-pointer">余额与充值</summary><div class="mt-3 space-y-2 text-xs"><p>新人民币充值按实付入账：支付 ¥100，到账 ¥100，另行配置赠送除外。旧余额、冻结金额和旧额度按固定 6.2 换算，保留原有购买力。</p><p>用户端显示余额和用量，隐藏采购成本、倍率和费用明细。</p><router-link to="/admin/settings?tab=payment" class="inline-block text-primary-600">管理支付与充值 →</router-link><router-link to="/admin/redeem" class="ml-4 inline-block text-primary-600">管理兑换码 →</router-link></div></details>
      </template>
    </div>
    <CostProfileEditor v-if="costEditor" :existing="costEditor.existing" :accounts="accounts" :saving="saving" :error="editorError" @close="!saving && (costEditor = null)" @save="saveCost" />
    <BillingPreviewDialog v-if="preview" :model="preview.model" :profile="preview.profile" :accounts="accounts" :groups="groups" :profiles="config?.cost_profiles || []" :models="models.map(row => row.model)" @close="preview = null" />
    <ConfirmDialog :show="!!deleting" title="删除基准价配置" :message="`删除 ${deleting?.name || ''} 后，无法匹配其他可靠价格的按量上游将需要重新确认单价。`" danger :confirm-text="saving ? '删除中…' : '确认删除'" @cancel="!saving && (deleting = null)" @confirm="deleteCost" />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import AdminWorkspaceTabs from '@/components/admin/AdminWorkspaceTabs.vue'
import Icon from '@/components/icons/Icon.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import CostProfileEditor from '@/components/admin/billing/CostProfileEditor.vue'
import BillingPreviewDialog from '@/components/admin/billing/BillingPreviewDialog.vue'
import { billingManagementAPI, type BillingCatalogEntry, type BillingManagementConfig, type ProcurementProfile } from '@/api/admin/billingManagement'
import channelsAPI, { type Channel } from '@/api/admin/channels'
import { accountsAPI } from '@/api/admin/accounts'
import { getAllIncludingInactive, update as updateGroup } from '@/api/admin/groups'
import type { Account, AdminGroup } from '@/types'
import type { ModelServicePolicy } from '@/types/modelService'
import { catalogModelProvider, catalogModelType, isAudioModel, type CatalogModelType } from '@/utils/modelCatalog'
import { modelPolicy, modelPricing, wildcardPricing } from '@/components/admin/modelServiceAdmin'
import { money, profileAccounts, profileState, purchaseLabels, unitLabels } from '@/components/admin/billing/billingManagement'
import { extractApiErrorMessage } from '@/utils/apiError'
type SaleRow = { key: string; model: string; platform: string; kind: CatalogModelType; provider: string; accounts: Account[] }
const route = useRoute()
const query = ref(typeof route.query.model === 'string' ? route.query.model : '')
const modality = ref('')
const config = ref<BillingManagementConfig | null>(null)
const accounts = ref<Account[]>([])
const groups = ref<AdminGroup[]>([])
const channels = ref<Channel[]>([])
const catalog = ref<BillingCatalogEntry[]>([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const notice = ref('')
const editorError = ref('')
const costEditor = ref<{ existing?: ProcurementProfile } | null>(null)
const deleting = ref<ProcurementProfile | null>(null)
const preview = ref<{ model?: string; profile?: string } | null>(null)
const apiMultiplier = ref(1.4)
const selectedGroupID = ref<number>()
const groupMultiplier = ref(1)
const selectedGroup = computed(() => groups.value.find(group => group.id === selectedGroupID.value))
watch(selectedGroup, group => { groupMultiplier.value = group?.rate_multiplier ?? 1 }, { immediate: true })
const apiRateChanged = computed(() => !!config.value && (config.value.simple_billing_enabled === false || apiMultiplier.value !== (config.value.api_sale_multiplier ?? 1.4)))
const groupRateChanged = computed(() => !!selectedGroup.value && groupMultiplier.value !== selectedGroup.value.rate_multiplier)
const models = computed<SaleRow[]>(() => {
  const inventory = new Map<string, SaleRow>()
  const add = (platform: string, model: string, account?: Account, kind?: string) => {
    if (!model || model.includes('*') || isAudioModel(model)) return
    const key = `${platform}:${model}`
    const row = inventory.get(key) || { key, model, platform, kind: catalogModelType(model, kind), provider: catalogModelProvider(model, platform), accounts: [] }
    if (account && !row.accounts.some(item => item.id === account.id)) row.accounts.push(account)
    if (kind) row.kind = catalogModelType(model, kind)
    inventory.set(key, row)
  }
  for (const account of accounts.value) for (const model of Object.keys(account.configured_models || {})) add(account.platform, model, account)
  for (const channel of channels.value) {
    for (const price of channel.model_pricing) for (const model of price.models) add(price.platform, model, undefined, price.billing_mode)
    for (const policy of (channel.features_config?.model_services as ModelServicePolicy[] | undefined) || []) add(policy.platform, policy.model, undefined, policy.kind)
  }
  return [...inventory.values()].sort((a, b) => a.model.localeCompare(b.model))
})
const filteredModels = computed(() => models.value.filter(row => (!modality.value || row.kind === modality.value) && `${row.model} ${row.provider}`.toLowerCase().includes(query.value.toLowerCase())))
const typeLabel = (kind: CatalogModelType) => ({ Chat: '文本', Images: '图像', Videos: '视频' })[kind]
function matches(pattern: string, model: string) { return pattern.endsWith('*') ? model.toLowerCase().startsWith(pattern.slice(0, -1).toLowerCase()) : pattern.toLowerCase() === model.toLowerCase() }
function matchingProfiles(row: SaleRow) { return (config.value?.cost_profiles || []).filter(profile => profileState(profile) === '生效中' && profile.account_ids.some(id => row.accounts.some(account => account.id === id)) && profile.prices.some(price => price.models.some(pattern => matches(pattern, row.model)))) }
function catalogEntries(row: SaleRow) { return catalog.value.filter(entry => !entry.operation && entry.platform === row.platform && entry.model === row.model && (!selectedGroupID.value || entry.group_id === selectedGroupID.value)) }
function sourceLabel(row: SaleRow) {
  const entries = catalogEntries(row)
  if (entries.length) {
    if (entries.every(entry => entry.status !== 'ready')) return '价格待确认'
    if (entries.some(entry => entry.status !== 'ready')) return '部分规格待确认'
    const labels = { manual: '人工采购基准', official: '维护的参考价表', official_reference: '官方参考价', reference: '账号参考价' }
    return [...new Set(entries.map(entry => labels[entry.source] || '参考价'))].join(' · ')
  }
  if (config.value?.simple_billing_enabled !== false) return '价格待核对 · 选择上游后试算'
  const profiles = matchingProfiles(row)
  if (profiles.length > 1) return '多个采购配置 · 预览核对'
  if (profiles.length) return profiles[0]?.name || '人工基准价'
  return row.accounts.some(account => account.type !== 'apikey') ? '系统计费参考价' : '自动匹配价表 · 预览核对'
}
function rateLabel(row: SaleRow) {
  const entries = catalogEntries(row).filter(entry => entry.status === 'ready')
  if (entries.length) return [...new Set(entries.map(entry => `${entry.route === 'api' ? 'API' : '分组'} ${entry.multiplier}×`))].join(' · ')
  if (config.value?.simple_billing_enabled !== false) return '按实际选定上游确认'
  const api = row.accounts.some(account => account.type === 'apikey')
  const authorized = row.accounts.some(account => account.type !== 'apikey')
  return api && authorized ? '混合来源 · 选择上游后试算' : api ? `按量 API 售价倍率 ${config.value?.api_sale_multiplier ?? 1.4}×` : `账号参考价 × 分组倍率 ${selectedGroup.value?.rate_multiplier ?? 1}×`
}
function priceLabel(row: SaleRow, sale = false): string {
  const entries = catalogEntries(row)
  if (entries.length) {
    const ready = entries.filter(entry => entry.status === 'ready')
    if (!ready.length) return '待确认'
    const values = new Set<string>()
    for (const entry of ready) for (const price of entry.base_prices) {
      if (price.service_tier || price.min_tokens || ['缓存读取', '缓存写入', '图像输入token', '图像输出token', '首张之后的参考输入图'].includes(price.label)) continue
      values.add(`${price.resolution || entry.resolution || ''} ${price.label === '基准单价' ? '' : price.label + ' '}${money(price.amount_cny * (sale ? entry.multiplier : 1))} / ${unitLabels[price.unit] || price.unit}`.trim())
    }
    const lines = [...values]
    return lines.slice(0, 2).join(' · ') + (lines.length > 2 ? ` · 共 ${lines.length} 项，试算查看` : '')
  }
  if (config.value?.simple_billing_enabled !== false) return '待核对 · 选择上游后试算'
  if (sale) return '选择上游后试算'
  const profiles = matchingProfiles(row)
  if (profiles.length > 1) return '多个来源 · 选择上游后查看'
  const profile = profiles[0]
  if (profile) {
    const prices = profile.prices.filter(price => price.models.some(pattern => matches(pattern, row.model)))
    if (prices.length > 1) return `${prices.length} 个规格 / 档位 · 试算查看`
    const price = prices[0]
    if (price?.billing_unit === 'per_million_tokens') return `输入 ${money(price.input_price == null ? null : price.input_price * profile.discount_multiplier, profile.currency)} · 输出 ${money(price.output_price == null ? null : price.output_price * profile.discount_multiplier, profile.currency)} / 百万 tokens`
    if (price) return `${money(price.unit_price == null ? null : price.unit_price * profile.discount_multiplier, profile.currency)} / ${unitLabels[price.billing_unit]}`
  }
  if (row.accounts.some(account => account.type === 'apikey')) return '按供应商价表匹配 · 试算查看'
  const references = channels.value.flatMap(channel => {
    const policy = modelPolicy(channel, row.model, row.platform)
    if (row.kind === 'Videos' && policy?.prices) return [Object.entries(policy.prices).map(([resolution, value]) => `${resolution} ${money(value, 'USD')} / 秒`).join(' · ')]
    const price = modelPricing(channel, row.model, row.platform) || wildcardPricing(channel, row.model, row.platform)
    if (!price) return []
    if (price.intervals.length) return [`${price.intervals.length} 个规格 / 用量档位`]
    if (price.billing_mode === 'token' || !price.billing_mode) return [`输入 ${money(price.input_price == null ? null : price.input_price * 1e6, 'USD')} · 输出 ${money(price.output_price == null ? null : price.output_price * 1e6, 'USD')} / 百万 tokens`]
    return [`${money(price.per_request_price, 'USD')} / ${price.billing_mode === 'image' ? '张' : '次'}`]
  })
  const unique = [...new Set(references)]
  return unique.length > 1 ? '多个参考价范围 · 试算查看' : unique[0] || '系统参考价 · 试算查看'
}
function openPreview(model?: string, profile?: string) { preview.value = { model, profile } }
function openCost(existing?: ProcurementProfile) { editorError.value = ''; costEditor.value = { existing } }
async function load() {
  loading.value = true; error.value = ''; notice.value = ''
  try {
    const [cost, inventory, scopes, groupList, prices] = await Promise.all([billingManagementAPI.getConfig(), loadAccounts(), loadChannels(), getAllIncludingInactive(), billingManagementAPI.catalog()])
    config.value = cost; accounts.value = inventory; channels.value = scopes; groups.value = groupList; catalog.value = prices
    apiMultiplier.value = cost.api_sale_multiplier ?? 1.4
    if (!groupList.some(group => group.id === selectedGroupID.value)) selectedGroupID.value = groupList[0]?.id
  } catch (cause) { error.value = extractApiErrorMessage(cause, '无法加载计费配置，请重试。') }
  finally { loading.value = false }
}
async function loadAccounts(): Promise<Account[]> { const items: Account[] = []; let page = 1; while (true) { const response = await accountsAPI.list(page++, 100); items.push(...response.items); if (items.length >= response.total || !response.items.length) return items } }
async function loadChannels(): Promise<Channel[]> { const items: Channel[] = []; let page = 1; while (true) { const response = await channelsAPI.list(page++, 100); items.push(...response.items); if (items.length >= response.total || !response.items.length) return items } }
async function refreshSavedCatalog() {
  try { catalog.value = await billingManagementAPI.catalog() }
  catch { catalog.value = []; error.value = '配置已保存，价格目录刷新失败，请点击刷新重试。' }
}
async function saveApiRate() {
  if (!config.value || saving.value) return
  if (!Number.isFinite(apiMultiplier.value) || apiMultiplier.value <= 0) { error.value = 'API 售价倍率须大于 0。'; return }
  saving.value = true; error.value = ''; notice.value = ''
  try { config.value = await billingManagementAPI.updateConfig({ ...config.value, simple_billing_enabled: true, api_sale_multiplier: apiMultiplier.value, usd_to_cny_rate: 6.2 }); await refreshSavedCatalog(); notice.value = 'API 统一倍率已保存，后续请求使用新规则。' }
  catch (cause) { error.value = extractApiErrorMessage(cause, '保存失败，请刷新配置后重试。') }
  finally { saving.value = false }
}
async function saveGroupRate() {
  const group = selectedGroup.value
  if (!group || saving.value) return
  if (!Number.isFinite(groupMultiplier.value) || groupMultiplier.value < 0) { error.value = '分组倍率须为非负数。'; return }
  saving.value = true; error.value = ''; notice.value = ''
  try { const updated = await updateGroup(group.id, { rate_multiplier: groupMultiplier.value }); groups.value = groups.value.map(item => item.id === updated.id ? updated : item); await refreshSavedCatalog(); notice.value = '账号分组倍率已保存。' }
  catch (cause) { error.value = extractApiErrorMessage(cause, '保存分组倍率失败，请重试。') }
  finally { saving.value = false }
}
async function saveCost(profile: ProcurementProfile) {
  if (!config.value || saving.value) return
  saving.value = true; editorError.value = ''; notice.value = ''
  try {
    const exists = config.value.cost_profiles.some(item => item.id === profile.id)
    const profiles = exists ? config.value.cost_profiles.map(item => item.id === profile.id ? profile : item) : [...config.value.cost_profiles, profile]
    config.value = await billingManagementAPI.updateConfig({ ...config.value, cost_profiles: profiles }); costEditor.value = null; await refreshSavedCatalog(); notice.value = '基准价配置已保存。'
  } catch (cause) { editorError.value = extractApiErrorMessage(cause, '保存失败，请刷新后重试。') }
  finally { saving.value = false }
}
async function deleteCost() {
  if (!config.value || !deleting.value || saving.value) return
  saving.value = true; error.value = ''
  try { config.value = await billingManagementAPI.updateConfig({ ...config.value, cost_profiles: config.value.cost_profiles.filter(item => item.id !== deleting.value?.id) }); deleting.value = null; await refreshSavedCatalog(); notice.value = '基准价配置已删除。' }
  catch (cause) { error.value = extractApiErrorMessage(cause, '删除失败，请刷新后重试。'); deleting.value = null }
  finally { saving.value = false }
}
onMounted(() => { void load() })
</script>
