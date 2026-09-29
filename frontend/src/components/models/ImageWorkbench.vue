<template>
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4" @click.self="!busy && emit('close')"><section role="dialog" aria-modal="true" aria-labelledby="image-title" class="max-h-[90vh] w-full max-w-2xl overflow-y-auto rounded-xl bg-white p-6 dark:bg-dark-800">
    <div class="flex justify-between"><h2 id="image-title" class="font-semibold">{{ model.name }} · 图片生成</h2><button class="btn btn-secondary" :disabled="busy" @click="emit('close')">关闭</button></div>
    <form class="mt-4 space-y-4" @submit.prevent="generate">
      <label class="block">调用密钥 · {{ model.group_name }}<select v-model="keyID" class="input" required :disabled="busy"><option value="">请选择</option><option v-for="key in keys" :key="key.id" :value="key.id">{{ key.name }}</option></select></label>
      <label class="block">画面描述<textarea v-model="prompt" class="input" rows="4" required :disabled="busy" /></label>
      <div class="grid grid-cols-2 gap-3"><label>图片尺寸<select v-model="size" class="input" :disabled="busy"><option v-for="value in sizes" :key="value">{{ value }}</option></select></label><label>数量<input v-model.number="count" type="number" min="1" :max="maximum" class="input" :disabled="busy || seedream" required /></label></div>
      <label v-if="seedream" class="block">参考图 URL（可选）<input v-model="reference" type="url" class="input" :disabled="busy" /></label>
      <p class="text-sm text-gray-500">图片使用现有按次或用量定价，最终扣费以实际输出和使用记录为准。支持的尺寸由上游模型决定。</p>
      <label class="flex gap-2 text-sm"><input v-model="accepted" type="checkbox" required :disabled="busy" />我确认此次生成会产生费用</label>
      <p v-if="error" role="alert" class="text-sm text-red-600">{{ error }}</p>
      <button class="btn btn-primary" :disabled="busy || !keyID || !accepted">{{ busy ? '正在生成，请勿重复提交…' : '生成图片' }}</button>
    </form>
    <div v-if="images.length" class="mt-5 grid grid-cols-2 gap-3"><a v-for="(url, index) in images" :key="index" :href="url" target="_blank" rel="noopener noreferrer"><img :src="url" alt="生成的图片" class="rounded-lg" /><span class="text-xs text-primary-600">打开并保存图片</span></a></div>
  </section></div>
</template>
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import type { UserAvailableModel } from '@/api/channels'
import type { ApiKey } from '@/types'
import { list } from '@/api/keys'
import { buildGatewayUrl } from '@/api/url'
import { isSeedreamModel } from '@/utils/modelTest'
const props = defineProps<{ model: UserAvailableModel }>()
const emit = defineEmits<{ close: [] }>()
const seedream = isSeedreamModel(props.model.name)
const sizes = props.model.policy?.image_sizes?.length ? props.model.policy.image_sizes : seedream ? ['2K', '4K'] : ['1024x1024', '1536x1024', '1024x1536']
const maximum = seedream ? 1 : Math.min(props.model.policy?.max_images || 1, 10)
const size = ref(sizes[0])
const count = ref(1)
const prompt = ref('')
const reference = ref('')
const keys = ref<ApiKey[]>([])
const keyID = ref<number | ''>('')
const accepted = ref(false)
const busy = ref(false)
const error = ref('')
const images = ref<string[]>([])
onMounted(async () => { try { keys.value = (await list(1, 100, { group_id: props.model.group_id, status: 'active' })).items; if (keys.value[0]) keyID.value = keys.value[0].id } catch { error.value = '密钥加载失败' } })
async function generate() {
  if (busy.value || !accepted.value) return
  const key = keys.value.find(k => k.id === keyID.value)
  if (!key) return
  busy.value = true; error.value = ''; images.value = []
  try {
    const response = await fetch(buildGatewayUrl('/v1/images/generations'), { method: 'POST', headers: { Authorization: `Bearer ${key.key}`, 'Content-Type': 'application/json' }, body: JSON.stringify({ model: props.model.name, prompt: prompt.value, size: size.value, ...(seedream ? { response_format: 'url', ...(reference.value ? { image: reference.value } : {}) } : { n: count.value }) }) })
    const result = await response.json()
    if (!response.ok) throw new Error(result.error?.message || '生成失败')
    images.value = (result.data || []).map((item: { url?: string; b64_json?: string }) => item.b64_json ? `data:image/png;base64,${item.b64_json}` : item.url).filter((url: unknown): url is string => typeof url === 'string' && (url.startsWith('https://') || /^data:image\/(png|jpeg|webp);base64,/.test(url)))
    if (!images.value.length) error.value = '未返回可显示的图片，请查看使用记录或 API 响应'
  } catch (e) { error.value = `${e instanceof Error ? e.message : '生成失败'}。若请求已提交，请先核对使用记录，避免重复付费。` } finally { busy.value = false; accepted.value = false }
}
</script>
