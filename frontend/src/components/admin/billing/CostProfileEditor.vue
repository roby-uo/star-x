<template>
  <div class="fixed inset-0 z-50 flex items-center justify-end bg-black/40" @click.self="!saving && emit('close')">
    <section ref="dialog" role="dialog" aria-modal="true" aria-labelledby="procurement-title" class="flex h-full w-full max-w-3xl flex-col bg-white shadow-2xl dark:bg-dark-800">
      <header class="flex items-start justify-between gap-4 border-b border-gray-100 px-6 py-5 dark:border-dark-700">
        <div><h2 id="procurement-title" class="text-lg font-semibold">{{ existing ? '编辑上游基准价' : '添加上游基准价' }}</h2><p class="mt-1 text-sm text-gray-500">金额统一填写人民币，按量 API 售价使用统一倍率。</p></div>
        <button type="button" class="btn btn-secondary" :disabled="saving" aria-label="关闭成本配置" @click="emit('close')">关闭</button>
      </header>
      <div class="min-h-0 flex-1 overflow-y-auto px-6 py-5">
        <form id="cost-profile" class="space-y-6" @submit.prevent="submit">
          <fieldset :disabled="saving" class="space-y-6">
            <section class="space-y-4">
              <h3 class="text-sm font-semibold">采购方式</h3>
              <div class="grid gap-4 sm:grid-cols-2">
                <label class="text-sm">方案名称<input v-model="draft.name" required maxlength="100" class="input mt-1" placeholder="例如 MiniMax 官方 API" /></label>
                <label class="text-sm">供应商<input v-model="draft.provider" required maxlength="100" class="input mt-1" placeholder="供应商及站点" /></label>
                <label class="text-sm">采购模式<select v-model="draft.purchase_mode" class="input mt-1" aria-label="采购模式"><option value="payg">按量 API 基准价</option><option value="subscription">账号 / 包月参考价</option><option v-if="existing?.purchase_mode === 'credit_pack'" value="credit_pack">{{ purchaseLabels.credit_pack }} · 历史记录</option><option v-if="existing?.purchase_mode === 'hybrid'" value="hybrid">{{ purchaseLabels.hybrid }} · 历史记录</option></select></label>
                <label class="text-sm">采购原币<select v-model="draft.currency" class="input mt-1" aria-label="采购原币"><option value="CNY">人民币价表</option><option value="USD">美元价表 · 固定 6.2 换算</option></select><span class="mt-1 block text-xs text-gray-500">下方单价始终显示人民币，保留来源币种用于匹配。</span></label>
                <label class="text-sm">供应商地区<input v-model="draft.region" class="input mt-1" placeholder="例如 cn、global；留空按关联上游识别" /></label>
                <label v-if="draft.purchase_mode !== 'payg'" class="text-sm">{{ draft.purchase_mode === 'credit_pack' ? '额度包实付金额' : '周期实付费用' }}（元）<input :value="displayAmount(draft.period_cost)" :required="draft.purchase_mode !== 'subscription'" type="number" min="0" step="any" class="input mt-1" @input="draft.period_cost = readAmount($event)" /><span v-if="draft.purchase_mode === 'subscription'" class="mt-1 block text-xs text-gray-500">可选；不参与单次售价计算。</span></label>
                <label v-if="draft.purchase_mode === 'credit_pack'" class="text-sm">供应商额度（折合人民币）<input :value="displayAmount(draft.credit_amount)" required type="number" min="0.000001" step="any" class="input mt-1" @input="draft.credit_amount = readAmount($event)" /></label>
                <label v-if="draft.purchase_mode !== 'subscription'" class="text-sm">单价采购折扣<input v-model.number="draft.discount_multiplier" required type="number" min="0.000001" step="any" class="input mt-1" /><span class="mt-1 block text-xs text-gray-500">原价填 1，八折填 0.8；周期费用按实付记录。</span></label>
              </div>
              <label class="inline-flex items-center gap-2 text-sm"><input v-model="draft.enabled" type="checkbox" />启用此成本方案</label>
              <p v-if="draft.purchase_mode === 'subscription' || draft.purchase_mode === 'hybrid'" class="rounded-lg bg-gray-50 px-3 py-2 text-xs text-gray-500 dark:bg-dark-900">包月费用按周期记录，不能直接当作单次调用的实际成本。混合模式的预览只计算增量用量费用。</p>
              <p v-if="draft.purchase_mode === 'credit_pack' || draft.purchase_mode === 'hybrid'" class="text-xs text-amber-700 dark:text-amber-400">历史套餐保留用于核对。简化扣费需另建明确的按量基准价，停用旧方案后关联新方案。</p>
              <div class="grid gap-4 sm:grid-cols-2">
                <label class="text-sm">开始时间<input v-model="startTime" type="datetime-local" class="input mt-1" /></label>
                <label class="text-sm">结束时间<input v-model="endTime" type="datetime-local" class="input mt-1" /></label>
              </div>
              <label class="block text-sm">价格来源<input v-model="draft.source" maxlength="500" class="input mt-1" placeholder="官方价目链接、合同或采购单编号" /></label>
            </section>
            <section class="space-y-3">
              <div><h3 class="text-sm font-semibold">关联上游</h3><p class="mt-1 text-xs text-gray-500">账号授权方式与采购方式分开记录。未关联账号的方案可保存为模板。</p></div>
              <input v-model="accountQuery" class="input" placeholder="搜索上游账号" aria-label="搜索上游账号" />
              <div class="max-h-40 overflow-y-auto rounded-xl border border-gray-200 p-2 dark:border-dark-600">
                <label v-for="account in filteredAccounts" :key="account.id" class="flex cursor-pointer items-center gap-3 rounded-lg px-3 py-2 text-sm hover:bg-gray-50 dark:hover:bg-dark-700">
                  <input v-model="draft.account_ids" type="checkbox" :value="account.id" /><span class="min-w-0 flex-1 truncate">{{ account.name }}</span><span class="text-xs text-gray-400">{{ ['apikey', 'service_account', 'bedrock', 'upstream'].includes(account.type) ? 'API 接入' : '账号授权' }} · {{ account.platform }}</span>
                </label>
                <p v-if="!filteredAccounts.length" class="p-3 text-sm text-gray-400">暂无符合条件的上游账号。</p>
                <label v-for="id in orphanedAccountIDs" :key="id" class="flex items-center gap-3 px-3 py-2 text-sm text-amber-600"><input v-model="draft.account_ids" type="checkbox" :value="id" />账号 #{{ id }}（已移除或当前不可见）</label>
              </div>
            </section>
            <section class="space-y-3">
              <div class="flex items-center justify-between gap-3"><div><h3 class="text-sm font-semibold">{{ draft.purchase_mode === 'subscription' ? '账号计费参考价表' : '模型采购基准价表' }}</h3><p class="mt-1 text-xs text-gray-500">{{ draft.purchase_mode === 'subscription' ? '已知官方型号自动匹配参考价；自定义型号可在这里明确参考基准。' : '同价模型共用规则；规格或输入档位分别设置。' }}</p></div><button type="button" class="btn btn-secondary whitespace-nowrap text-xs" @click="addPrice">添加规则</button></div>
              <details><summary class="cursor-pointer text-xs text-primary-600">使用参考模板</summary><div class="mt-2 rounded-lg bg-gray-50 p-3 dark:bg-dark-900"><p class="text-xs text-gray-500">MiniMax 中国站语言与 H3 系列参考价。保存前请核对采购协议和最新官方价格；现有匹配规则会保留。</p><button type="button" class="btn btn-secondary mt-3 text-xs" :disabled="draft.currency !== 'CNY'" @click="addMiniMaxTemplate">追加 MiniMax 中国站价表</button><p v-if="draft.currency !== 'CNY'" class="mt-2 text-xs text-gray-500">该模板以人民币计价，请在人民币采购方案中使用。</p></div></details>
              <p v-if="!draft.prices.length" class="rounded-xl border border-dashed border-gray-200 p-5 text-center text-sm text-gray-500 dark:border-dark-600">尚未登记单价，调用预览会标为成本待配置。</p>
              <details v-for="(row, index) in draft.prices" :key="row.id" class="rounded-xl border border-gray-200 dark:border-dark-600" :open="index === draft.prices.length - 1">
                <summary class="cursor-pointer px-4 py-3 text-sm font-medium">{{ row.models.join('、') || `成本规则 ${index + 1}` }}<span class="ml-2 text-xs font-normal text-gray-500">{{ modalityLabels[row.modality] }} · {{ row.resolution || unitLabels[row.billing_unit] }}</span></summary>
                <div class="space-y-4 border-t border-gray-100 p-4 dark:border-dark-700">
                  <label class="block text-sm">模型名称<textarea :value="row.models.join('\n')" class="input mt-1" rows="2" required placeholder="每行一个模型；可使用末尾 * 匹配同系列" @input="setModels(row, $event)" /></label>
                  <div class="grid gap-3 sm:grid-cols-2">
                    <label class="text-sm">模型类型<select v-model="row.modality" class="input mt-1" aria-label="模型类型" @change="changeModality(row)"><option value="chat">文本</option><option value="image">图像</option><option value="video">视频</option></select></label>
                    <label class="text-sm">计费单位<select v-model="row.billing_unit" class="input mt-1" aria-label="计费单位"><option v-if="row.modality === 'chat' || row.modality === 'image'" value="per_million_tokens">每百万 tokens</option><option v-if="row.modality === 'image'" value="per_image">每张</option><option v-if="row.modality === 'video'" value="per_second">每秒</option><option value="per_request">每次请求</option></select></label>
                    <label v-if="row.modality === 'chat'" class="text-sm">服务档位<select v-model="row.service_tier" class="input mt-1" aria-label="服务档位"><option value="">标准</option><option value="priority">优先处理 priority</option><option value="flex">弹性处理 flex</option></select></label>
                    <label v-if="row.modality === 'chat' && row.billing_unit === 'per_request'" class="text-sm">调用用途<select v-model="row.operation" class="input mt-1" aria-label="调用用途"><option :value="undefined">普通模型请求</option><option value="alpha_search">Codex 独立搜索</option></select><span class="mt-1 block text-xs text-gray-500">独立搜索按调用次数计费，与普通文本请求分开。</span></label>
                    <label v-if="row.modality !== 'chat'" class="text-sm">分辨率<input v-model="row.resolution" class="input mt-1" placeholder="如 480P、768P、2K；留空适用全部" /></label>
                    <label v-if="row.modality !== 'chat'" class="text-sm">质量档位<input v-model="row.quality" class="input mt-1" placeholder="留空适用全部" /></label>
                  </div>
                  <template v-if="row.billing_unit === 'per_million_tokens'">
                    <div class="grid gap-3 sm:grid-cols-2"><label v-for="field in tokenFields" :key="field.key" class="text-sm">{{ field.label }}（元 / 百万 tokens）<input :value="displayAmount(row[field.key])" required type="number" min="0" step="any" class="input mt-1" placeholder="不收费填 0" @input="row[field.key] = readAmount($event)" /></label></div>
                    <div v-if="row.modality === 'image'" class="grid gap-3 sm:grid-cols-2"><label class="text-sm">图像输入（元 / 百万 tokens）<input :value="displayAmount(row.image_input_price)" type="number" min="0" step="any" class="input mt-1" placeholder="未确认可留空" @input="row.image_input_price = readAmount($event)" /></label><label class="text-sm">图像输出（元 / 百万 tokens）<input :value="displayAmount(row.image_output_price)" type="number" min="0" step="any" class="input mt-1" placeholder="未确认可留空" @input="row.image_output_price = readAmount($event)" /></label></div>
                    <details v-else><summary class="cursor-pointer text-xs text-gray-500">视觉输入基准价</summary><label class="mt-3 block text-sm">图像输入（元 / 百万 tokens）<input :value="displayAmount(row.image_input_price)" type="number" min="0" step="any" class="input mt-1" placeholder="与普通输入同价时填写相同单价" @input="row.image_input_price = readAmount($event)" /></label></details>
                    <details><summary class="cursor-pointer text-xs text-gray-500">按总输入用量分档（含缓存）</summary><div class="mt-3 grid gap-3 sm:grid-cols-2"><label class="text-sm">输入下限<input v-model.number="row.min_tokens" type="number" min="0" step="1" class="input mt-1" /></label><label class="text-sm">输入上限<input v-model.number="row.max_tokens" type="number" min="1" step="1" class="input mt-1" placeholder="不限" /></label></div><p class="mt-2 text-xs text-gray-500">下限为 0 时包含零输入，其余区间按大于下限、小于等于上限匹配。</p></details>
                  </template>
                  <template v-else><label class="block text-sm">单价（元 / {{ unitLabels[row.billing_unit] }}）<input :value="displayAmount(row.unit_price)" required type="number" min="0" step="any" class="input mt-1" @input="row.unit_price = readAmount($event)" /></label><details v-if="row.modality === 'image'" class="mt-3"><summary class="cursor-pointer text-xs text-gray-500">多参考图片附加单价</summary><label class="mt-3 block text-sm">首张之后每张参考图（元）<input :value="displayAmount(row.input_image_unit_price)" type="number" min="0" step="any" class="input mt-1" placeholder="不适用可留空" @input="row.input_image_unit_price = readAmount($event)" /></label></details></template>
                  <button type="button" class="text-xs text-red-600" @click="draft.prices.splice(index, 1)">删除这条规则</button>
                </div>
              </details>
            </section>
            <details><summary class="cursor-pointer text-sm text-gray-500">备注</summary><textarea v-model="draft.notes" maxlength="2000" rows="3" class="input mt-2" /></details>
          </fieldset>
          <p v-if="localError || error" role="alert" class="text-sm text-red-600">{{ localError || error }}</p>
        </form>
      </div>
      <footer class="flex flex-wrap items-center justify-between gap-3 border-t border-gray-100 bg-gray-50 px-6 py-4 dark:border-dark-700 dark:bg-dark-900"><p class="text-xs text-gray-500">启用简化计费后，关联的按量上游使用此基准价。</p><button form="cost-profile" type="submit" class="btn btn-primary" :disabled="saving">{{ saving ? '保存中…' : '保存基准价' }}</button></footer>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { Account } from '@/types'
import type { ProcurementPrice, ProcurementProfile } from '@/api/admin/billingManagement'
import { changeProfileCurrency, emptyPrice, emptyProfile, modalityLabels, procurementFromCny, procurementToCny, profileIssue, purchaseLabels, unitLabels } from './billingManagement'
import { useBillingDialog } from '@/composables/useBillingDialog'
import { appendMissingMiniMaxPrices, MINIMAX_COST_TEMPLATE_SOURCE } from './minimaxCostTemplate'
const props = defineProps<{ existing?: ProcurementProfile; accounts: Account[]; saving: boolean; error: string }>()
const emit = defineEmits<{ close: []; save: [profile: ProcurementProfile] }>()
const dialog = ref<HTMLElement | null>(null)
useBillingDialog(dialog, () => emit('close'), () => props.saving)
const draft = ref<ProcurementProfile>(props.existing ? JSON.parse(JSON.stringify(props.existing)) : emptyProfile())
watch(() => draft.value.currency, (_, previous) => changeProfileCurrency(draft.value, previous), { flush: 'sync' })
function displayAmount(value: number | null | undefined) { return procurementToCny(value, draft.value.currency) }
function readAmount(event: Event) {
  const value = (event.target as HTMLInputElement).value
  return procurementFromCny(value === '' ? null : Number(value), draft.value.currency)
}
const accountQuery = ref('')
const localError = ref('')
const filteredAccounts = computed(() => props.accounts.filter(account => `${account.name} ${account.platform}`.toLowerCase().includes(accountQuery.value.toLowerCase())))
const orphanedAccountIDs = computed(() => draft.value.account_ids.filter(id => !props.accounts.some(account => account.id === id)))
const tokenFields = [{ key: 'input_price', label: '输入' }, { key: 'output_price', label: '输出' }, { key: 'cache_read_price', label: '缓存读取' }, { key: 'cache_write_price', label: '缓存写入' }] as const
function localTime(value: string | null) {
  if (!value) return ''
  const date = new Date(value)
  return new Date(date.getTime() - date.getTimezoneOffset() * 60000).toISOString().slice(0, 16)
}
const startTime = ref(localTime(draft.value.valid_from))
const endTime = ref(localTime(draft.value.valid_until))
const initialStartTime = startTime.value
const initialEndTime = endTime.value
function addPrice() { draft.value.prices.push(emptyPrice()) }
function addMiniMaxTemplate() {
  if (draft.value.currency !== 'CNY') return
  draft.value.prices = appendMissingMiniMaxPrices(draft.value.prices)
  if (!draft.value.provider) draft.value.provider = 'MiniMax 中国站'
  if (!draft.value.name) draft.value.name = 'MiniMax 中国站采购参考'
  if (!draft.value.source) draft.value.source = MINIMAX_COST_TEMPLATE_SOURCE
}
function setModels(row: ProcurementPrice, event: Event) {
  row.models = [...new Set((event.target as HTMLTextAreaElement).value.split(/[\n,，]/).map(model => model.trim()).filter(Boolean))]
}
function changeModality(row: ProcurementPrice) {
  row.billing_unit = row.modality === 'chat' ? 'per_million_tokens' : row.modality === 'image' ? 'per_image' : 'per_second'
  row.resolution = ''; row.quality = ''
  row.service_tier = ''
  row.operation = undefined
}
function submit() {
  if (startTime.value !== initialStartTime) draft.value.valid_from = startTime.value ? new Date(startTime.value).toISOString() : null
  if (endTime.value !== initialEndTime) draft.value.valid_until = endTime.value ? new Date(endTime.value).toISOString() : null
  const normalized = JSON.parse(JSON.stringify(draft.value)) as ProcurementProfile
  if ((normalized.period_cost as unknown) === '') normalized.period_cost = null
  if ((normalized.credit_amount as unknown) === '') normalized.credit_amount = null
  for (const row of normalized.prices) {
    for (const key of ['max_tokens', 'image_input_price', 'image_output_price'] as const) if ((row[key] as unknown) === '') row[key] = null
  }
  if (normalized.purchase_mode === 'payg') { normalized.period_cost = null; normalized.credit_amount = null }
  if (normalized.purchase_mode !== 'credit_pack') normalized.credit_amount = null
  localError.value = profileIssue(normalized)
  if (!localError.value) emit('save', normalized)
}
</script>
