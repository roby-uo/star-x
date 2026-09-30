<template>
  <form class="space-y-4" @submit.prevent="test">
    <label class="block text-sm font-medium text-gray-700 dark:text-gray-200">
      测试内容
      <textarea v-model="prompt" class="input mt-2" rows="4" placeholder="输入一段内容，查看模型响应" required :disabled="busy" />
    </label>
    <label class="flex items-start gap-2 text-sm text-gray-600 dark:text-gray-300">
      <input v-model="accepted" type="checkbox" class="mt-1" :disabled="busy" />
      <span>在线测试会消耗账户额度</span>
    </label>
    <button class="btn btn-primary" :disabled="busy || !apiKey || !accepted || !prompt.trim()">{{ busy ? '正在测试…' : '发送测试' }}</button>
    <p v-if="error" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ error }}</p>
    <section v-if="answer" class="rounded-xl border border-gray-200 bg-gray-50 p-4 dark:border-dark-600 dark:bg-dark-900">
      <p class="mb-2 text-xs font-medium text-gray-500">模型响应</p>
      <p class="whitespace-pre-wrap break-words text-sm text-gray-800 dark:text-gray-200">{{ answer }}</p>
    </section>
  </form>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import type { UserAvailableModel } from '@/api/channels'
import { buildGatewayUrl } from '@/api/url'
import { buildTextRequest, parseTextResponse } from './userModelCatalog'

const props = defineProps<{ model: UserAvailableModel; apiKey: string }>()
const emit = defineEmits<{ busy: [value: boolean] }>()
const prompt = ref('你好，请用一句话介绍自己。')
const accepted = ref(false)
const busy = ref(false)
const answer = ref('')
const error = ref('')

async function test() {
  if (busy.value || !accepted.value || !props.apiKey || !prompt.value.trim()) return
  busy.value = true
  emit('busy', true)
  answer.value = ''
  error.value = ''
  try {
    const request = buildTextRequest(props.model, prompt.value.trim())
    const response = await fetch(buildGatewayUrl(request.path), { method: 'POST', headers: { Authorization: `Bearer ${props.apiKey}`, 'Content-Type': 'application/json', ...request.headers }, body: JSON.stringify(request.body) })
    const result = await response.json()
    if (!response.ok) throw new Error(result.error?.message || `测试失败（${response.status}）`)
    answer.value = parseTextResponse(result)
    if (!answer.value) error.value = '请求已完成，但响应未包含可显示的文本。请通过 API 查看完整响应。'
  } catch (cause) {
    error.value = `${cause instanceof Error ? cause.message : '测试失败'}。请求可能已提交，请核对使用记录后再重试。`
  } finally {
    busy.value = false
    accepted.value = false
    emit('busy', false)
  }
}
</script>
