<template>
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-3 sm:p-6" @click.self="!saving && emit('close')">
    <section role="dialog" aria-modal="true" aria-labelledby="service-title" class="flex max-h-[92vh] w-full max-w-5xl flex-col overflow-hidden rounded-2xl bg-white shadow-xl dark:bg-dark-800">
      <header class="flex items-start justify-between gap-4 border-b border-gray-100 px-5 py-5 dark:border-dark-700 sm:px-7">
        <div><p class="mb-1 text-xs text-gray-500">{{ kindLabel }}模型 · {{ catalogModelProvider(model, platform) }}</p><h2 id="service-title" class="break-all text-xl font-semibold text-gray-900 dark:text-white">{{ model }}</h2><p class="mt-1 text-xs text-gray-500">调用名称与当前模型一致。配置按选定分组范围生效。</p></div>
        <button type="button" class="btn btn-secondary shrink-0" :disabled="saving" @click="emit('close')">关闭</button>
      </header>
      <div class="overflow-y-auto px-5 py-5 sm:px-7">
        <section class="mb-5 rounded-xl border border-gray-200 bg-gray-50 p-4 dark:border-dark-600 dark:bg-dark-900/30">
          <label class="input-label" for="service-scope">本次修改的分组范围</label>
          <select v-if="candidateChannels.length > 1 || !selectedChannel" id="service-scope" v-model="selectedChannel" class="input" @change="selectChannel">
            <option value="">先选择范围，再编辑配置</option><option v-for="channel in candidateChannels" :key="channel.id" :value="channel.id">{{ scopeLabel(channel) }}</option>
          </select>
          <p v-else class="font-medium text-gray-900 dark:text-white">{{ selectedScopeLabel }}</p>
          <p v-if="selectedChannel" class="mt-2 text-xs text-gray-500">这些分组共用本页基础售价和服务规格；分组倍率保留各自设置。当前状态：{{ currentStateLabel }}。</p>
          <details v-if="unassignedGroups.length" class="mt-3 text-sm"><summary class="cursor-pointer text-primary-600">为其他分组创建开放范围</summary><div class="mt-3 space-y-3"><input v-model="newName" class="input" placeholder="范围名称，例如普通用户服务" aria-label="新开放范围名称" /><div class="flex flex-wrap gap-3"><label v-for="group in unassignedGroups" :key="group.id" class="inline-flex items-center gap-2"><input v-model="newGroups" type="checkbox" :value="group.id" />{{ group.name }}</label></div><button class="btn btn-secondary text-xs" type="button" :disabled="saving || !newName.trim() || !newGroups.length" @click="createChannel">创建并配置</button></div></details>
        </section>
        <template v-if="selectedChannel">
          <nav class="mb-5 flex flex-wrap gap-2 border-b border-gray-100 pb-3 dark:border-dark-700" aria-label="配置内容">
            <button v-for="tab in tabs" :key="tab.value" class="rounded-lg px-3 py-2 text-sm" :class="activeTab === tab.value ? 'bg-primary-50 font-medium text-primary-700 dark:bg-primary-900/30 dark:text-primary-300' : 'text-gray-500 hover:bg-gray-50 dark:hover:bg-dark-700'" type="button" @click="activeTab = tab.value">{{ tab.label }}</button>
          </nav>
          <form id="service-form" @submit.prevent="prepare('apply')">
            <section v-if="activeTab === 'specs'" class="space-y-5">
              <div><h3 class="font-semibold text-gray-900 dark:text-white">规格与基础售价</h3><p class="mt-1 text-xs text-gray-500">统一使用美元。这里只维护基础售价，倍率在“分组与倍率”中管理。</p></div>
              <template v-if="kind === 'video' && model === 'MiniMax-H3'">
                <div class="overflow-hidden rounded-xl border border-gray-200 dark:border-dark-600"><table class="w-full text-sm"><thead class="bg-gray-50 text-left text-gray-500 dark:bg-dark-900/30"><tr><th class="px-4 py-3">开放规格</th><th class="px-4 py-3">基础售价（USD / 秒）</th></tr></thead><tbody><tr v-for="resolution in ['768P', '2K']" :key="resolution" class="border-t border-gray-100 dark:border-dark-700"><td class="px-4 py-3"><label class="inline-flex items-center gap-2"><input v-model="policy.resolutions" type="checkbox" :value="resolution" />{{ resolution }}</label></td><td class="px-4 py-3"><input v-model.number="prices[resolution]" class="input max-w-48" type="number" min="0" step="any" :disabled="!policy.resolutions?.includes(resolution)" :aria-label="`${resolution} 每秒基础售价`" /></td></tr></tbody></table></div>
                <div class="grid grid-cols-2 gap-4"><label class="text-sm">最短时长（秒）<input v-model.number="policy.min_duration" type="number" min="4" max="15" class="input mt-1" /></label><label class="text-sm">最长时长（秒）<input v-model.number="policy.max_duration" type="number" min="4" max="15" class="input mt-1" /></label></div>
                <div><p class="input-label">生成方式</p><div class="flex flex-wrap gap-4"><label v-for="mode in videoModes" :key="mode.value" class="inline-flex items-center gap-2 text-sm"><input v-model="policy.modes" type="checkbox" :value="mode.value" />{{ mode.label }}</label></div></div>
                <p class="text-xs text-gray-500">以上为对外售价，不是采购成本。未配置服务规则时沿用系统默认规格与定价。</p>
              </template>
              <p v-else-if="kind === 'video'" class="rounded-lg bg-amber-50 p-4 text-sm text-amber-700 dark:bg-amber-950/30 dark:text-amber-300">此模型尚未适配规格管理，保留当前调用配置。请在高级渠道配置中管理已有路由。</p>
              <template v-else>
                <div v-if="kind === 'image'" class="grid gap-4 sm:grid-cols-2"><label class="text-sm">允许尺寸<input v-model="imageSizes" class="input mt-1" placeholder="2K,4K 或 1024x1024" /><span class="mt-1 block text-xs text-gray-400">逗号分隔，留空沿用上游支持范围。</span></label><label class="text-sm">单次图片数量上限<input v-model.number="policy.max_images" type="number" min="0" max="15" class="input mt-1" /><span class="mt-1 block text-xs text-gray-400">0 表示沿用上游限制。</span></label></div>
                <div v-if="hasWildcardPrice" class="rounded-lg bg-amber-50 p-4 text-sm text-amber-700 dark:bg-amber-950/30 dark:text-amber-300">当前价格由通配规则覆盖多个模型。为避免改变其他模型的费用，请在 <router-link to="/admin/channels/pricing" class="underline">高级渠道配置</router-link> 中调整该价格规则；本页仍可管理规格与开放。</div>
                <template v-else>
                  <label class="flex items-center gap-2 text-sm font-medium"><input v-model="customPricing" type="checkbox" />为这个模型设置基础售价</label>
                  <p v-if="!customPricing" class="text-sm text-gray-500">沿用现有系统与分组定价。</p>
                  <div v-if="customPricing" class="space-y-4 rounded-xl border border-gray-200 p-4 dark:border-dark-600">
                    <label class="block text-sm">计费方式<select v-model="pricing.billing_mode" class="input mt-1" @change="pricing.intervals = []"><option value="token">按用量（美元 / 百万词元）</option><option value="per_request">按次（美元 / 次）</option><option v-if="kind === 'image'" value="image">按图片（美元 / 张）</option></select></label>
                    <div v-if="pricing.billing_mode === 'token'" class="grid grid-cols-2 gap-4 sm:grid-cols-3"><label v-for="field in tokenPriceFields" :key="field.key" class="text-sm">{{ field.label }}<input v-model="pricing[field.key]" class="input mt-1" type="number" min="0" step="any" placeholder="沿用默认" /></label></div>
                    <label v-else class="block text-sm">默认基础售价<input v-model="pricing.per_request_price" class="input mt-1 max-w-56" type="number" min="0" step="any" placeholder="有分档时可留空" /></label>
                    <details :open="pricing.intervals.length > 0"><summary class="cursor-pointer text-sm text-primary-600">分档价格 {{ pricing.intervals.length ? `（${pricing.intervals.length} 档）` : '（可选）' }}</summary><div class="mt-3 space-y-2 overflow-x-auto"><IntervalRow v-for="(interval, index) in pricing.intervals" :key="index" class="min-w-[600px]" :interval="interval" :mode="pricing.billing_mode" @update="pricing.intervals[index] = $event" @remove="pricing.intervals.splice(index, 1)" /><button class="btn btn-secondary text-xs" type="button" @click="addInterval">添加价格档位</button></div></details>
                  </div>
                </template>
              </template>
            </section>
            <section v-if="activeTab === 'groups'" class="space-y-4">
              <div><h3 class="font-semibold text-gray-900 dark:text-white">开放分组与费用预览</h3><p class="mt-1 text-xs text-gray-500">勾选分组并限定其可用规格。预览采用当前分组倍率；用户单独倍率、峰时倍率等以实际结算为准。</p></div>
              <div v-for="group in eligibleGroups" :key="group.id" class="rounded-xl border border-gray-200 p-4 dark:border-dark-600">
                <label class="flex flex-wrap items-center gap-2 text-sm"><input v-model="rules[group.id]!.enabled" type="checkbox" :disabled="group.status !== 'active'" /><span class="font-medium">{{ group.name }}</span><span class="text-xs text-gray-500">分组倍率 {{ group.rate_multiplier }}×{{ group.status !== 'active' ? ' · 分组已停用' : '' }}</span></label>
                <div v-if="kind === 'video' && rules[group.id]!.enabled" class="mt-4 grid gap-4 sm:grid-cols-2">
                  <div><p class="mb-2 text-xs text-gray-500">允许分辨率</p><label v-for="resolution in policy.resolutions" :key="resolution" class="mr-4 inline-flex items-center gap-2 text-sm"><input v-model="rules[group.id]!.resolutions" type="checkbox" :value="resolution" />{{ resolution }}</label></div>
                  <label class="text-sm">最长时长（秒）<input v-model.number="rules[group.id]!.max_duration" class="input mt-1 max-w-32" type="number" :min="policy.min_duration" :max="policy.max_duration" /></label>
                  <div class="sm:col-span-2"><p class="mb-2 text-xs text-gray-500">允许生成方式</p><label v-for="mode in videoModes.filter(item => policy.modes?.includes(item.value))" :key="mode.value" class="mr-4 inline-flex items-center gap-2 text-sm"><input v-model="rules[group.id]!.modes" type="checkbox" :value="mode.value" />{{ mode.label }}</label></div>
                </div>
                <p v-if="rules[group.id]!.enabled" class="mt-3 text-xs text-primary-700 dark:text-primary-300">{{ groupPreview(group) }}</p>
                <p v-if="rules[group.id]!.enabled && !hasSourceForGroup(group.id)" class="mt-2 text-xs text-amber-600">没有已登记该模型的启用账号关联此分组，请先配置上游。</p>
              </div>
              <p v-if="!eligibleGroups.length" class="text-sm text-gray-500">该范围尚未关联匹配平台的分组，请在高级配置中关联。</p>
            </section>
            <section v-if="activeTab === 'sources'" class="space-y-4">
              <div><h3 class="font-semibold text-gray-900 dark:text-white">上游来源</h3><p class="mt-1 text-xs text-gray-500">模型登记表示已加入账号配置，不代表已验证真实生成权限。</p></div>
              <div v-for="account in accounts || []" :key="account.id" class="rounded-xl border border-gray-200 p-4 text-sm dark:border-dark-600"><div class="flex flex-wrap justify-between gap-2"><strong>{{ account.name }}</strong><span class="text-xs text-gray-500">{{ account.status === 'active' ? '账号启用' : '账号未启用' }} · 生成权限需实际验证</span></div><div class="mt-2 text-xs text-gray-500">上游调用名称：{{ account.configured_models?.[model] || model }}</div><div class="mt-1 text-xs text-gray-500">所属分组：{{ groups.filter(group => (account.group_ids || account.groups?.map(item => item.id) || []).includes(group.id)).map(group => group.name).join('、') || '尚未分配' }}</div></div>
              <p v-if="!accounts?.length" class="text-sm text-amber-600">尚无明确登记此模型的上游账号。</p>
              <details class="rounded-xl border border-gray-200 p-4 dark:border-dark-600"><summary class="cursor-pointer text-sm">高级配置</summary><dl class="mt-3 grid grid-cols-2 gap-2 text-xs"><dt class="text-gray-500">底层渠道</dt><dd>{{ currentChannel?.name }}</dd><dt class="text-gray-500">协议平台</dt><dd>{{ platform }}</dd><dt class="text-gray-500">渠道状态</dt><dd>{{ currentChannel?.status === 'active' ? '启用' : '停用' }}</dd></dl><router-link to="/admin/channels/pricing" class="mt-4 inline-block text-sm text-primary-600 underline">管理渠道、模型映射与路由</router-link></details>
            </section>
          </form>
        </template>
        <p v-if="error" role="alert" class="mt-4 text-sm text-red-600">{{ error }}</p>
      </div>
      <footer v-if="selectedChannel" class="flex flex-wrap items-center justify-between gap-3 border-t border-gray-100 bg-gray-50 px-5 py-4 dark:border-dark-700 dark:bg-dark-900/30 sm:px-7">
        <p class="text-xs text-gray-500">编辑尚未生效。应用前将确认本次影响的分组。</p>
        <div class="flex flex-wrap gap-2"><button v-if="canSaveDraft" type="button" class="btn btn-secondary text-sm" :disabled="saving || unsupportedVideo" @click="prepare('draft')">保存草稿</button><button v-if="initialPolicy.state === 'published'" type="button" class="btn btn-secondary text-sm text-red-600" :disabled="saving || unsupportedVideo" @click="prepare('pause')">暂停服务</button><button type="button" class="btn btn-primary text-sm" :disabled="saving || unsupportedVideo || currentChannel?.status !== 'active'" @click="prepare('apply')">{{ applyLabel }}</button></div>
      </footer>
    </section>
    <div v-if="confirmation" class="absolute inset-0 flex items-center justify-center bg-black/30 p-4">
      <section role="alertdialog" aria-modal="true" aria-labelledby="service-confirm-title" class="w-full max-w-lg rounded-2xl bg-white p-6 shadow-xl dark:bg-dark-800">
        <h3 id="service-confirm-title" class="text-lg font-semibold">{{ confirmation.action === 'pause' ? '确认暂停这个范围的服务' : confirmation.action === 'draft' ? '确认保存草稿' : '确认应用模型配置' }}</h3>
        <p class="mt-3 text-sm text-gray-600 dark:text-gray-300">模型：{{ model }}</p><p class="mt-2 text-sm text-gray-600 dark:text-gray-300">范围：{{ selectedScopeLabel }}</p>
        <p class="mt-2 text-sm text-gray-600 dark:text-gray-300">{{ confirmation.action === 'pause' ? '将停止此范围的新调用，已有视频任务仍可查询；未应用的价格和规格修改不会一起保存。' : confirmation.action === 'draft' ? '草稿不允许新调用，当前选择的配置将作为草稿保存。' : `应用后开放：${enabledGroupNames.join('、') || '无'}。未勾选分组将无法发起这个模型的新调用。` }}</p>
        <p v-if="confirmation.action === 'apply' && initialPolicy.state === 'published'" class="mt-2 text-sm text-gray-600 dark:text-gray-300">已发布状态保持；新规格与价格用于后续调用。</p>
        <p v-if="error" role="alert" class="mt-3 text-sm text-red-600">{{ error }}</p>
        <div class="mt-5 flex justify-end gap-3"><button class="btn btn-secondary" type="button" :disabled="saving" @click="confirmation = null">返回修改</button><button class="btn btn-primary" type="button" :disabled="saving" @click="save">{{ saving ? '应用中…' : '确认' }}</button></div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import channelsAPI, { type Channel, type ChannelModelPricing } from '@/api/admin/channels'
import { getAllIncludingInactive } from '@/api/admin/groups'
import type { Account, AdminGroup } from '@/types'
import { defaultModelPolicy, videoModes, type ModelServicePolicy, type ModelServiceRule } from '@/types/modelService'
import { extractApiErrorMessage } from '@/utils/apiError'
import { catalogModelProvider } from '@/utils/modelCatalog'
import { formatScaled } from '@/utils/pricing'
import IntervalRow from '@/components/admin/channel/IntervalRow.vue'
import { apiIntervalsToForm, formIntervalsToAPI, mTokToPerToken, perTokenToMTok, toNullableNumber, validateIntervals, type PricingFormEntry } from '@/components/admin/channel/types'
import { configuredModelOpen, mergeModelPolicy, mergeModelPricing, modelPolicy, modelPricing, modelScopeSnapshot, wildcardPricing } from './modelServiceAdmin'

const props = defineProps<{ model: string; platform: string; kind: ModelServicePolicy['kind']; channels: Channel[]; accounts?: Account[] }>()
const emit = defineEmits<{ close: []; saved: [] }>()
const { t } = useI18n()
const clone = <T,>(value: T): T => JSON.parse(JSON.stringify(value))
const localChannels = ref<Channel[]>([...props.channels])
const groups = ref<AdminGroup[]>([])
const selectedChannel = ref<number | ''>('')
const activeTab = ref('specs')
const tabs = [{ value: 'specs', label: '规格与价格' }, { value: 'groups', label: '开放分组' }, { value: 'sources', label: '上游来源' }]
const policy = ref(defaultModelPolicy(props.model, props.platform, props.kind))
const initialPolicy = ref(defaultModelPolicy(props.model, props.platform, props.kind))
const scopeSnapshot = ref('')
const rules = ref<Record<string, ModelServiceRule>>({})
const prices = ref<Record<string, number>>({ '768P': 0.08, '2K': 0.13 })
const imageSizes = ref('')
const customPricing = ref(false)
const newName = ref(`${props.model} 服务`)
const newGroups = ref<number[]>([])
const saving = ref(false)
const error = ref('')
const confirmation = ref<{ action: 'apply' | 'draft' | 'pause'; policy: ModelServicePolicy; pricing: ChannelModelPricing | null | undefined } | null>(null)
const currentChannel = computed(() => localChannels.value.find(channel => channel.id === selectedChannel.value))
const candidateChannels = computed(() => localChannels.value.filter(channel => groups.value.some(group => group.platform === props.platform && channel.group_ids.includes(group.id)) || modelPolicy(channel, props.model, props.platform) || modelPricing(channel, props.model, props.platform)))
const unassignedGroups = computed(() => groups.value.filter(group => group.platform === props.platform && group.status === 'active' && !localChannels.value.some(channel => channel.group_ids.includes(group.id))))
const eligibleGroups = computed(() => groups.value.filter(group => group.platform === props.platform && currentChannel.value?.group_ids.includes(group.id)))
const hasWildcardPrice = computed(() => !!currentChannel.value && !modelPricing(currentChannel.value, props.model, props.platform) && !!wildcardPricing(currentChannel.value, props.model, props.platform))
const kindLabel = computed(() => ({ chat: '文本', image: '图片', video: '视频' })[props.kind])
const unsupportedVideo = computed(() => props.kind === 'video' && (props.model !== 'MiniMax-H3' || props.platform !== 'openai'))
const currentStateLabel = computed(() => ({ draft: '草稿', published: '已开放', paused: '已暂停' })[initialPolicy.value.state])
const canSaveDraft = computed(() => initialPolicy.value.state === 'draft')
const applyLabel = computed(() => initialPolicy.value.state === 'published' ? '预览并应用' : initialPolicy.value.state === 'paused' ? '预览并恢复' : '预览并发布')
const selectedScopeLabel = computed(() => currentChannel.value ? scopeLabel(currentChannel.value) : '')
const enabledGroupNames = computed(() => eligibleGroups.value.filter(group => rules.value[group.id]?.enabled && group.status === 'active').map(group => group.name))
const tokenPriceFields = [ { key: 'input_price', label: '输入' }, { key: 'output_price', label: '输出' }, { key: 'cache_read_price', label: '缓存读取' }, { key: 'cache_write_price', label: '缓存写入' }, { key: 'image_input_price', label: '图像输入' }, { key: 'image_output_price', label: '图像输出' } ] as const
const pricing = ref<PricingFormEntry>(emptyPricing())

function emptyPricing(): PricingFormEntry { return { models: [props.model], billing_mode: props.kind === 'image' ? 'image' : 'token', input_price: null, output_price: null, cache_write_price: null, cache_read_price: null, image_input_price: null, image_output_price: null, per_request_price: null, intervals: [] } }
function scopeLabel(channel: Channel): string { return groups.value.filter(group => channel.group_ids.includes(group.id) && group.platform === props.platform).map(group => group.name).join('、') || `${channel.name}（未关联匹配分组）` }
function hasSourceForGroup(id: number): boolean { return (props.accounts || []).some(account => account.status === 'active' && account.schedulable !== false && (account.group_ids || account.groups?.map(group => group.id) || []).includes(id)) }
function legacyGroupOpen(group: AdminGroup): boolean { return group.status === 'active' && (props.kind === 'chat' || group.allow_image_generation) && (props.accounts || []).some(account => account.status === 'active' && account.schedulable !== false && (account.group_ids || account.groups?.map(item => item.id) || []).includes(group.id) && configuredModelOpen(currentChannel.value, props.model, props.platform, group.id, account.configured_models?.[props.model] || props.model)) }
function groupMultiplier(group: AdminGroup): number { return props.kind === 'video' && group.video_rate_independent ? group.video_rate_multiplier : props.kind === 'image' && group.image_rate_independent ? group.image_rate_multiplier : group.rate_multiplier }
function groupPreview(group: AdminGroup): string {
  const multiplier = groupMultiplier(group)
  if (props.kind === 'video') {
    const rule = rules.value[group.id]
    const resolution = rule?.resolutions.find(value => policy.value.resolutions?.includes(value))
    const duration = Math.max(policy.value.min_duration || 4, Math.min(5, rule?.max_duration || 5, policy.value.max_duration || 15))
    if (!resolution || duration < (policy.value.min_duration || 4) || prices.value[resolution] == null) return '选择有效规格后显示费用预览。'
    return `${resolution} · ${duration} 秒：$${Number((prices.value[resolution]! * duration * multiplier).toPrecision(10))}（基础价 × 时长 × ${multiplier}）`
  }
  if (hasWildcardPrice.value || !customPricing.value) return '沿用现有系统／分组定价；具体费用以实际调用结算为准。'
  if (pricing.value.intervals.length) return `采用已配置分档价格 × ${multiplier}；具体费用由请求档位与用量决定。`
  if (pricing.value.billing_mode === 'token') return `输入 ${formatScaled(mTokToPerToken(pricing.value.input_price), 1_000_000 * multiplier)} · 输出 ${formatScaled(mTokToPerToken(pricing.value.output_price), 1_000_000 * multiplier)} / 百万词元`
  return `${formatScaled(toNullableNumber(pricing.value.per_request_price), multiplier)} / ${pricing.value.billing_mode === 'image' ? '张' : '次'}`
}
function selectChannel() {
  confirmation.value = null; error.value = ''; activeTab.value = 'specs'
  const channel = currentChannel.value
  if (!channel) return
  scopeSnapshot.value = modelScopeSnapshot(channel, props.model, props.platform)
  const existing = modelPolicy(channel, props.model, props.platform)
  const base = existing ? clone(existing) : defaultModelPolicy(props.model, props.platform, props.kind)
  if (!existing) {
    const currentlyAvailable = eligibleGroups.value.some(legacyGroupOpen)
    base.state = currentlyAvailable ? 'published' : 'draft'
  }
  policy.value = base
  prices.value = { '768P': 0.08, '2K': 0.13, ...base.prices }
  rules.value = Object.fromEntries(eligibleGroups.value.map(group => [group.id, clone(base.groups?.[group.id] || {
    enabled: existing ? base.groups == null : legacyGroupOpen(group),
    resolutions: [...(base.resolutions || ['768P', '2K'])], max_duration: base.max_duration || 15, modes: [...(base.modes || ['text', 'first_frame', 'first_last_frame'])]
  })]))
  initialPolicy.value = clone({ ...base, groups: { ...base.groups, ...rules.value } })
  imageSizes.value = base.image_sizes?.join(',') || ''
  const existingPrice = modelPricing(channel, props.model, props.platform)
  customPricing.value = !!existingPrice
  pricing.value = existingPrice ? { ...clone(existingPrice), input_price: perTokenToMTok(existingPrice.input_price), output_price: perTokenToMTok(existingPrice.output_price), cache_write_price: perTokenToMTok(existingPrice.cache_write_price), cache_read_price: perTokenToMTok(existingPrice.cache_read_price), image_input_price: perTokenToMTok(existingPrice.image_input_price), image_output_price: perTokenToMTok(existingPrice.image_output_price), intervals: apiIntervalsToForm(existingPrice.intervals) } : emptyPricing()
}
function addInterval() { pricing.value.intervals.push({ min_tokens: 0, max_tokens: null, tier_label: '', input_price: null, output_price: null, cache_write_price: null, cache_read_price: null, per_request_price: null, sort_order: pricing.value.intervals.length }) }
watch(() => [policy.value.resolutions, policy.value.modes, policy.value.min_duration, policy.value.max_duration], () => {
  for (const rule of Object.values(rules.value)) if (rule.enabled && props.kind === 'video') { rule.resolutions = rule.resolutions.filter(value => policy.value.resolutions?.includes(value)); rule.modes = rule.modes.filter(value => policy.value.modes?.includes(value)); rule.max_duration = Math.max(policy.value.min_duration || 4, Math.min(rule.max_duration, policy.value.max_duration || 15)) }
}, { deep: true })
async function createChannel() {
  saving.value = true; error.value = ''
  try { const channel = await channelsAPI.create({ name: newName.value.trim(), group_ids: newGroups.value, model_pricing: [], model_mapping: {}, restrict_models: false }); localChannels.value.push(channel); selectedChannel.value = channel.id; newGroups.value = []; selectChannel() }
  catch (failure) { error.value = extractApiErrorMessage(failure, '创建开放范围失败') }
  finally { saving.value = false }
}
function prepare(action: 'apply' | 'draft' | 'pause') {
  error.value = ''
  const next: ModelServicePolicy = action === 'pause' ? { ...clone(initialPolicy.value), state: 'paused' } : {
    ...clone(policy.value), model: props.model, platform: props.platform, kind: props.kind,
    state: action === 'draft' ? 'draft' : 'published', image_sizes: imageSizes.value.split(/[,，]/).map(value => value.trim()).filter(Boolean),
    groups: { ...policy.value.groups, ...clone(rules.value) }, ...(props.kind === 'video' ? { prices: { ...prices.value } } : {})
  }
  if (action === 'apply') {
    const enabled = eligibleGroups.value.filter(group => group.status === 'active' && next.groups?.[group.id]?.enabled)
    if (!enabled.length) { error.value = '发布至少需要开放一个启用分组。'; activeTab.value = 'groups'; return }
    if (enabled.some(group => !hasSourceForGroup(group.id))) { error.value = '请先为开放分组关联已登记该模型的启用账号。'; activeTab.value = 'groups'; return }
    if (props.kind === 'image' && enabled.some(group => !group.allow_image_generation)) { error.value = '请先在分组配置中启用图片生成权限。'; activeTab.value = 'groups'; return }
  }
  if (action !== 'pause' && props.kind === 'image' && (!Number.isInteger(next.max_images || 0) || (next.max_images || 0) < 0 || (next.max_images || 0) > 15)) { error.value = '图片数量上限应为 0–15 的整数。'; activeTab.value = 'specs'; return }
  if (action !== 'pause' && props.kind === 'video') {
    if (!Number.isInteger(next.min_duration) || !Number.isInteger(next.max_duration) || next.min_duration! < 4 || next.max_duration! > 15 || next.min_duration! > next.max_duration! || !next.resolutions?.length || !next.modes?.length || next.resolutions.some(value => !Number.isFinite(next.prices?.[value]) || next.prices![value]! < 0)) { error.value = '请选择有效分辨率、每秒价格、生成方式和 4–15 秒内的时长。'; activeTab.value = 'specs'; return }
    if (Object.values(next.groups || {}).some(rule => rule.enabled && (!rule.resolutions.length || !rule.modes.length || !Number.isInteger(rule.max_duration) || rule.max_duration < next.min_duration! || rule.max_duration > next.max_duration! || rule.resolutions.some(value => !next.resolutions?.includes(value)) || rule.modes.some(value => !next.modes?.includes(value))))) { error.value = '开放分组的规格必须处于模型服务范围内。'; activeTab.value = 'groups'; return }
  }
  let nextPrice: ChannelModelPricing | null | undefined
  if (action !== 'pause' && props.kind !== 'video' && !hasWildcardPrice.value) {
    if (customPricing.value) {
      const values = [...tokenPriceFields.map(field => pricing.value[field.key]), pricing.value.per_request_price]
      if (values.some(value => value != null && value !== '' && (!Number.isFinite(Number(value)) || Number(value) < 0))) { error.value = '价格须为非负数字，留空表示沿用默认。'; activeTab.value = 'specs'; return }
      const intervalError = validateIntervals(pricing.value.intervals, pricing.value.billing_mode, t)
      if (intervalError) { error.value = intervalError; activeTab.value = 'specs'; return }
      if (pricing.value.intervals.some(interval => pricing.value.billing_mode === 'token'
        ? [interval.input_price, interval.output_price, interval.cache_write_price, interval.cache_read_price].every(value => toNullableNumber(value) == null)
        : toNullableNumber(interval.per_request_price) == null)) { error.value = '每个分档必须填写对应计费方式的价格。'; activeTab.value = 'specs'; return }
      if (pricing.value.billing_mode !== 'token' && toNullableNumber(pricing.value.per_request_price) == null && !pricing.value.intervals.length) { error.value = '按次或按图片计费需要默认价格或分档价格。'; activeTab.value = 'specs'; return }
      nextPrice = { platform: props.platform, models: [props.model], billing_mode: pricing.value.billing_mode, input_price: mTokToPerToken(pricing.value.input_price), output_price: mTokToPerToken(pricing.value.output_price), cache_write_price: mTokToPerToken(pricing.value.cache_write_price), cache_read_price: mTokToPerToken(pricing.value.cache_read_price), image_input_price: mTokToPerToken(pricing.value.image_input_price), image_output_price: mTokToPerToken(pricing.value.image_output_price), per_request_price: toNullableNumber(pricing.value.per_request_price), intervals: formIntervalsToAPI(pricing.value.intervals) }
    } else if (currentChannel.value && modelPricing(currentChannel.value, props.model, props.platform)) nextPrice = null
  }
  confirmation.value = { action, policy: next, pricing: nextPrice }
}
async function save() {
  if (!selectedChannel.value || !confirmation.value) return
  saving.value = true; error.value = ''
  try {
    const fresh = await channelsAPI.getById(selectedChannel.value)
    if (modelScopeSnapshot(fresh, props.model, props.platform) !== scopeSnapshot.value) { localChannels.value = localChannels.value.map(channel => channel.id === fresh.id ? fresh : channel); selectChannel(); error.value = '这个模型或分组范围已被修改，已载入最新配置，请重新确认。'; return }
    const request = { features_config: mergeModelPolicy(fresh, confirmation.value.policy), ...(confirmation.value.pricing !== undefined ? { model_pricing: mergeModelPricing(fresh.model_pricing, props.model, props.platform, confirmation.value.pricing) } : {}) }
    await channelsAPI.update(fresh.id, request)
    emit('saved')
  } catch (failure) { error.value = extractApiErrorMessage(failure, '应用模型配置失败') }
  finally { saving.value = false }
}
onMounted(async () => { try { groups.value = await getAllIncludingInactive(); if (candidateChannels.value.length === 1) { selectedChannel.value = candidateChannels.value[0]!.id; selectChannel() } } catch (failure) { error.value = extractApiErrorMessage(failure, '分组加载失败') } })
</script>
