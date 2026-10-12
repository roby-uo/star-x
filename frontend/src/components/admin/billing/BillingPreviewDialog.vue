<template>
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4" @click.self="!loading && emit('close')">
    <section ref="dialog" role="dialog" aria-modal="true" aria-labelledby="billing-preview-title" class="flex max-h-[92vh] w-full max-w-3xl flex-col rounded-2xl bg-white shadow-xl dark:bg-dark-800">
      <header class="flex items-start justify-between gap-4 border-b border-gray-100 p-5 dark:border-dark-700"><div><h2 id="billing-preview-title" class="text-lg font-semibold">算费预览</h2><p class="mt-1 text-xs text-gray-500">使用已保存的配置计算，不产生模型调用或扣费。</p></div><button type="button" class="btn btn-secondary" :disabled="loading" aria-label="关闭算费预览" @click="emit('close')">关闭</button></header>
      <div class="min-h-0 overflow-y-auto p-5">
        <form class="space-y-4" @submit.prevent="calculate">
          <div class="grid gap-4 sm:grid-cols-2">
            <label class="text-sm">模型<input v-model="input.model" required class="input mt-1" list="billing-model-options" /><datalist id="billing-model-options"><option v-for="model in models" :key="model" :value="model" /></datalist></label>
            <label class="text-sm">类型<select v-model="input.modality" class="input mt-1"><option value="chat">文本</option><option value="image">图像</option><option value="video">视频</option></select></label>
            <label class="text-sm">调用分组<select v-model.number="input.group_id" class="input mt-1" aria-label="调用分组" required><option :value="undefined">请选择</option><option v-for="group in eligibleGroups" :key="group.id" :value="group.id">{{ group.name }}</option></select></label>
            <label class="text-sm">实际上游账号<select v-model.number="input.account_id" class="input mt-1"><option :value="undefined">按所选成本模板预览</option><option v-for="account in eligibleAccounts" :key="account.id" :value="account.id">{{ account.name }}</option></select></label>
            <label class="text-sm">成本方案<select v-model="input.profile_id" class="input mt-1"><option :value="undefined">按账号自动匹配</option><option v-for="profile in profiles" :key="profile.id" :value="profile.id">{{ profile.name }}</option></select></label>
            <label v-if="input.modality === 'chat'" class="text-sm">处理层级<select v-model="input.service_tier" class="input mt-1"><option value="">标准</option><option value="priority">优先处理</option><option value="flex">弹性处理</option></select></label>
            <label v-if="input.modality === 'chat'" class="text-sm">调用用途<select v-model="input.operation" class="input mt-1"><option :value="undefined">普通模型请求</option><option value="alpha_search">Codex 独立搜索</option></select></label>
          </div>
          <div v-if="input.modality === 'chat'" class="grid gap-3 sm:grid-cols-2"><label v-for="field in counters" :key="field.key" class="text-sm">{{ field.label }}<input v-model.number="input[field.key]" type="number" min="0" step="1" required class="input mt-1" /></label></div>
          <details v-if="input.modality === 'chat' && !input.operation"><summary class="cursor-pointer text-sm text-gray-500">图片理解用量（可选）</summary><label class="mt-3 block text-sm">其中图像输入 tokens<input v-model.number="input.image_input_tokens" type="number" min="0" step="1" class="input mt-1 max-w-40" /></label><p class="mt-2 text-xs text-gray-500">包含在上方未缓存输入总量中，不重复增加。填写后会检查图片输入基准价；文本价格已确认不代表视觉输入也已确认。</p></details>
          <div v-else class="grid gap-3 sm:grid-cols-3">
            <label class="text-sm">分辨率<input v-model="input.resolution" class="input mt-1" placeholder="例如 768P、2K" /></label>
            <label class="text-sm">质量档位<input v-model="input.quality" class="input mt-1" placeholder="可留空" /></label>
            <label v-if="input.modality === 'image'" class="text-sm">图片数量<input v-model.number="input.image_count" type="number" min="1" step="1" required class="input mt-1" /></label>
            <label v-else class="text-sm">输出时长（秒）<input v-model.number="input.duration_seconds" type="number" min="1" step="1" required class="input mt-1" /></label>
          </div>
          <label v-if="input.modality === 'image'" class="block text-sm">参考输入图片数量<input v-model.number="input.input_image_count" type="number" min="0" step="1" class="input mt-1 max-w-40" /><span class="mt-1 block text-xs text-gray-500">普通单图生成；多参考图的附加费用按供应商价表计算。</span></label>
          <label class="block text-sm">{{ input.modality === 'video' ? '视频数量' : '请求数量（仅按次规则使用）' }}<input v-model.number="input.requests" type="number" min="1" step="1" required class="input mt-1 max-w-40" /></label>
          <details v-if="input.modality === 'image'"><summary class="cursor-pointer text-sm text-gray-500">图像按 token 计费的用量</summary><div class="mt-3 grid gap-3 sm:grid-cols-2"><label v-for="field in counters" :key="field.key" class="text-sm">{{ field.label }}<input v-model.number="input[field.key]" type="number" min="0" step="1" class="input mt-1" /></label><label class="text-sm">其中图像输入 tokens<input v-model.number="input.image_input_tokens" type="number" min="0" step="1" class="input mt-1" /></label><label class="text-sm">其中图像输出 tokens<input v-model.number="input.image_output_tokens" type="number" min="0" step="1" class="input mt-1" /></label></div><p class="mt-2 text-xs text-gray-500">图像用量是输入 / 输出的子集，不能超过对应总量。按张计费可保持 0。</p></details>
          <p v-if="input.modality === 'chat'" class="text-xs text-gray-500">普通输入与缓存明细分别填写。总输入档位按三者合计匹配，避免重复计算。</p>
          <div class="flex justify-end"><button class="btn btn-primary" :disabled="loading">{{ loading ? '计算中…' : '计算费用' }}</button></div>
        </form>
        <p v-if="error" role="alert" class="mt-4 text-sm text-red-600">{{ error }}</p>
        <section v-if="result" aria-label="算费结果" class="mt-5 space-y-3 border-t border-gray-100 pt-5 dark:border-dark-700">
          <div class="grid gap-3 sm:grid-cols-2">
            <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-900"><p class="text-xs text-gray-500">{{ result.cost.status === 'period_only' ? '周期采购费用' : '上游基准费用' }}</p><p class="mt-2 text-xl font-semibold">{{ result.cost.status === 'period_only' ? money(result.cost.period_cost, result.cost.currency) : money(result.cost.amount, result.cost.currency) }}</p><p class="mt-2 text-xs text-gray-500">{{ result.cost.reason }}</p></div>
            <div class="rounded-xl bg-primary-50 p-4 dark:bg-primary-900/20"><p class="text-xs text-gray-500">最终扣费</p><p class="mt-2 text-xl font-semibold text-primary-700 dark:text-primary-300">{{ result.sale.amount_cny == null ? money(result.sale.amount, result.sale.currency) : money(result.sale.amount_cny) }}</p><p v-if="result.sale.multiplier != null" class="mt-1 text-xs text-gray-500">使用倍率 {{ result.sale.multiplier }}×</p><p class="mt-2 text-xs text-gray-500">{{ result.sale.reason }}</p></div>
          </div>
          <p v-if="result.cost.period_cost != null && result.cost.status !== 'period_only'" class="text-xs text-gray-500">另有周期 / 套餐实付 {{ money(result.cost.period_cost, result.cost.currency) }}，未重复计入单次增量费用。</p>
          <p class="rounded-lg border border-gray-200 px-3 py-2 text-xs text-gray-500 dark:border-dark-600">{{ result.comparison.reason }}</p>
          <ul v-if="result.warnings.length" class="list-disc space-y-1 pl-5 text-xs text-amber-700 dark:text-amber-400"><li v-for="warning in result.warnings" :key="warning">{{ warning }}</li></ul>
          <p class="text-xs text-gray-400">配置版本 {{ result.cost.config_version }} · 金额统一人民币 · 美元固定按 6.2 换算</p>
        </section>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { Account, AdminGroup } from '@/types'
import { billingManagementAPI, type BillingPreviewInput, type BillingPreview, type ProcurementProfile } from '@/api/admin/billingManagement'
import { extractApiErrorMessage } from '@/utils/apiError'
import { catalogModelType } from '@/utils/modelCatalog'
import { money } from './billingManagement'
import { useBillingDialog } from '@/composables/useBillingDialog'
const props = defineProps<{ model?: string; profile?: string; accounts: Account[]; groups: AdminGroup[]; profiles: ProcurementProfile[]; models: string[] }>()
const emit = defineEmits<{ close: [] }>()
const input = ref<BillingPreviewInput>({ model: props.model || '', profile_id: props.profile, modality: 'chat', input_tokens: 1000, output_tokens: 200, cache_read_tokens: 0, cache_write_tokens: 0, image_input_tokens: 0, image_output_tokens: 0, image_count: 1, input_image_count:0, image_scene:'single_image', duration_seconds: 5, requests: 1, resolution: '768P', quality: '', service_tier: '' })
const counters = [{ key: 'input_tokens', label: '未缓存输入 tokens' }, { key: 'output_tokens', label: '输出 tokens' }, { key: 'cache_read_tokens', label: '缓存读取 tokens' }, { key: 'cache_write_tokens', label: '缓存写入 tokens' }] as const
const eligibleAccounts = computed(() => props.accounts.filter(account => !input.value.group_id || (account.group_ids || account.groups?.map(group => group.id) || []).includes(input.value.group_id)))
const eligibleGroups = computed(() => props.groups.filter(group => !input.value.account_id || props.accounts.some(account => account.id === input.value.account_id && (account.group_ids || account.groups?.map(item => item.id) || []).includes(group.id))))
const loading = ref(false)
const dialog = ref<HTMLElement | null>(null)
useBillingDialog(dialog, () => emit('close'), () => loading.value)
const error = ref('')
const result = ref<BillingPreview | null>(null)
let revision = 0
watch(() => input.value.model, model => {
  const kind = catalogModelType(model)
  const next = kind === 'Videos' ? 'video' : kind === 'Images' ? 'image' : 'chat'
  input.value.modality = next
  if (next !== 'chat') input.value.resolution = next === 'image' ? '2K' : '768P'
  if (next !== 'chat') { input.value.input_tokens = 0; input.value.output_tokens = 0 }
}, { immediate: true })
watch(() => input.value.modality, kind => {
  input.value.input_tokens = kind === 'chat' ? 1000 : 0
  input.value.output_tokens = kind === 'chat' ? 200 : 0
  input.value.cache_read_tokens = 0; input.value.cache_write_tokens = 0
  input.value.image_input_tokens = 0; input.value.image_output_tokens = 0
  input.value.service_tier = ''
  input.value.operation = undefined
}, { flush: 'sync' })
watch(input, () => { revision++; result.value = null; error.value = '' }, { deep: true, flush: 'sync' })
async function calculate() {
  error.value = ''; result.value = null
  if (!input.value.group_id) { error.value = '请选择调用分组。'; return }
  const request = JSON.parse(JSON.stringify(input.value)) as BillingPreviewInput
  const requestRevision = revision
  loading.value = true
  try {
    const response = await billingManagementAPI.preview(request)
    if (revision === requestRevision) result.value = response
  } catch (cause) { if (revision === requestRevision) error.value = extractApiErrorMessage(cause, '计算失败，请检查配置后重试。') }
  finally { loading.value = false }
}
</script>
