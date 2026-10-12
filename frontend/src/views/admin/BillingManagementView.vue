<template>
  <AppLayout>
    <div class="mx-auto max-w-7xl space-y-5">
      <header class="flex flex-wrap items-start justify-between gap-4">
        <div><h2 class="text-xl font-semibold text-gray-900 dark:text-white">计费管理</h2><p class="mt-1 text-sm text-gray-500">统一维护售价、采购成本和换算规则。</p></div>
        <div class="flex gap-2"><button type="button" class="btn btn-secondary" :disabled="loading || saving" aria-label="刷新计费配置" @click="load"><Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" /></button><button type="button" class="btn btn-secondary" :disabled="loading || !config" @click="openPreview()">算费预览</button></div>
      </header>
      <p v-if="error" role="alert" class="rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-700 dark:bg-red-950/20">{{ error }}</p>
      <p v-if="notice" role="status" class="text-sm text-emerald-600">{{ notice }}</p>
      <div class="grid gap-3 sm:grid-cols-3">
        <div class="rounded-xl border border-gray-200 bg-white px-5 py-4 dark:border-dark-700 dark:bg-dark-800"><p class="text-xs text-gray-500">已登记模型</p><p class="mt-2 text-2xl font-semibold">{{ models.length }}</p></div>
        <div class="rounded-xl border border-gray-200 bg-white px-5 py-4 dark:border-dark-700 dark:bg-dark-800"><p class="text-xs text-gray-500">已配置成本的上游</p><p class="mt-2 text-2xl font-semibold">{{ coveredAccounts }}<span class="ml-1 text-sm font-normal text-gray-400">/ {{ accounts.length }}</span></p></div>
        <div class="rounded-xl border border-gray-200 bg-white px-5 py-4 dark:border-dark-700 dark:bg-dark-800"><p class="text-xs text-gray-500">成本换算</p><p class="mt-2 text-lg font-semibold">{{ config?.usd_to_cny_rate ? `1 USD = ${config.usd_to_cny_rate} CNY` : '待配置' }}</p></div>
      </div>
      <nav class="flex gap-1 overflow-x-auto rounded-xl bg-gray-100 p-1 dark:bg-dark-900" aria-label="计费管理分类">
        <button v-for="item in tabs" :key="item.value" type="button" class="flex-1 whitespace-nowrap rounded-lg px-4 py-2.5 text-sm font-medium transition-colors" :class="tab === item.value ? 'bg-white text-primary-700 shadow-sm dark:bg-dark-700 dark:text-primary-300' : 'text-gray-500 hover:text-gray-900 dark:hover:text-gray-200'" :aria-current="tab === item.value ? 'page' : undefined" @click="tab = item.value">{{ item.label }}</button>
      </nav>
      <p v-if="loading" class="py-16 text-center text-sm text-gray-500">正在加载计费配置…</p>
      <template v-else-if="config">
        <section v-if="tab === 'sales'" class="space-y-4">
          <div class="flex flex-wrap items-center justify-between gap-3"><div class="flex flex-1 flex-wrap gap-3"><input v-model="query" class="input w-full sm:max-w-sm" placeholder="搜索模型或供应商" aria-label="搜索费率模型" /><select v-model="modality" class="input w-36" aria-label="费率模型类型"><option value="">全部类型</option><option value="Chat">文本</option><option value="Images">图像</option><option value="Videos">视频</option></select></div><router-link to="/admin/groups" class="text-sm text-primary-600">管理分组倍率 →</router-link></div>
          <div class="overflow-x-auto rounded-xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
            <table class="w-full text-left text-sm"><thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-900"><tr><th class="px-5 py-3">模型</th><th class="px-5 py-3">基础售价 · USD</th><th class="px-5 py-3">采购配置</th><th class="px-5 py-3 text-right">操作</th></tr></thead><tbody>
              <tr v-for="row in filteredModels" :key="row.key" class="border-t border-gray-100 dark:border-dark-700"><td class="px-5 py-4"><p class="font-medium text-gray-900 dark:text-white">{{ row.model }}</p><p class="mt-1 text-xs text-gray-500">{{ typeLabel(row.kind) }} · {{ row.provider }}</p></td><td class="px-5 py-4"><p class="text-xs">{{ priceLabel(row) }}</p><p class="mt-1 text-xs text-gray-400">{{ row.scopes ? `${row.scopes} 个售价范围` : '沿用系统价表' }}</p></td><td class="px-5 py-4"><p class="text-xs" :class="row.costSources ? 'text-emerald-600' : 'text-amber-600'">{{ row.costSources ? `${row.costSources} 个上游已关联成本方案` : '成本待登记' }}</p><p class="mt-1 text-xs text-gray-400">{{ row.accounts.length }} 个上游来源</p></td><td class="px-5 py-4"><div class="flex justify-end gap-2"><button type="button" class="btn btn-secondary text-xs" :aria-label="`预览 ${row.model} 费用`" @click="openPreview(row.model)">预览</button><button type="button" class="btn btn-secondary text-xs" :aria-label="`设置 ${row.model} 售价`" @click="editingModel = row">设置售价</button></div></td></tr>
              <tr v-if="!filteredModels.length"><td colspan="4" class="px-5 py-12 text-center text-sm text-gray-500">{{ models.length ? '没有符合条件的模型。' : '先在上游管理登记模型，再设置费率。' }}</td></tr>
            </tbody></table>
          </div>
          <p class="text-xs text-gray-500">售价与模型服务共用配置；同一模型在不同分组范围可以分别定价。预览查看实际档位与最终倍率。</p>
          <details class="text-sm text-gray-500"><summary class="cursor-pointer">复杂路由与批量价表</summary><p class="mt-2 text-xs">同价模型可在渠道价表中共用一条价格规则；特殊模型再单独设置。</p><router-link to="/admin/channels/pricing" class="mt-2 inline-block text-primary-600">打开渠道价表 →</router-link></details>
        </section>
        <section v-else-if="tab === 'costs'" class="space-y-4">
          <div class="flex flex-wrap items-center justify-between gap-3"><p class="text-sm text-gray-500">按采购方式记录费用，支持账号授权和 API 密钥来源。</p><button type="button" class="btn btn-primary" :disabled="saving" @click="openCost()">添加成本方案</button></div>
          <div v-if="!config.cost_profiles.length" class="rounded-xl border border-dashed border-gray-300 bg-white p-12 text-center dark:border-dark-600 dark:bg-dark-800"><h3 class="font-semibold">先登记一份上游采购方案</h3><p class="mx-auto mt-2 max-w-md text-sm text-gray-500">填写原币费用、模型单价，再关联上游账号；包月和按量费用分别管理。</p><button type="button" class="btn btn-primary mt-5" @click="openCost()">添加成本方案</button></div>
          <div class="grid gap-4 lg:grid-cols-2">
            <article v-for="profile in config.cost_profiles" :key="profile.id" class="rounded-xl border border-gray-200 bg-white p-5 dark:border-dark-700 dark:bg-dark-800">
              <div class="flex items-start justify-between gap-3"><div><h3 class="font-semibold">{{ profile.name }}</h3><p class="mt-1 text-xs text-gray-500">{{ profile.provider }} · {{ purchaseLabels[profile.purchase_mode] }} · {{ profile.currency }}</p></div><span class="whitespace-nowrap rounded-full bg-gray-100 px-2.5 py-1 text-xs text-gray-600 dark:bg-dark-700 dark:text-gray-300">{{ profileState(profile) }}</span></div>
              <dl class="mt-4 space-y-2 text-sm"><div class="flex justify-between gap-3"><dt class="text-gray-500">模型规则</dt><dd>{{ profile.prices.length }} 条</dd></div><div v-if="profile.period_cost != null" class="flex justify-between gap-3"><dt class="text-gray-500">{{ profile.purchase_mode === 'credit_pack' ? '套餐实付' : '周期实付' }}</dt><dd>{{ money(profile.period_cost, profile.currency) }}</dd></div><div class="flex justify-between gap-3"><dt class="text-gray-500">关联上游</dt><dd class="max-w-[70%] text-right">{{ profileAccounts(profile, accounts) || '未关联 · 成本模板' }}</dd></div></dl>
              <div class="mt-4 flex flex-wrap items-center justify-between gap-2 border-t border-gray-100 pt-4 dark:border-dark-700"><button type="button" class="text-xs text-gray-400 hover:text-red-600" :disabled="saving" @click="deleting = profile">删除方案</button><div class="flex gap-2"><button type="button" class="btn btn-secondary text-xs" @click="openPreview(undefined, profile.id)">预览</button><button type="button" class="btn btn-secondary text-xs" @click="openCost(profile)">编辑</button></div></div>
            </article>
          </div>
          <p class="text-xs text-gray-500">单次费用是按价表估算，供应商账单用于最终对账。包月方案显示周期费用，未登记成本不会按 0 元处理。</p>
        </section>
        <section v-else class="space-y-4">
          <div class="grid items-start gap-5 lg:grid-cols-2">
            <form class="space-y-4 rounded-xl border border-gray-200 bg-white p-5 dark:border-dark-700 dark:bg-dark-800" @submit.prevent="saveExchange">
              <div><h3 class="font-semibold">采购成本汇率</h3><p class="mt-1 text-xs text-gray-500">用于美元采购成本折算成人民币。</p></div>
              <label class="block text-sm">1 美元对应人民币<input v-model.number="exchangeRate" type="number" min="0" step="any" class="input mt-1" placeholder="未配置时填 0" /><span class="mt-1 block text-xs text-gray-500">0 表示尚未配置；美元成本预览不进行换算。</span></label>
              <label class="block text-sm">汇率来源<input v-model="exchangeSource" maxlength="500" class="input mt-1" placeholder="实际采购结算汇率、银行或其他来源" /></label>
              <p v-if="config.exchange_rate_effective_at" class="text-xs text-gray-400">最近生效 {{ new Date(config.exchange_rate_effective_at).toLocaleString('zh-CN') }} · 配置版本 {{ config.version }}</p>
              <div class="flex justify-end"><button type="submit" class="btn btn-primary" :disabled="saving || !exchangeChanged">保存成本汇率</button></div>
            </form>
            <form v-if="paymentConfig" class="space-y-4 rounded-xl border border-gray-200 bg-white p-5 dark:border-dark-700 dark:bg-dark-800" @submit.prevent="reviewRecharge">
              <div><h3 class="font-semibold">在线充值入账</h3><p class="mt-1 text-xs text-gray-500">充值商业规则与采购汇率分别设置。</p></div>
              <label class="block text-sm">入账倍率<input v-model.number="rechargeMultiplier" type="number" min="0.000001" step="any" class="input mt-1" /><span class="mt-1 block text-xs text-gray-500">每 1 收款币种到账多少美元额度，应用于已启用的在线支付通道。</span></label>
              <div class="rounded-lg bg-gray-50 p-4 dark:bg-dark-900"><p class="text-xs text-gray-500">人民币通道示例</p><p class="mt-2 text-sm">支付 <strong>¥100</strong> → 到账 <strong>{{ money(100 * rechargeMultiplier, 'USD') }}</strong></p><p v-if="rechargeMultiplier > 0" class="mt-1 text-xs text-gray-500">每 $1 额度对应充值款约 {{ money(1 / rechargeMultiplier) }}，未计手续费或额外赠送。</p></div>
              <details><summary class="cursor-pointer text-sm text-gray-500">订阅收款换算</summary><label class="mt-3 block text-sm">订阅价格 1 USD 对应 CNY<input v-model.number="subscriptionRate" type="number" min="0" step="any" class="input mt-1" /><span class="mt-1 block text-xs text-gray-500">只作用于人民币通道订阅订单；0 保留原数值直付规则。</span></label></details>
              <p class="text-xs text-gray-500">外部店铺卡密按照卡密面额入账，需在店铺与兑换码中单独配置。</p>
              <div class="flex justify-end"><button type="submit" class="btn btn-primary" :disabled="saving || !rechargeChanged">预览充值规则</button></div>
            </form>
            <div v-else class="rounded-xl border border-gray-200 bg-white p-5 text-sm text-gray-500 dark:border-dark-700 dark:bg-dark-800">{{ paymentError || '在线支付配置未加载。' }}<button type="button" class="mt-3 block text-primary-600" @click="loadPayment">重试加载支付配置</button></div>
          </div>
          <details class="rounded-xl border border-gray-200 bg-white p-5 text-sm dark:border-dark-700 dark:bg-dark-800"><summary class="cursor-pointer font-medium">人民币钱包迁移</summary><p class="mt-3 text-sm text-gray-500">当前余额、扣费和限额以美元额度保存。迁移到人民币需先根据充值记录和原售卖承诺，制定旧余额兑换与补偿规则，再统一迁移余额、售价、限额和展示。成本汇率与充值规则不执行历史余额迁移。</p><router-link to="/admin/redeem" class="mt-3 inline-block text-primary-600">管理外部店铺兑换码 →</router-link></details>
        </section>
      </template>
    </div>
    <CostProfileEditor v-if="costEditor" :existing="costEditor.existing" :accounts="accounts" :saving="saving" :error="editorError" @close="closeCost" @save="saveCost" />
    <BillingPreviewDialog v-if="preview" :model="preview.model" :profile="preview.profile" :accounts="accounts" :groups="groups" :profiles="config?.cost_profiles || []" :models="models.map(row => row.model)" @close="preview = null" />
    <ModelServiceEditor v-if="editingModel" :model="editingModel.model" :platform="editingModel.platform" :kind="editingModel.kind === 'Videos' ? 'video' : editingModel.kind === 'Images' ? 'image' : 'chat'" :channels="channels" :accounts="editingModel.accounts" @close="editingModel = null" @saved="salesSaved" />
    <ConfirmDialog :show="!!deleting" title="删除成本方案" :message="`删除 ${deleting?.name || ''} 后，关联上游的成本预览可能变为待配置。`" danger :confirm-text="saving ? '删除中…' : '确认删除'" @cancel="!saving && (deleting = null)" @confirm="deleteCost" />
    <ConfirmDialog :show="rechargeReview" title="应用在线充值规则" :message="`人民币在线支付 ¥100 将到账 ${money(100 * rechargeMultiplier, 'USD')}。订阅人民币换算为 ${subscriptionRate > 0 ? `1 USD = ${subscriptionRate} CNY` : '未启用'}。规则用于后续在线订单，外部店铺卡密需独立配置。`" :confirm-text="saving ? '保存中…' : '确认应用'" @cancel="!saving && (rechargeReview = false)" @confirm="saveRecharge" />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import ModelServiceEditor from '@/components/admin/ModelServiceEditor.vue'
import CostProfileEditor from '@/components/admin/billing/CostProfileEditor.vue'
import BillingPreviewDialog from '@/components/admin/billing/BillingPreviewDialog.vue'
import { billingManagementAPI, type BillingManagementConfig, type ProcurementProfile } from '@/api/admin/billingManagement'
import channelsAPI, { type Channel } from '@/api/admin/channels'
import { accountsAPI } from '@/api/admin/accounts'
import { getAllIncludingInactive } from '@/api/admin/groups'
import { adminPaymentAPI, type AdminPaymentConfig } from '@/api/admin/payment'
import type { Account, AdminGroup } from '@/types'
import type { ModelServicePolicy } from '@/types/modelService'
import { catalogModelProvider, catalogModelType, isAudioModel, type CatalogModelType } from '@/utils/modelCatalog'
import { modelPolicy, modelPricing, wildcardPricing } from '@/components/admin/modelServiceAdmin'
import { costCoverage, money, profileAccounts, profileState, purchaseLabels } from '@/components/admin/billing/billingManagement'
import { extractApiErrorMessage } from '@/utils/apiError'
type SaleRow = { key: string; model: string; platform: string; kind: CatalogModelType; provider: string; accounts: Account[]; scopes: number; costSources: number }
const route = useRoute()
const tabs = [{ value: 'sales', label: '模型售价' }, { value: 'costs', label: '上游成本' }, { value: 'exchange', label: '充值与换算' }]
const tab = ref(tabs.some(item => item.value === route.query.tab) ? String(route.query.tab) : 'sales')
const query = ref(typeof route.query.model === 'string' ? route.query.model : '')
const modality = ref('')
const config = ref<BillingManagementConfig | null>(null)
const accounts = ref<Account[]>([])
const groups = ref<AdminGroup[]>([])
const channels = ref<Channel[]>([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const notice = ref('')
const editorError = ref('')
const paymentError = ref('')
const costEditor = ref<{ existing?: ProcurementProfile } | null>(null)
const deleting = ref<ProcurementProfile | null>(null)
const preview = ref<{ model?: string; profile?: string } | null>(null)
const editingModel = ref<SaleRow | null>(null)
const paymentConfig = ref<AdminPaymentConfig | null>(null)
const exchangeRate = ref(0)
const exchangeSource = ref('')
const rechargeMultiplier = ref(1)
const subscriptionRate = ref(0)
const rechargeReview = ref(false)
const exchangeChanged = computed(() => !!config.value && (exchangeRate.value !== config.value.usd_to_cny_rate || exchangeSource.value !== config.value.exchange_rate_source))
const rechargeChanged = computed(() => !!paymentConfig.value && (rechargeMultiplier.value !== paymentConfig.value.balance_recharge_multiplier || subscriptionRate.value !== paymentConfig.value.subscription_usd_to_cny_rate))
const coveredAccounts = computed(() => config.value ? costCoverage(config.value, accounts.value) : 0)
const activeCostAccounts = computed(() => new Set((config.value?.cost_profiles || []).filter(profile => profileState(profile) === '生效中').flatMap(profile => profile.account_ids)))
const models = computed<SaleRow[]>(() => {
  const inventory = new Map<string, SaleRow>()
  const add = (platform: string, model: string, account?: Account, kind?: string) => {
    if (!model || model.includes('*') || isAudioModel(model)) return
    const key = `${platform}:${model}`
    const entry = inventory.get(key) || { key, model, platform, kind: catalogModelType(model, kind), provider: catalogModelProvider(model, platform), accounts: [], scopes: 0, costSources: 0 }
    if (account && !entry.accounts.some(item => item.id === account.id)) entry.accounts.push(account)
    if (kind) entry.kind = catalogModelType(model, kind)
    inventory.set(key, entry)
  }
  for (const account of accounts.value) for (const model of Object.keys(account.configured_models || {})) add(account.platform, model, account)
  for (const channel of channels.value) {
    for (const entry of channel.model_pricing) for (const model of entry.models) add(entry.platform, model, undefined, entry.billing_mode)
    for (const policy of (channel.features_config?.model_services as ModelServicePolicy[] | undefined) || []) add(policy.platform, policy.model, undefined, policy.kind)
  }
  return [...inventory.values()].map(row => ({ ...row, scopes: channels.value.filter(channel => modelPolicy(channel, row.model, row.platform) || modelPricing(channel, row.model, row.platform) || wildcardPricing(channel, row.model, row.platform)).length, costSources: row.accounts.filter(account => activeCostAccounts.value.has(account.id)).length })).sort((a, b) => a.model.localeCompare(b.model))
})
const filteredModels = computed(() => models.value.filter(row => (!modality.value || row.kind === modality.value) && `${row.model} ${row.provider}`.toLowerCase().includes(query.value.toLowerCase())))
const typeLabel = (kind: CatalogModelType) => ({ Chat: '文本', Images: '图像', Videos: '视频' })[kind]
function priceLabel(row: SaleRow): string {
  const references = channels.value.flatMap(channel => {
    const policy = modelPolicy(channel, row.model, row.platform)
    if (row.kind === 'Videos' && policy?.prices) return [Object.entries(policy.prices).map(([resolution, price]) => `${resolution} ${money(price, 'USD')} / 秒`).join(' · ')]
    const price = modelPricing(channel, row.model, row.platform) || wildcardPricing(channel, row.model, row.platform)
    if (!price) return []
    if (price.intervals.length) return [`按规格 / 用量分 ${price.intervals.length} 档`]
    if (price.billing_mode === 'token' || !price.billing_mode) return [`输入 ${money(price.input_price == null ? null : price.input_price * 1e6, 'USD')} · 输出 ${money(price.output_price == null ? null : price.output_price * 1e6, 'USD')} / 百万 tokens`]
    return [`${money(price.per_request_price, 'USD')} / ${price.billing_mode === 'image' ? '张' : '次'}`]
  })
  const unique = [...new Set(references)]
  return unique.length > 1 ? '多个售价范围 · 预览查看实际价格' : unique[0] || '系统默认价格 · 预览核对'
}
function openPreview(model?: string, profile?: string) { preview.value = { model, profile } }
function openCost(existing?: ProcurementProfile) { editorError.value = ''; costEditor.value = { existing } }
function closeCost() { if (!saving.value) costEditor.value = null }
async function loadPayment() {
  paymentError.value = ''
  try { paymentConfig.value = (await adminPaymentAPI.getConfig()).data; rechargeMultiplier.value = paymentConfig.value.balance_recharge_multiplier; subscriptionRate.value = paymentConfig.value.subscription_usd_to_cny_rate }
  catch (cause) { paymentError.value = extractApiErrorMessage(cause, '无法加载在线支付配置。') }
}
async function load() {
  loading.value = true; error.value = ''; notice.value = ''
  try {
    const [cost, inventory, scopes, groupList] = await Promise.all([billingManagementAPI.getConfig(), loadAccounts(), loadChannels(), getAllIncludingInactive()])
    config.value = cost; accounts.value = inventory; channels.value = scopes; groups.value = groupList
    exchangeRate.value = cost.usd_to_cny_rate; exchangeSource.value = cost.exchange_rate_source
  } catch (cause) { error.value = extractApiErrorMessage(cause, '无法加载计费配置，请重试。') }
  finally { loading.value = false }
}
async function loadAccounts(): Promise<Account[]> {
  const result: Account[] = []; let page = 1
  while (true) { const response = await accountsAPI.list(page++, 100); result.push(...response.items); if (result.length >= response.total || !response.items.length) return result }
}
async function loadChannels(): Promise<Channel[]> {
  const result: Channel[] = []; let page = 1
  while (true) { const response = await channelsAPI.list(page++, 100); result.push(...response.items); if (result.length >= response.total || !response.items.length) return result }
}
async function saveCost(profile: ProcurementProfile) {
  if (!config.value || saving.value) return
  saving.value = true; editorError.value = ''; notice.value = ''
  try {
    const exists = config.value.cost_profiles.some(item => item.id === profile.id)
    const profiles = exists ? config.value.cost_profiles.map(item => item.id === profile.id ? profile : item) : [...config.value.cost_profiles, profile]
    config.value = await billingManagementAPI.updateConfig({ ...config.value, cost_profiles: profiles })
    costEditor.value = null; notice.value = '成本方案已保存。'
  } catch (cause) { editorError.value = extractApiErrorMessage(cause, '保存失败；如配置已被他人修改，请刷新后重试。') }
  finally { saving.value = false }
}
async function deleteCost() {
  if (!config.value || !deleting.value || saving.value) return
  saving.value = true; error.value = ''
  try { config.value = await billingManagementAPI.updateConfig({ ...config.value, cost_profiles: config.value.cost_profiles.filter(item => item.id !== deleting.value?.id) }); deleting.value = null; notice.value = '成本方案已删除。' }
  catch (cause) { error.value = extractApiErrorMessage(cause, '删除失败，请刷新后重试。'); deleting.value = null }
  finally { saving.value = false }
}
async function saveExchange() {
  if (!config.value || saving.value) return
  if (!Number.isFinite(exchangeRate.value) || exchangeRate.value < 0) { error.value = '汇率须为非负有效数值。'; return }
  saving.value = true; error.value = ''; notice.value = ''
  try { config.value = await billingManagementAPI.updateConfig({ ...config.value, usd_to_cny_rate: exchangeRate.value, exchange_rate_source: exchangeSource.value, exchange_rate_effective_at: exchangeRate.value > 0 ? new Date().toISOString() : null }); notice.value = '采购成本汇率已保存。' }
  catch (cause) { error.value = extractApiErrorMessage(cause, '保存失败，请刷新配置后重试。') }
  finally { saving.value = false }
}
function reviewRecharge() {
  error.value = ''
  if (!Number.isFinite(rechargeMultiplier.value) || rechargeMultiplier.value <= 0 || !Number.isFinite(subscriptionRate.value) || subscriptionRate.value < 0) { error.value = '请填写有效的充值倍率与订阅换算值。'; return }
  rechargeReview.value = true
}
async function saveRecharge() {
  if (!paymentConfig.value || saving.value) return
  saving.value = true; error.value = ''; notice.value = ''
  try {
    const latest = (await adminPaymentAPI.getConfig()).data
    if (latest.balance_recharge_multiplier !== paymentConfig.value.balance_recharge_multiplier || latest.subscription_usd_to_cny_rate !== paymentConfig.value.subscription_usd_to_cny_rate) throw new Error('充值规则已被其他操作修改，请重新加载后核对。')
    await adminPaymentAPI.updateConfig({ balance_recharge_multiplier: rechargeMultiplier.value, subscription_usd_to_cny_rate: subscriptionRate.value })
    paymentConfig.value = { ...latest, balance_recharge_multiplier: rechargeMultiplier.value, subscription_usd_to_cny_rate: subscriptionRate.value }; notice.value = '在线充值规则已保存。'
  } catch (cause) { error.value = extractApiErrorMessage(cause, cause instanceof Error ? cause.message : '充值规则保存失败。') }
  finally { saving.value = false; rechargeReview.value = false }
}
async function salesSaved() { editingModel.value = null; channels.value = await loadChannels(); notice.value = '模型售价已更新。' }
onMounted(() => { void load(); void loadPayment() })
</script>
