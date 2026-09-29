<template>
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4" @click.self="emit('close')">
    <section role="dialog" aria-modal="true" aria-labelledby="service-title" class="max-h-[90vh] w-full max-w-3xl overflow-y-auto rounded-xl bg-white p-6 shadow-xl dark:bg-dark-800">
      <div class="flex items-center justify-between"><h2 id="service-title" class="text-lg font-semibold">{{ model }} · 服务配置</h2><button type="button" class="btn btn-secondary" @click="emit('close')">关闭</button></div>
      <p class="my-3 text-sm text-gray-500">选择服务所属渠道，配置发布状态和分组开放范围。分组倍率继续生效；文本与图片价格使用现有定价。</p>
      <label class="input-label">服务渠道</label>
      <select v-model="selectedChannel" class="input" @change="selectChannel"><option value="">选择已创建的服务渠道</option><option v-for="c in localChannels" :key="c.id" :value="c.id">{{ c.name }}</option></select>
      <details v-if="unassignedGroups.length" class="mt-3 rounded-lg border p-3 dark:border-dark-600"><summary class="cursor-pointer text-sm text-primary-600">为尚未配置服务的分组创建渠道</summary><div class="mt-3 space-y-3"><input v-model="newName" class="input" placeholder="服务渠道名称" aria-label="新渠道名称" /><label v-for="g in unassignedGroups" :key="g.id" class="mr-3 inline-flex gap-2"><input v-model="newGroups" type="checkbox" :value="g.id" />{{ g.name }}</label><button class="btn btn-secondary" type="button" :disabled="saving || !newName.trim() || !newGroups.length" @click="createChannel">创建并继续配置</button></div></details>
      <p v-if="error && !selectedChannel" role="alert" class="mt-3 text-sm text-red-600">{{ error }}</p>
      <form v-if="selectedChannel" class="mt-5 space-y-5" @submit.prevent="save">
        <div><label class="input-label">发布状态</label><select v-model="policy.state" class="input"><option value="draft">草稿 · 停止新调用</option><option value="published">已发布 · 允许授权分组调用</option><option value="paused">暂停 · 已有任务仍可查询</option></select></div>
        <div v-if="kind === 'image'" class="space-y-3"><label class="block">允许的图片尺寸（逗号分隔，留空沿用模型支持范围）<input v-model="imageSizes" class="input" placeholder="例如 2K,4K 或 1024x1024,1536x1024" /></label><label class="block">单次图片数量上限（0 表示沿用上游限制）<input v-model.number="policy.max_images" type="number" min="0" max="15" class="input" /></label><p class="text-xs text-gray-500">尺寸必须是所选上游模型实际支持的参数；图片按次／用量价格在渠道基础定价中配置。</p></div>
        <template v-if="kind === 'video'">
          <p v-if="model !== 'MiniMax-H3'" class="text-amber-600">该视频模型的规格编辑尚未适配，请使用现有分组配置。</p>
          <template v-else>
            <div><label class="input-label">开放分辨率与基础售价（USD／秒）</label><div v-for="r in ['768P','2K']" :key="r" class="mb-2 flex items-center gap-4"><label class="w-24"><input v-model="policy.resolutions" type="checkbox" :value="r" /> {{ r }}</label><input v-model.number="prices[r]" type="number" min="0" step="0.000001" required class="input max-w-40" :aria-label="`${r} 每秒售价`" /></div><p class="text-xs text-gray-500">用户费用 = 基础售价 × 时长 × 有效倍率。这是对外售价，请按实际采购成本设置。</p></div>
            <div class="grid grid-cols-2 gap-4"><label>最短时长（秒）<input v-model.number="policy.min_duration" type="number" min="4" max="15" required class="input" /></label><label>最长时长（秒）<input v-model.number="policy.max_duration" type="number" min="4" max="15" required class="input" /></label></div>
            <div><label class="input-label">开放生成方式</label><label v-for="mode in videoModes" :key="mode.value" class="mr-4 inline-flex items-center gap-1"><input v-model="policy.modes" type="checkbox" :value="mode.value" />{{ mode.label }}</label></div>
          </template>
        </template>
        <div><h3 class="font-semibold">分组开放范围</h3><p class="mb-3 text-xs text-gray-500">仅显示渠道已关联、平台匹配的分组。新配置默认不向任何组开放。</p>
          <div v-for="g in eligibleGroups" :key="g.id" class="mb-3 rounded-lg border p-3 dark:border-dark-600">
            <label class="flex items-center gap-2"><input v-model="rules[g.id]!.enabled" type="checkbox" />{{ g.name }} <span class="text-xs text-gray-500">分组倍率 {{ g.rate_multiplier }}×</span></label>
            <div v-if="kind === 'video' && rules[g.id]!.enabled" class="mt-3 space-y-3">
              <div><label v-for="r in policy.resolutions" :key="r" class="mr-4"><input v-model="rules[g.id]!.resolutions" type="checkbox" :value="r" /> {{ r }}</label></div>
              <label class="flex items-center gap-3">最长时长 <input v-model.number="rules[g.id]!.max_duration" class="input max-w-24" type="number" :min="policy.min_duration" :max="policy.max_duration" required /> 秒</label>
              <div><label v-for="mode in videoModes.filter(m => policy.modes?.includes(m.value))" :key="mode.value" class="mr-4"><input v-model="rules[g.id]!.modes" type="checkbox" :value="mode.value" /> {{ mode.label }}</label></div>
            </div>
          </div>
          <p v-if="!eligibleGroups.length" class="text-sm text-amber-600">该渠道尚未关联此平台分组，请先在服务与定价中配置。</p>
        </div>
        <p v-if="error" role="alert" class="text-sm text-red-600">{{ error }}</p>
        <div class="flex justify-end gap-3"><router-link to="/admin/channels/pricing" class="btn btn-secondary">渠道与基础定价</router-link><button class="btn btn-primary" :disabled="saving || !eligibleGroups.length || (kind === 'video' && model !== 'MiniMax-H3')">{{ saving ? '保存中…' : '保存服务配置' }}</button></div>
      </form>
    </section>
  </div>
</template>
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import channelsAPI, { type Channel } from '@/api/admin/channels'
import { getAll } from '@/api/admin/groups'
import type { AdminGroup } from '@/types'
import { defaultModelPolicy, videoModes, type ModelServicePolicy, type ModelServiceRule } from '@/types/modelService'
import { extractApiErrorMessage } from '@/utils/apiError'
const props = defineProps<{ model: string; platform: string; kind: ModelServicePolicy['kind']; channels: Channel[] }>()
const emit = defineEmits<{ close: []; saved: [] }>()
const localChannels = ref<Channel[]>([...props.channels])
const newName = ref(`${props.model} 服务`)
const newGroups = ref<number[]>([])
const unassignedGroups = computed(() => groups.value.filter(g => g.platform === props.platform && !localChannels.value.some(c => c.group_ids.includes(g.id))))
const selectedChannel = ref<number | ''>('')
const groups = ref<AdminGroup[]>([])
const policy = ref(defaultModelPolicy(props.model, props.platform, props.kind))
const rules = ref<Record<string, ModelServiceRule>>({})
const prices = ref<Record<string, number>>({ '768P': 0.08, '2K': 0.13 })
const imageSizes = ref('')
const saving = ref(false)
const error = ref('')
const eligibleGroups = computed(() => groups.value.filter(g => g.platform === props.platform && localChannels.value.find(c => c.id === selectedChannel.value)?.group_ids.includes(g.id)))
function selectChannel() {
  const c = localChannels.value.find(c => c.id === selectedChannel.value)
  const existing = (c?.features_config?.model_services as ModelServicePolicy[] | undefined)?.find(p => p.model === props.model && p.platform === props.platform)
  policy.value = existing ? JSON.parse(JSON.stringify(existing)) : defaultModelPolicy(props.model, props.platform, props.kind)
  prices.value = { '768P': 0.08, '2K': 0.13, ...policy.value.prices }
  rules.value = Object.fromEntries(eligibleGroups.value.map(g => [g.id, policy.value.groups?.[g.id] || { enabled: false, resolutions: ['768P'], max_duration: 5, modes: ['text'] }]))
  imageSizes.value = policy.value.image_sizes?.join(',') || ''
  error.value = ''
}
async function createChannel() {
 saving.value = true; error.value = ''
 try { const channel = await channelsAPI.create({ name: newName.value.trim(), group_ids: newGroups.value, model_pricing: [], model_mapping: {}, restrict_models: false }); localChannels.value.push(channel); selectedChannel.value = channel.id; selectChannel() } catch (e) { error.value = extractApiErrorMessage(e, '创建失败') } finally { saving.value = false }
}
async function save() {
  if (!selectedChannel.value) return
  saving.value = true; error.value = ''
  try {
    const channel = await channelsAPI.getById(selectedChannel.value)
    const policies = (channel.features_config?.model_services as ModelServicePolicy[] | undefined) || []
    const next = { ...policy.value, image_sizes: imageSizes.value.split(/[,，]/).map(s => s.trim()).filter(Boolean), groups: rules.value, ...(props.kind === 'video' ? { prices: prices.value } : {}) }
    await channelsAPI.update(channel.id, { features_config: { ...channel.features_config, model_services: [...policies.filter(p => p.model !== props.model || p.platform !== props.platform), next] } })
    emit('saved')
  } catch (e) { error.value = extractApiErrorMessage(e, '保存失败') } finally { saving.value = false }
}
onMounted(async () => { try { groups.value = await getAll(); const c = localChannels.value.find(c => (c.features_config?.model_services as ModelServicePolicy[] | undefined)?.some(p => p.model === props.model && p.platform === props.platform)); if (c) { selectedChannel.value = c.id; selectChannel() } } catch (e) { error.value = extractApiErrorMessage(e, '分组加载失败') } })
</script>
