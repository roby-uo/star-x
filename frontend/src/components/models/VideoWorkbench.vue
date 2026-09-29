<template>
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4" @click.self="!busy && emit('close')">
    <section role="dialog" aria-modal="true" aria-labelledby="video-title" class="max-h-[90vh] w-full max-w-2xl overflow-y-auto rounded-xl bg-white p-6 dark:bg-dark-800">
      <div class="mb-4 flex items-center justify-between"><h2 id="video-title" class="text-lg font-semibold">{{ model.name }} · {{ model.group_name }}</h2><button class="btn btn-secondary" :disabled="busy" @click="emit('close')">关闭</button></div>
      <p class="mb-4 text-sm text-gray-500">先选择规格并获取报价，再提交生成。任务受理后按报价计费；生成失败时需根据上游账单核对费用。</p>
      <form class="space-y-4" @submit.prevent="getQuote">
        <label class="block">调用密钥<select v-model="keyID" class="input" required :disabled="busy"><option value="">请选择</option><option v-for="k in keys" :key="k.id" :value="k.id">{{ k.name }}</option></select></label>
        <p v-if="!keys.length" class="text-sm text-amber-600">此分组暂无可用密钥，请先在 API 密钥管理中创建或启用。</p>
        <label class="block">生成方式<select v-model="mode" class="input" :disabled="busy"><option v-for="m in allowedModes" :key="m.value" :value="m.value">{{ m.label }}</option></select></label>
        <label class="block">提示词<textarea v-model="prompt" required maxlength="10000" class="input" rows="4" :disabled="busy" placeholder="描述主体、动作、场景和镜头运动" /></label>
        <div v-if="mode !== 'text'" class="space-y-2"><label class="block">首帧图片 URL<input v-model="firstFrame" class="input" placeholder="https://… 或上传图片" :disabled="busy" required /></label><input type="file" accept="image/jpeg,image/png,image/webp" :disabled="busy" aria-label="上传首帧" @change="readImage($event, 'first')" /></div>
        <div v-if="mode === 'first_last_frame'" class="space-y-2"><label class="block">尾帧图片 URL<input v-model="lastFrame" class="input" placeholder="https://… 或上传图片" :disabled="busy" required /></label><input type="file" accept="image/jpeg,image/png,image/webp" :disabled="busy" aria-label="上传尾帧" @change="readImage($event, 'last')" /></div>
        <div class="grid grid-cols-3 gap-3"><label>分辨率<select v-model="resolution" class="input" :disabled="busy"><option v-for="r in resolutions" :key="r">{{ r }}</option></select></label><label>时长（秒）<select v-model.number="duration" class="input" :disabled="busy"><option v-for="d in durations" :key="d" :value="d">{{ d }}</option></select></label><label>画幅<select v-model="ratio" class="input" :disabled="busy || mode !== 'text'"><option v-for="r in ['21:9','16:9','4:3','1:1','3:4','9:16']" :key="r">{{ r }}</option></select><span v-if="mode !== 'text'" class="text-xs text-gray-500">跟随输入图</span></label></div>
        <p v-if="error" role="alert" class="text-sm text-red-600">{{ error }}</p>
        <div v-if="quote" class="rounded-lg bg-primary-50 p-4 text-primary-900 dark:bg-primary-900/20 dark:text-primary-200"><strong>本次费用 ${{ quote.total.toFixed(4) }}</strong><p class="mt-1 text-xs">${{ quote.unit_price }}／秒 × {{ duration }} 秒 × {{ quote.multiplier }} 倍率 · {{ quote.currency }}</p></div>
        <div class="flex flex-wrap justify-end gap-3"><button type="button" class="btn btn-secondary" @click="showExample = !showExample">API 调用示例</button><button class="btn btn-secondary" :disabled="busy || !keyID">{{ quoting ? '报价中…' : '获取最新报价' }}</button><button type="button" class="btn btn-primary" :disabled="busy || !quote || submitted" @click="generate">{{ submitting ? '提交中…' : submitted ? '已提交，请查看任务记录' : `生成视频${quote ? ` · $${quote.total.toFixed(4)}` : ''}` }}</button></div>
        <pre v-if="showExample" class="overflow-x-auto rounded-lg bg-gray-900 p-4 text-xs text-gray-100">{{ example }}</pre>
      </form>
    </section>
  </div>
</template>
<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import type { UserAvailableModel } from '@/api/channels'
import { list } from '@/api/keys'
import { apiClient } from '@/api/client'
import { buildGatewayUrl } from '@/api/url'
import type { ApiKey } from '@/types'
import { videoModes, type VideoQuote } from '@/types/modelService'
const props = defineProps<{ model: UserAvailableModel }>()
const emit = defineEmits<{ close: []; submitted: [] }>()
const keys = ref<ApiKey[]>([])
const keyID = ref<number | ''>('')
const keyRules = ref<{ restrict_models?: boolean; models?: string[]; video_resolutions?: string[]; video_max_duration?: number }>({})
const resolutions = computed(() => (props.model.policy?.resolutions || ['768P', '2K']).filter(r => !keyRules.value.video_resolutions?.length || keyRules.value.video_resolutions.includes(r)))
const allowedModes = videoModes.filter(m => !props.model.policy?.modes || props.model.policy.modes.includes(m.value))
const durations = computed(() => Array.from({ length: Math.max(0, Math.min(props.model.policy?.max_duration || 15, keyRules.value.video_max_duration || 15) - (props.model.policy?.min_duration || 4) + 1) }, (_, i) => i + (props.model.policy?.min_duration || 4)))
const resolution = ref(resolutions.value[0] || '768P')
const duration = ref(durations.value.includes(5) ? 5 : durations.value[0] || 4)
const mode = ref(allowedModes[0]?.value || 'text')
const prompt = ref('')
const firstFrame = ref('')
const lastFrame = ref('')
const ratio = ref('16:9')
const quote = ref<VideoQuote | null>(null)
const error = ref('')
const quoting = ref(false)
const submitting = ref(false)
const submitted = ref(false)
const busy = computed(() => quoting.value || submitting.value)
const showExample = ref(false)
let idempotency = crypto.randomUUID()
let revision = 0
const payload = computed(() => ({ model: props.model.name, resolution: resolution.value, duration: duration.value, ratio: mode.value === 'text' ? ratio.value : 'adaptive', content: [ { type: 'text', text: prompt.value }, ...(mode.value !== 'text' ? [{ type: 'image_url', role: 'first_frame', image_url: { url: firstFrame.value } }] : []), ...(mode.value === 'first_last_frame' ? [{ type: 'image_url', role: 'last_frame', image_url: { url: lastFrame.value } }] : []) ] }))
const example = computed(() => `POST /v2/video_generation\nAuthorization: Bearer YOUR_API_KEY\nContent-Type: application/json\nIdempotency-Key: YOUR_UNIQUE_REQUEST_ID\n\n${JSON.stringify({ ...payload.value, content: payload.value.content.map(item => 'image_url' in item && item.image_url?.url.startsWith('data:') ? { ...item, image_url: { url: 'https://example.com/frame.png' } } : item) }, null, 2)}\n\nGET /v2/query/video_generation/{task_id}`)
watch(keyID, async (id) => {
 if (!id) return
 try { const { data } = await apiClient.get(`/keys/${id}/model-access`); if (keyID.value !== id) return; keyRules.value = data
 if (!resolutions.value.includes(resolution.value)) resolution.value = resolutions.value[0] || ''
 if (!durations.value.includes(duration.value)) duration.value = durations.value[0] || 0
 if (data.restrict_models && !data.models?.includes(props.model.name)) error.value = '此密钥未授权调用该模型，请更换密钥'
 } catch { error.value = '密钥权限读取失败' }
})
watch([payload, keyID], () => { quote.value = null; revision++; idempotency = crypto.randomUUID(); submitted.value = false }, { deep: true })
async function request(path: string, extra: Record<string, string> = {}) {
  const key = keys.value.find(k => k.id === keyID.value)
  if (!key) throw new Error('请选择有效密钥')
  const response = await fetch(buildGatewayUrl(path), { method: 'POST', headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${key.key}`, ...extra }, body: JSON.stringify(payload.value) })
  const data = await response.json()
  if (!response.ok) throw new Error(data.error?.message || data.message || '请求失败')
  return data
}
async function getQuote() {
  quoting.value = true; error.value = ''; quote.value = null
  const current = revision
  try { const result = await request('/v2/video_generation/quote'); if (current === revision) quote.value = result } catch (e) { error.value = e instanceof Error ? e.message : '报价失败' } finally { quoting.value = false }
}
async function generate() {
  if (!quote.value || busy.value || submitted.value) return
  submitting.value = true; error.value = ''
  try { await request('/v2/video_generation', { 'X-Video-Quote': quote.value.version, 'Idempotency-Key': idempotency }); submitted.value = true; emit('submitted') } catch (e) { error.value = e instanceof Error ? e.message : '提交失败，请先检查任务记录'; emit('submitted') } finally { submitting.value = false }
}
async function readImage(event: Event, target: 'first' | 'last') {
  const file = (event.target as HTMLInputElement).files?.[0]
  if (!file) return
  if (file.size > 10 * 1024 * 1024) { error.value = '页面上传限 10 MB；更大图片请填写公开 URL'; return }
  const reader = new FileReader()
  reader.onload = () => { if (target === 'first') firstFrame.value = String(reader.result); else lastFrame.value = String(reader.result) }
  reader.onerror = () => { error.value = '读取图片失败' }
  reader.readAsDataURL(file)
}
onMounted(async () => { try { const result = await list(1, 100, { group_id: props.model.group_id, status: 'active' }); keys.value = result.items; if (keys.value[0]) keyID.value = keys.value[0].id } catch { error.value = '密钥列表加载失败' } })
</script>
