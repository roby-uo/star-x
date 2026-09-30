<template>
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-3 sm:p-6" @click.self="close">
    <section role="dialog" aria-modal="true" aria-labelledby="model-details-title" class="flex max-h-[92vh] w-full max-w-3xl flex-col overflow-hidden rounded-2xl bg-white shadow-xl dark:bg-dark-800">
      <header class="flex items-start justify-between gap-4 border-b border-gray-100 px-5 py-5 dark:border-dark-700 sm:px-6">
        <div class="min-w-0">
          <p class="mb-2 text-xs text-gray-500">{{ modelSupplier(item.name, item.platform) }} · {{ catalogTypeLabels[item.type] }}</p>
          <h2 id="model-details-title" class="break-words text-lg font-semibold text-gray-900 dark:text-white">{{ item.name }}</h2>
        </div>
        <button class="rounded-lg p-2 text-gray-500 hover:bg-gray-100 disabled:opacity-40 dark:hover:bg-dark-700" :disabled="busy" aria-label="关闭模型详情" @click="close"><Icon name="x" size="md" /></button>
      </header>
      <div class="overflow-y-auto px-5 pb-6 sm:px-6">
        <div class="grid gap-3 py-5 sm:grid-cols-2">
          <label class="text-xs font-medium text-gray-500">调用分组
            <select v-model.number="groupId" class="input mt-2" :disabled="busy"><option v-for="group in item.groups" :key="group.group_id" :value="group.group_id">{{ group.group_name }}</option></select>
          </label>
          <label class="text-xs font-medium text-gray-500">调用密钥
            <select v-model.number="keyId" class="input mt-2" :disabled="busy || keysLoading"><option :value="0">{{ keysLoading ? '加载密钥…' : '请选择密钥' }}</option><option v-for="key in keys" :key="key.id" :value="key.id">{{ key.name }}</option></select>
          </label>
        </div>
        <p v-if="keysError" role="alert" class="mb-4 text-sm text-red-600">{{ keysError }}</p>
        <p v-else-if="!keysLoading && !keys.length" class="mb-4 text-sm text-gray-500">此分组暂无可用密钥。<router-link to="/keys" class="text-primary-600 hover:underline">创建调用密钥</router-link></p>
        <div class="mb-5 flex gap-6 border-b border-gray-200 dark:border-dark-700" aria-label="模型详情">
          <button v-for="entry in tabs" :key="entry.value" class="border-b-2 pb-3 text-sm font-medium disabled:opacity-50" :class="tab === entry.value ? 'border-primary-500 text-primary-600 dark:text-primary-400' : 'border-transparent text-gray-500'" :aria-pressed="tab === entry.value" :disabled="busy" @click="tab = entry.value">{{ entry.label }}</button>
        </div>
        <section v-show="tab === 'api'" class="space-y-5">
          <dl class="grid gap-4 sm:grid-cols-2">
            <div><dt class="text-xs text-gray-500">Base URL（服务地址）</dt><dd class="mt-1 break-all font-mono text-sm text-gray-800 dark:text-gray-200">{{ baseUrl }}</dd></div>
            <div><dt class="text-xs text-gray-500">调用模型名称</dt><dd class="mt-1 break-all font-mono text-sm text-gray-800 dark:text-gray-200">{{ model.name }}</dd></div>
            <div class="sm:col-span-2"><dt class="text-xs text-gray-500">接口 · {{ protocolLabel(model.platform) }}</dt><dd class="mt-1 break-all font-mono text-sm text-gray-800 dark:text-gray-200">{{ endpoint === '暂未适配' ? endpoint : `POST ${endpoint}` }}</dd></div>
          </dl>
          <template v-if="endpoint !== '暂未适配'">
            <div class="overflow-hidden rounded-xl border border-gray-200 dark:border-dark-600">
              <div class="flex items-center justify-between gap-3 border-b border-gray-200 bg-gray-50 px-4 py-3 dark:border-dark-600 dark:bg-dark-900"><p class="text-xs font-medium text-gray-600 dark:text-gray-300">{{ item.type === 'Videos' ? '创建与查询 · cURL 示例' : 'cURL 请求示例' }}</p><button class="flex items-center gap-1 text-xs text-primary-600 dark:text-primary-300" @click="copyExample"><Icon name="copy" size="sm" />{{ copied ? '已复制' : '复制示例' }}</button></div>
              <pre class="overflow-x-auto bg-gray-900 p-4 text-xs leading-6 text-gray-100">{{ example }}</pre>
            </div>
            <p v-if="item.type === 'Videos'" class="text-xs leading-5 text-gray-500">视频通过异步接口调用：创建后保存任务编号，再由你的应用查询状态与下载结果。此页面提供接入示例和当前测试结果。</p>
            <p class="text-xs leading-5 text-gray-500">示例使用 YOUR_API_KEY 占位符，请在客户端填入所选分组的密钥；真实密钥请在 API 密钥管理中查看。</p>
            <p v-if="copyError" role="alert" class="text-xs text-red-600">{{ copyError }}</p>
          </template>
          <p v-else class="rounded-xl bg-gray-50 p-4 text-sm text-gray-500 dark:bg-dark-900">此模型已列入目录，当前暂无对应的调用适配。请联系管理员确认接入方式。</p>
        </section>
        <section v-if="testVisited" v-show="tab === 'test'" class="space-y-4">
          <p class="text-sm text-gray-500">使用上方分组与密钥发送一次请求，检查 API 是否可用。</p>
          <p v-if="item.type === 'Videos'" class="text-xs text-gray-500">切换分组或密钥会重置测试显示，已受理的请求不会取消，请保存任务编号。</p>
          <p v-if="keysLoading" class="py-6 text-center text-sm text-gray-500">正在加载调用密钥…</p>
          <p v-else-if="!selectedKey" class="rounded-xl bg-gray-50 p-4 text-sm text-gray-500 dark:bg-dark-900">请选择可用的调用密钥后开始测试。</p>
          <TextModelTester v-else-if="item.type === 'Chat'" :key="contextId" :model="model" :api-key="selectedKey.key" @busy="busy = $event" />
          <ImageWorkbench v-else-if="item.type === 'Images' && ['openai', 'grok'].includes(model.platform)" :key="contextId" embedded :model="model" :selected-key-id="selectedKey.id" :context-keys="keys" @busy="busy = $event" />
          <VideoWorkbench v-else-if="item.type === 'Videos' && miniMaxVideoSpec(item.name) && model.platform === 'openai'" :key="contextId" embedded :model="model" :selected-key-id="selectedKey.id" :context-keys="keys" @busy="busy = $event" />
          <p v-else class="rounded-xl bg-gray-50 p-4 text-sm text-gray-500 dark:bg-dark-900">此模型暂不支持页面内测试，请使用 API 接入示例调用。</p>
        </section>
      </div>
    </section>
  </div>
</template>
<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import ImageWorkbench from './ImageWorkbench.vue'
import VideoWorkbench from './VideoWorkbench.vue'
import TextModelTester from './TextModelTester.vue'
import { list } from '@/api/keys'
import { buildGatewayUrl } from '@/api/url'
import { useAppStore } from '@/stores/app'
import type { ApiKey } from '@/types'
import { catalogModelEndpoint } from '@/utils/modelCatalog'
import { isSeedreamModel } from '@/utils/modelTest'
import { miniMaxVideoSpec } from '@/utils/minimaxVideo'
import { buildTextRequest, catalogTypeLabels, modelSupplier, protocolLabel, type UserCatalogItem } from './userModelCatalog'
const props = withDefaults(defineProps<{ item: UserCatalogItem; initialTab?: 'api' | 'test' }>(), { initialTab: 'api' })
const emit = defineEmits<{ close: [] }>()
const appStore = useAppStore()
const tab = ref<'api' | 'test'>(props.initialTab)
const testVisited = ref(props.initialTab === 'test')
watch(tab, value => { if (value === 'test') testVisited.value = true })
const tabs = [{ value: 'api' as const, label: 'API 接入' }, { value: 'test' as const, label: '在线测试' }]
const groupId = ref(props.item.groups[0].group_id)
const keyId = ref(0)
const keys = ref<ApiKey[]>([])
const keysLoading = ref(false)
const keysError = ref('')
const busy = ref(false)
const copied = ref(false)
const copyError = ref('')
const model = computed(() => props.item.groups.find(group => group.group_id === groupId.value) || props.item.groups[0])
const selectedKey = computed(() => keys.value.find(key => key.id === keyId.value))
const contextId = computed(() => `${groupId.value}:${keyId.value}`)
const endpoint = computed(() => catalogModelEndpoint(props.item.type, model.value.platform, model.value.name))
const origin = computed(() => {
  try { return new URL(appStore.apiBaseUrl || buildGatewayUrl('/'), window.location.origin).origin }
  catch { return window.location.origin }
})
const baseUrl = computed(() => `${origin.value}${endpoint.value.startsWith('/v1beta/') ? '/v1beta' : endpoint.value.startsWith('/v2/') ? '/v2' : '/v1'}`)
const example = computed(() => {
  const selectedModel = model.value
  if (props.item.type === 'Chat') {
    const request = buildTextRequest(selectedModel, '你好，请用一句话介绍自己。')
    return curl(request.path, request.body, request.headers)
  }
  if (props.item.type === 'Images') {
    return curl(endpoint.value, { model: selectedModel.name, prompt: '一只在窗边晒太阳的猫', size: selectedModel.policy?.image_sizes?.[0] || (isSeedreamModel(selectedModel.name) ? '2K' : '1024x1024'), ...(isSeedreamModel(selectedModel.name) ? { response_format: 'url' } : { n: 1 }) })
  }
  const mode = !selectedModel.policy?.modes?.length || selectedModel.policy.modes.includes('text') ? 'text' : selectedModel.policy.modes[0]
  const content: Record<string, unknown>[] = [{ type: 'text', text: '一只猫在阳光下缓慢走过花园' }]
  if (mode !== 'text') content.push({ type: 'image_url', role: 'first_frame', image_url: { url: 'https://example.com/first-frame.png' } })
  if (mode === 'first_last_frame') content.push({ type: 'image_url', role: 'last_frame', image_url: { url: 'https://example.com/last-frame.png' } })
  const spec = miniMaxVideoSpec(selectedModel.name)
  const payload = { model: selectedModel.name, resolution: spec?.resolutions.find(value => !selectedModel.policy?.resolutions || selectedModel.policy.resolutions.includes(value)) || '', duration: Math.max(spec?.minDuration || 0, selectedModel.policy?.min_duration || 0), ratio: mode === 'text' ? '16:9' : 'adaptive', content }
  if (spec && selectedModel.platform === 'openai') {
    return `# 1. 获取本次请求的报价版本\n${curl('/v2/video_generation/quote', payload)}\n\n# 2. 将响应的 version 填入 X-Video-Quote；同一次请求始终使用同一 Idempotency-Key\n${curl('/v2/video_generation', payload, { 'X-Video-Quote': 'QUOTE_VERSION', 'Idempotency-Key': 'YOUR_UNIQUE_REQUEST_ID' })}\n\n# 3. 保存创建响应的 task_id，查询生成状态\ncurl '${origin.value}/v2/query/video_generation/TASK_ID' \\\n  -H 'Authorization: Bearer YOUR_API_KEY'`
  }
  return curl(endpoint.value, { model: selectedModel.name, prompt: '一只猫在阳光下缓慢走过花园' }) + `\n\n# 保存创建响应的 request_id，查询生成状态\ncurl '${origin.value}/v1/videos/REQUEST_ID' \\\n  -H 'Authorization: Bearer YOUR_API_KEY'`
})
function curl(path: string, body: Record<string, unknown>, headers: Record<string, string> = {}) {
  return [`curl '${origin.value}${path}' \\`, "  -H 'Authorization: Bearer YOUR_API_KEY' \\", "  -H 'Content-Type: application/json' \\", ...Object.entries(headers).map(([name, value]) => `  -H '${name}: ${value}' \\`), `  -d '${JSON.stringify(body, null, 2).replace(/'/g, "'\\''")}'`].join('\n')
}
let keysRevision = 0
watch(groupId, async id => {
  const revision = ++keysRevision
  keys.value = []
  keyId.value = 0
  keysError.value = ''
  keysLoading.value = true
  try {
    const first = await list(1, 100, { group_id: id, status: 'active' })
    const allKeys = [...first.items]
    for (let page = 2; page <= first.pages; page++) allKeys.push(...(await list(page, 100, { group_id: id, status: 'active' })).items)
    if (revision !== keysRevision) return
    keys.value = allKeys.filter(key => key.group_id === id && key.status === 'active')
    keyId.value = keys.value[0]?.id || 0
  } catch { if (revision === keysRevision) keysError.value = '调用密钥加载失败，请关闭详情后重新打开。' }
  finally { if (revision === keysRevision) keysLoading.value = false }
}, { immediate: true })
function close() { if (!busy.value) emit('close') }
function onEscape(event: KeyboardEvent) { if (event.key === 'Escape') close() }
async function copyExample() {
  copyError.value = ''
  try { await navigator.clipboard.writeText(example.value); copied.value = true; window.setTimeout(() => { copied.value = false }, 2000) }
  catch { copyError.value = '浏览器未允许复制，请手动选择并复制示例。' }
}
onMounted(() => window.addEventListener('keydown', onEscape))
onUnmounted(() => { keysRevision++; window.removeEventListener('keydown', onEscape) })
</script>
