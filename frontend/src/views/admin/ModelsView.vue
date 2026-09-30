<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="mb-5 flex flex-wrap items-start justify-between gap-4">
          <div>
            <h2 class="text-xl font-semibold text-gray-900 dark:text-white">管理对外提供的模型</h2>
            <p class="mt-1 text-sm text-gray-500">在一个地方设置规格、基础售价和开放分组，分组倍率继续沿用原有配置。</p>
          </div>
          <router-link to="/admin/accounts" class="btn btn-secondary">添加上游模型</router-link>
        </div>
        <div class="mb-4 flex flex-wrap gap-2" role="tablist" aria-label="模型类型">
          <button v-for="tab in typeTabs" :key="tab.value" type="button" role="tab" :aria-selected="type === tab.value"
            class="rounded-lg px-4 py-2 text-sm font-medium transition-colors" :class="type === tab.value ? 'bg-primary-600 text-white' : 'bg-white text-gray-600 hover:bg-gray-50 dark:bg-dark-800 dark:text-gray-300'" @click="type = tab.value">
            {{ tab.label }} <span class="ml-1 text-xs opacity-75">{{ tab.value ? rows.filter(row => row.type === tab.value).length : rows.length }}</span>
          </button>
        </div>
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div class="flex flex-1 flex-wrap items-center gap-3">
            <div class="relative w-full sm:w-80">
              <Icon name="search" size="md" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
              <input v-model="query" class="input pl-10" placeholder="搜索模型、供应商或上游账号" aria-label="搜索模型" />
            </div>
            <select v-model="statusFilter" class="input w-40" aria-label="开放状态">
              <option value="">全部状态</option><option value="open">已开放</option><option value="partial">部分开放</option>
              <option value="paused">已暂停</option><option value="draft">草稿</option><option value="unassigned">待配置</option>
            </select>
          </div>
          <button class="btn btn-secondary" :disabled="loading" aria-label="刷新模型" @click="load">
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
          </button>
        </div>
      </template>
      <template #table>
        <div class="table-wrapper">
          <table class="min-w-full">
            <thead><tr><th>模型</th><th>供应商</th><th>规格与基础售价</th><th>开放分组</th><th>状态</th><th class="text-right">管理</th></tr></thead>
            <tbody>
              <tr v-for="row in filteredRows" :key="row.key" class="hover:bg-gray-50 dark:hover:bg-dark-700/40">
                <td><div class="font-semibold text-gray-900 dark:text-white">{{ row.model }}</div><div class="mt-1 text-xs text-gray-500">{{ typeLabel(row.type) }} · {{ row.sources.length }} 个上游来源</div></td>
                <td><div>{{ catalogModelProvider(row.model, row.platform) }}</div><div class="mt-1 text-xs text-gray-400">{{ row.accounts || '尚未关联账号' }}</div></td>
                <td><div class="text-sm">{{ row.specs }}</div><div class="mt-1 text-xs text-gray-500">{{ row.price }}</div></td>
                <td><div v-if="row.groupNames.length" class="flex max-w-64 flex-wrap gap-1"><span v-for="name in row.groupNames" :key="name" class="rounded bg-gray-100 px-2 py-1 text-xs dark:bg-dark-700">{{ name }}</span></div><span v-else class="text-xs text-gray-400">尚未开放</span></td>
                <td><span class="inline-flex rounded-full px-2.5 py-1 text-xs font-medium" :class="statusClass(row.status)">{{ statusLabel(row.status) }}</span><div v-if="row.legacy" class="mt-1 text-xs text-gray-400">沿用现有配置</div></td>
                <td><button class="btn btn-secondary whitespace-nowrap text-xs" :aria-label="`配置 ${row.model}`" @click="editing = row">配置模型</button></td>
              </tr>
              <tr v-if="loading"><td colspan="6" class="py-16 text-center text-sm text-gray-500">正在加载模型…</td></tr>
              <tr v-else-if="!filteredRows.length"><td colspan="6" class="py-16 text-center text-sm text-gray-500"><span v-if="rows.length">没有符合条件的模型。</span><span v-else>先在 <router-link to="/admin/accounts" class="text-primary-600 underline">上游管理</router-link> 登记模型，随后在这里配置开放。</span></td></tr>
            </tbody>
          </table>
        </div>
      </template>
    </TablePageLayout>
    <ModelServiceEditor v-if="editing" :model="editing.model" :platform="editing.platform" :kind="editing.type === 'Videos' ? 'video' : editing.type === 'Images' ? 'image' : 'chat'" :channels="channels" :accounts="editing.sources" @close="editing = null" @saved="onSaved" />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import ModelServiceEditor from '@/components/admin/ModelServiceEditor.vue'
import Icon from '@/components/icons/Icon.vue'
import channelsAPI, { type Channel } from '@/api/admin/channels'
import { accountsAPI } from '@/api/admin/accounts'
import { getAllIncludingInactive } from '@/api/admin/groups'
import type { Account, AdminGroup } from '@/types'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { catalogModelProvider, catalogModelType, isAudioModel, type CatalogModelType } from '@/utils/modelCatalog'
import { configuredModelOpen, modelPolicy, modelPricing } from '@/components/admin/modelServiceAdmin'
import type { ModelServicePolicy } from '@/types/modelService'
import { formatScaled } from '@/utils/pricing'

type ModelStatus = 'open' | 'partial' | 'paused' | 'draft' | 'unassigned'
type ModelRow = { key: string; model: string; type: CatalogModelType; platform: string; accounts: string; sources: Account[]; groupNames: string[]; status: ModelStatus; legacy: boolean; specs: string; price: string }
const typeTabs = [{ value: '', label: '全部模型' }, { value: 'Chat', label: '文本' }, { value: 'Images', label: '图片' }, { value: 'Videos', label: '视频' }]
const appStore = useAppStore()
const channels = ref<Channel[]>([])
const accounts = ref<Account[]>([])
const groups = ref<AdminGroup[]>([])
const editing = ref<ModelRow | null>(null)
const loading = ref(false)
const query = ref('')
const type = ref('')
const statusFilter = ref('')
const typeLabel = (value: CatalogModelType) => ({ Chat: '文本', Images: '图片', Videos: '视频' })[value]
const statusLabel = (value: ModelStatus) => ({ open: '已开放', partial: '部分开放', paused: '已暂停', draft: '草稿', unassigned: '待配置' })[value]
const statusClass = (value: ModelStatus) => value === 'open' ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-300' : value === 'partial' ? 'bg-amber-50 text-amber-700 dark:bg-amber-950/40 dark:text-amber-300' : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'

const rows = computed<ModelRow[]>(() => {
  const inventory = new Map<string, { model: string; platform: string; kind?: string; sources: Account[] }>()
  const add = (platform: string, model: string, source?: Account, kind?: string) => {
    if (isAudioModel(model) || model.includes('*')) return
    const key = `${platform}:${model}`
    const entry = inventory.get(key) || { model, platform, sources: [] }
    if (source && !entry.sources.some(account => account.id === source.id)) entry.sources.push(source)
    if (kind) entry.kind = kind
    inventory.set(key, entry)
  }
  for (const account of accounts.value) for (const model of Object.keys(account.configured_models || {})) add(account.platform, model, account)
  for (const channel of channels.value) {
    for (const policy of (channel.features_config?.model_services as ModelServicePolicy[] | undefined) || []) add(policy.platform, policy.model, undefined, policy.kind)
    for (const pricing of channel.model_pricing) for (const model of pricing.models) add(pricing.platform, model, undefined, pricing.billing_mode)
  }
  return [...inventory.entries()].map(([key, entry]) => {
    const modelType = catalogModelType(entry.model, entry.kind)
    const activeSources = entry.sources.filter(account => account.status === 'active' && account.schedulable !== false)
    const sourceGroupIDs = new Set(activeSources.flatMap(account => account.group_ids || account.groups?.map(group => group.id) || []))
    const matchingGroups = groups.value.filter(group => group.platform === entry.platform && sourceGroupIDs.has(group.id) && group.status === 'active')
    const policies = channels.value.flatMap(channel => {
      const policy = modelPolicy(channel, entry.model, entry.platform)
      return policy ? [{ channel, policy }] : []
    })
    const openGroups = matchingGroups.filter(group => {
      const channel = channels.value.find(item => item.group_ids.includes(group.id))
      const policy = channel && modelPolicy(channel, entry.model, entry.platform)
      if (modelType === 'Images' && !group.allow_image_generation) return false
      if (modelType === 'Videos' && !policy && !group.allow_image_generation) return false
      return activeSources.some(account => (account.group_ids || account.groups?.map(item => item.id) || []).includes(group.id) && configuredModelOpen(channel, entry.model, entry.platform, group.id, account.configured_models?.[entry.model] || entry.model))
    })
    const status: ModelStatus = openGroups.length
      ? (openGroups.length < matchingGroups.length || policies.some(({ channel, policy }) => channel.status !== 'active' || policy.state !== 'published') ? 'partial' : 'open')
      : policies.some(({ channel, policy }) => channel.status !== 'active' || policy.state === 'paused') ? 'paused'
      : policies.some(({ policy }) => policy.state === 'draft') ? 'draft' : 'unassigned'
    const selectedPolicies = policies.map(item => item.policy)
    const specs = modelType === 'Videos'
      ? selectedPolicies.length ? [...new Set(selectedPolicies.flatMap(policy => policy.resolutions || []))].join(' / ') + ' · ' + `${Math.min(...selectedPolicies.map(policy => policy.min_duration || 4))}–${Math.max(...selectedPolicies.map(policy => policy.max_duration || 15))} 秒` : entry.model === 'MiniMax-H3' ? '768P / 2K · 4–15 秒' : '沿用上游规格'
      : modelType === 'Images' ? [...new Set(selectedPolicies.flatMap(policy => policy.image_sizes || []))].join(' / ') || '沿用上游图片规格' : '文本输入与输出'
    const pricingEntries = channels.value.flatMap(channel => { const pricing = modelPricing(channel, entry.model, entry.platform); return pricing ? [pricing] : [] })
    const summaries = modelType === 'Videos' ? [...new Set(selectedPolicies.map(policy => (policy.resolutions || []).map(resolution => `${resolution} ${formatScaled(policy.prices?.[resolution] ?? null, 1)}/秒`).join(' · ')).filter(Boolean))] : [...new Set(pricingEntries.map(pricing => pricing.intervals.length ? '已配置分档价格' : pricing.billing_mode === 'token' ? `输入 ${formatScaled(pricing.input_price, 1_000_000)} · 输出 ${formatScaled(pricing.output_price, 1_000_000)}/百万词元` : `${formatScaled(pricing.per_request_price, 1)}/${pricing.billing_mode === 'image' ? '张' : '次'}`))]
    return { key, model: entry.model, platform: entry.platform, type: modelType, sources: entry.sources, accounts: entry.sources.map(account => account.name).join('、'), groupNames: openGroups.map(group => group.name), status, legacy: !policies.length, specs, price: summaries.length > 1 ? '多个价格范围 · 在配置中查看' : summaries[0] || '沿用系统／分组定价' }
  }).sort((a, b) => a.type.localeCompare(b.type) || a.model.localeCompare(b.model))
})

const filteredRows = computed(() => {
  const text = query.value.trim().toLowerCase()
  return rows.value.filter(row => (!type.value || row.type === type.value) && (!statusFilter.value || row.status === statusFilter.value) && (!text || [row.model, row.platform, row.accounts, catalogModelProvider(row.model, row.platform)].some(value => value.toLowerCase().includes(text))))
})

async function load() {
  loading.value = true
  try {
    const fetchChannels = async () => {
      const all: Channel[] = []
      for (let page = 1; ; page++) { const result = await channelsAPI.list(page, 100); all.push(...result.items); if (!result.items.length || all.length >= result.total) return all }
    }
    const fetchAccounts = async () => {
      const all: Account[] = []
      for (let page = 1; ; page++) { const result = await accountsAPI.list(page, 100); all.push(...result.items); if (!result.items.length || all.length >= result.total) return all }
    }
    const [nextChannels, nextAccounts, nextGroups] = await Promise.all([fetchChannels(), fetchAccounts(), getAllIncludingInactive()])
    channels.value = nextChannels; accounts.value = nextAccounts; groups.value = nextGroups
  } catch (error) { appStore.showError(extractApiErrorMessage(error, '模型服务加载失败')) }
  finally { loading.value = false }
}
function onSaved() { editing.value = null; appStore.showSuccess('模型配置已应用'); void load() }
onMounted(load)
</script>
