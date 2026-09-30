<template>
  <div :class="embedded ? '' : 'fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4'" @click.self="!busy && !embedded && emit('close')">
    <section :role="embedded ? undefined : 'dialog'" :aria-modal="embedded ? undefined : true" :aria-labelledby="embedded ? undefined : 'video-title'" :class="embedded ? 'space-y-5' : 'max-h-[90vh] w-full max-w-2xl overflow-y-auto rounded-2xl bg-white p-6 dark:bg-dark-800'">
      <div v-if="!embedded" class="mb-4 flex items-center justify-between gap-3">
        <div><h2 id="video-title" class="text-lg font-semibold">视频在线测试</h2><p class="mt-1 text-sm text-gray-500">{{ model.name }} · {{ model.group_name }}</p></div>
        <button class="btn btn-secondary" :disabled="busy" @click="emit('close')">关闭</button>
      </div>
      <p class="mb-4 text-sm text-gray-500">用于验证 API 接入。视频异步生成，提交后可在当前页面查询结果；接入自己的应用时，通过任务编号查询状态。</p>
      <form class="space-y-4" @submit.prevent="generate">
        <label v-if="!embedded" class="block text-sm">调用密钥<select v-model="keyID" class="input mt-1" required :disabled="formLocked"><option value="">请选择</option><option v-for="key in keys" :key="key.id" :value="key.id">{{ key.name }}</option></select></label>
        <p v-if="!keys.length" class="text-sm text-amber-600">此分组暂无可用密钥，请先在 API 密钥管理中创建或启用。</p>
        <label class="block text-sm">生成方式<select v-model="mode" class="input mt-1" :disabled="formLocked"><option v-for="item in allowedModes" :key="item.value" :value="item.value">{{ item.label }}</option></select></label>
        <label class="block text-sm">提示词<textarea v-model="prompt" required maxlength="10000" class="input mt-1" rows="4" :disabled="formLocked" placeholder="描述主体、动作、场景和镜头运动" /></label>
        <div v-if="mode !== 'text'" class="space-y-2"><label class="block text-sm">首帧图片 URL<input v-model="firstFrame" class="input mt-1" placeholder="https://… 或上传图片" :disabled="formLocked" required /></label><input type="file" accept="image/jpeg,image/png,image/webp" :disabled="formLocked" aria-label="上传首帧" @change="readImage($event, 'first')" /></div>
        <div v-if="mode === 'first_last_frame'" class="space-y-2"><label class="block text-sm">尾帧图片 URL<input v-model="lastFrame" class="input mt-1" placeholder="https://… 或上传图片" :disabled="formLocked" required /></label><input type="file" accept="image/jpeg,image/png,image/webp" :disabled="formLocked" aria-label="上传尾帧" @change="readImage($event, 'last')" /></div>
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
          <label class="text-sm">分辨率<select v-model="resolution" class="input mt-1" :disabled="formLocked"><option v-for="value in resolutions" :key="value">{{ value }}</option></select></label>
          <label class="text-sm">时长（秒）<select v-model.number="duration" class="input mt-1" :disabled="formLocked"><option v-for="value in durations" :key="value" :value="value">{{ value }}</option></select></label>
          <label class="text-sm">画幅<select v-model="ratio" class="input mt-1" :disabled="formLocked || mode !== 'text'"><option v-for="value in ['21:9', '16:9', '4:3', '1:1', '3:4', '9:16']" :key="value">{{ value }}</option></select><span v-if="mode !== 'text'" class="text-xs text-gray-500">跟随输入图</span></label>
        </div>
        <label class="flex items-start gap-2 text-sm"><input v-model="accepted" type="checkbox" class="mt-0.5" required :disabled="formLocked" />我确认在线测试会消耗账户额度</label>
        <p v-if="error" role="alert" class="text-sm text-red-600">{{ error }}</p>
        <div class="flex flex-wrap justify-end gap-3">
          <button v-if="!embedded" type="button" class="btn btn-secondary" @click="showExample = !showExample">API 调用示例</button>
          <button class="btn btn-primary" :disabled="formLocked || !keyID || !accepted || !validInput">{{ quoting ? '校验调用配置…' : submitting ? '提交中…' : submitted ? '已提交' : '开始测试' }}</button>
        </div>
        <pre v-if="!embedded && showExample" class="overflow-x-auto rounded-lg bg-gray-900 p-4 text-xs text-gray-100">{{ example }}</pre>
      </form>
      <section v-if="submitted" aria-label="本次测试结果" class="mt-5 space-y-3 rounded-xl border border-gray-200 bg-gray-50 p-4 dark:border-dark-600 dark:bg-dark-900">
        <div class="flex flex-wrap items-center justify-between gap-2"><h3 class="font-semibold">本次测试结果</h3><span class="text-sm">{{ states[taskState] || taskState }}</span></div>
        <dl class="space-y-1 text-xs text-gray-500">
          <div v-if="upstreamTaskID" class="flex flex-wrap gap-2"><dt>任务编号</dt><dd class="break-all font-mono">{{ upstreamTaskID }}</dd></div>
          <div v-if="internalTaskID" class="flex flex-wrap gap-2"><dt>记录编号</dt><dd class="break-all font-mono">{{ internalTaskID }}</dd></div>
          <div v-if="taskState === 'uncertain'" class="flex flex-wrap gap-2"><dt>请求编号</dt><dd class="break-all font-mono">{{ idempotency }}</dd></div>
        </dl>
        <p v-if="taskState === 'uncertain'" class="text-sm text-amber-600">提交结果待核对，请保留上述编号并联系管理员，避免重复提交。</p>
        <p v-else-if="!terminal" class="text-sm text-gray-500">{{ autoPollingStopped ? '自动查询已停止，可手动查询状态。' : '正在定期查询状态。关闭页面后，应用仍可通过 API 查询本次任务。' }}</p>
        <p v-if="queryError" role="alert" class="text-sm text-amber-600">{{ queryError }}</p>
        <video v-if="resultURL" :src="resultURL" controls preload="none" class="max-h-80 w-full rounded-lg" />
        <div class="flex flex-wrap gap-2">
          <button v-if="upstreamTaskID" type="button" class="btn btn-secondary text-sm" :disabled="refreshing || busy" @click="refreshTask">{{ refreshing ? '查询中…' : '查询状态' }}</button>
          <a v-if="resultURL" :href="resultURL" target="_blank" rel="noopener noreferrer" class="btn btn-secondary text-sm">打开视频</a>
          <button v-if="terminal" type="button" class="btn btn-secondary text-sm" :disabled="refreshing || busy" @click="resetTest">开始新的测试</button>
        </div>
      </section>
    </section>
  </div>
</template>
<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import type { UserAvailableModel } from '@/api/channels'
import { list } from '@/api/keys'
import { apiClient } from '@/api/client'
import { buildGatewayUrl } from '@/api/url'
import type { ApiKey } from '@/types'
import { videoModes, type VideoQuote } from '@/types/modelService'
import { miniMaxVideoSpec } from '@/utils/minimaxVideo'

const props = defineProps<{ model: UserAvailableModel; embedded?: boolean; selectedKeyId?: number; contextKeys?: ApiKey[] }>()
const emit = defineEmits<{ close: []; submitted: []; busy: [value: boolean] }>()
const keys = ref<ApiKey[]>([])
const keyID = ref<number | ''>('')
const keyRules = ref<{ restrict_models?: boolean; models?: string[]; video_resolutions?: string[]; video_max_duration?: number }>({})
const rulesLoading = ref(false)
const rulesValid = ref(false)
const videoSpec = computed(() => props.model.platform === 'openai' ? miniMaxVideoSpec(props.model.name) : undefined)
const resolutions = computed(() => (videoSpec.value?.resolutions || []).filter(value => (!props.model.policy?.resolutions || props.model.policy.resolutions.includes(value)) && (!keyRules.value.video_resolutions?.length || keyRules.value.video_resolutions.includes(value))))
const allowedModes = computed(() => videoModes.filter(item => videoSpec.value?.modes.includes(item.value) && (!props.model.policy?.modes || props.model.policy.modes.includes(item.value))))
const durations = computed(() => {
  if (!videoSpec.value) return []
  const min = Math.max(videoSpec.value.minDuration, props.model.policy?.min_duration || videoSpec.value.minDuration)
  const max = Math.min(videoSpec.value.maxDuration, props.model.policy?.max_duration || videoSpec.value.maxDuration, keyRules.value.video_max_duration || videoSpec.value.maxDuration)
  return Array.from({ length: Math.max(0, max - min + 1) }, (_, index) => min + index)
})
const resolution = ref(resolutions.value[0] || '')
const duration = ref(durations.value.includes(5) ? 5 : durations.value[0] || 0)
const mode = ref(allowedModes.value[0]?.value || 'text')
const prompt = ref('')
const firstFrame = ref('')
const lastFrame = ref('')
const ratio = ref('16:9')
const quote = ref<VideoQuote | null>(null)
const accepted = ref(false)
const error = ref('')
const querying = ref(false)
const quoting = ref(false)
const submitting = ref(false)
const submitted = ref(false)
const busy = computed(() => quoting.value || submitting.value || rulesLoading.value)
const formLocked = computed(() => busy.value || submitted.value)
const showExample = ref(false)
const internalTaskID = ref('')
const upstreamTaskID = ref('')
const taskState = ref('')
const resultURL = ref('')
const queryError = ref('')
const autoPollingStopped = ref(false)
const refreshing = computed(() => querying.value)
const terminal = computed(() => ['succeeded', 'failed', 'cancelled', 'rejected'].includes(taskState.value))
const states: Record<string, string> = { submitting: '提交中', uncertain: '提交结果待核对', queued: '排队中', running: '生成中', succeeded: '已完成', failed: '生成失败', cancelled: '已取消', rejected: '未受理' }
const idempotency = ref(crypto.randomUUID())
let revision = 0
let pollTimer: ReturnType<typeof setTimeout> | undefined
let pollingAttempts = 0
let disposed = false
let taskKey = ''
const payload = computed(() => ({ model: props.model.name, resolution: resolution.value, duration: duration.value, ratio: mode.value === 'text' ? ratio.value : 'adaptive', content: [{ type: 'text', text: prompt.value }, ...(mode.value !== 'text' ? [{ type: 'image_url', role: 'first_frame', image_url: { url: firstFrame.value } }] : []), ...(mode.value === 'first_last_frame' ? [{ type: 'image_url', role: 'last_frame', image_url: { url: lastFrame.value } }] : [])] }))
const validInput = computed(() => rulesValid.value && !!prompt.value.trim() && resolutions.value.includes(resolution.value) && durations.value.includes(duration.value) && allowedModes.value.some(item => item.value === mode.value) && (mode.value === 'text' || !!firstFrame.value.trim()) && (mode.value !== 'first_last_frame' || !!lastFrame.value.trim()))
const example = computed(() => `POST /v2/video_generation\nAuthorization: Bearer YOUR_API_KEY\nContent-Type: application/json\nIdempotency-Key: YOUR_UNIQUE_REQUEST_ID\n\n${JSON.stringify({ ...payload.value, content: payload.value.content.map(item => 'image_url' in item && item.image_url?.url.startsWith('data:') ? { ...item, image_url: { url: 'https://example.com/frame.png' } } : item) }, null, 2)}\n\nGET /v2/query/video_generation/{task_id}`)

watch(() => busy.value || querying.value, value => emit('busy', value))
watch(() => props.selectedKeyId, value => {
  if (!formLocked.value && value && keys.value.some(key => key.id === value)) keyID.value = value
})
watch(keyID, async id => {
  rulesValid.value = false
  keyRules.value = {}
  error.value = ''
  if (!id) return
  rulesLoading.value = true
  try {
    const { data } = await apiClient.get(`/keys/${id}/model-access`)
    if (keyID.value !== id || disposed) return
    keyRules.value = data
    if (!resolutions.value.includes(resolution.value)) resolution.value = resolutions.value[0] || ''
    if (!durations.value.includes(duration.value)) duration.value = durations.value[0] || 0
    if (data.restrict_models && !data.models?.includes(props.model.name)) error.value = '此密钥未授权调用该模型，请更换密钥'
    else rulesValid.value = true
  } catch { if (keyID.value === id) error.value = '密钥权限读取失败，请重新选择密钥' }
  finally { if (keyID.value === id) rulesLoading.value = false }
})
watch([payload, keyID], () => {
  quote.value = null
  accepted.value = false
  revision++
  if (!submitted.value) idempotency.value = crypto.randomUUID()
}, { deep: true })

interface GatewayResult { id?: string; task_id?: string; state?: string; error?: { message?: string }; message?: string }
class GatewayError extends Error {
  constructor(message: string, public status: number, public result: GatewayResult) { super(message) }
}
async function createRequest(path: string, key: string, requestPayload: typeof payload.value, extra: Record<string, string> = {}) {
  const response = await fetch(buildGatewayUrl(path), { method: 'POST', headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${key}`, ...extra }, body: JSON.stringify(requestPayload) })
  const data = await response.json()
  if (!response.ok) throw new GatewayError(data.error?.message || data.message || '请求失败', response.status, data)
  return data
}
async function generate() {
  if (formLocked.value || !accepted.value || !validInput.value) return
  const key = keys.value.find(item => item.id === keyID.value)
  if (!key) return
  const current = revision
  const requestPayload = payload.value
  quoting.value = true
  error.value = ''
  try {
    quote.value = await createRequest('/v2/video_generation/quote', key.key, requestPayload) as VideoQuote
    if (current !== revision || disposed) return
    if (!quote.value || typeof quote.value.version !== 'string' || !quote.value.version) throw new Error('调用配置校验未返回有效凭证，请稍后重试')
    submitting.value = true
    submitted.value = true
    taskKey = key.key
    taskState.value = 'submitting'
    try {
      const result = await createRequest('/v2/video_generation', key.key, requestPayload, { 'X-Video-Quote': quote.value.version, 'Idempotency-Key': idempotency.value }) as GatewayResult
      internalTaskID.value = result.id || ''
      upstreamTaskID.value = result.task_id || ''
      taskState.value = result.state || (result.task_id ? 'queued' : 'uncertain')
      if (!upstreamTaskID.value && !terminal.value) taskState.value = 'uncertain'
      emit('submitted')
      schedulePoll()
    } catch (cause) {
      if (cause instanceof GatewayError) {
        internalTaskID.value = cause.result.id || ''
        upstreamTaskID.value = cause.result.task_id || ''
        if (cause.status >= 400 && cause.status < 500 && !cause.result.id && !cause.result.task_id) {
          submitted.value = false
          taskState.value = ''
          error.value = cause.message
          accepted.value = false
          quote.value = null
          return
        }
      }
      taskState.value = 'uncertain'
      error.value = '提交结果待核对，请保留本次请求编号，避免重复提交。'
      emit('submitted')
      if (upstreamTaskID.value) schedulePoll()
    } finally { submitting.value = false }
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : '调用配置校验失败'
    quote.value = null
  } finally { quoting.value = false }
}
function clearPoll() { if (pollTimer) clearTimeout(pollTimer); pollTimer = undefined }
function schedulePoll() {
  clearPoll()
  if (disposed || terminal.value || !upstreamTaskID.value) return
  if (pollingAttempts >= 60) { autoPollingStopped.value = true; return }
  pollTimer = setTimeout(() => { pollingAttempts++; void refreshTask() }, 10000)
}
async function refreshTask() {
  if (!upstreamTaskID.value || querying.value || !taskKey || disposed) return
  clearPoll()
  querying.value = true
  queryError.value = ''
  try {
    const response = await fetch(buildGatewayUrl(`/v2/query/video_generation/${encodeURIComponent(upstreamTaskID.value)}`), { headers: { Authorization: `Bearer ${taskKey}` } })
    const data = await response.json()
    if (disposed) return
    if (!response.ok) throw new Error(data.error?.message || '状态查询失败')
    taskState.value = data.task?.status || taskState.value
    resultURL.value = safeVideoURL(data.task?.content?.url)
  } catch (cause) { if (!disposed) queryError.value = cause instanceof Error ? cause.message : '状态查询失败，可稍后手动重试' }
  finally { querying.value = false; schedulePoll() }
}
function safeVideoURL(raw: unknown) {
  if (typeof raw !== 'string') return ''
  try { const url = new URL(raw); return url.protocol === 'https:' && !url.username && !url.password ? url.href : '' } catch { return '' }
}
function resetTest() {
  if (!terminal.value || querying.value || busy.value) return
  clearPoll()
  submitted.value = false
  internalTaskID.value = ''
  upstreamTaskID.value = ''
  taskState.value = ''
  resultURL.value = ''
  queryError.value = ''
  error.value = ''
  accepted.value = false
  quote.value = null
  taskKey = ''
  pollingAttempts = 0
  autoPollingStopped.value = false
  idempotency.value = crypto.randomUUID()
}
async function readImage(event: Event, target: 'first' | 'last') {
  if (formLocked.value) return
  const file = (event.target as HTMLInputElement).files?.[0]
  if (!file) return
  if (file.size > 10 * 1024 * 1024) { error.value = '页面上传限 10 MB；更大图片请填写公开 URL'; return }
  const reader = new FileReader()
  reader.onload = () => { if (formLocked.value || disposed) return; if (target === 'first') firstFrame.value = String(reader.result); else lastFrame.value = String(reader.result) }
  reader.onerror = () => { error.value = '读取图片失败' }
  reader.readAsDataURL(file)
}
onMounted(async () => {
  try {
    const available = props.contextKeys || (await list(1, 100, { group_id: props.model.group_id, status: 'active' })).items
    if (disposed) return
    keys.value = available
    keyID.value = keys.value.some(key => key.id === props.selectedKeyId) ? props.selectedKeyId! : keys.value[0]?.id || ''
  } catch { error.value = '密钥列表加载失败' }
})
onUnmounted(() => { disposed = true; clearPoll(); emit('busy', false) })
</script>
