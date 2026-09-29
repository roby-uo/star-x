<template>
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4" @click.self="emit('close')"><section role="dialog" aria-modal="true" aria-labelledby="key-access-title" class="max-h-[85vh] w-full max-w-xl overflow-y-auto rounded-xl bg-white p-6 dark:bg-dark-800">
    <h2 id="key-access-title" class="text-lg font-semibold">{{ apiKey.name }} · 模型权限</h2><p class="my-3 text-sm text-gray-500">密钥只能收窄所属分组的权限。余额、额度、有效期与速率限制仍在密钥编辑中管理。启用模型白名单后请使用 HTTP 接口，WebSocket 调用将被禁用。</p>
    <form class="space-y-4" @submit.prevent="save">
      <label class="flex items-center gap-2"><input v-model="rules.restrict_models" type="checkbox" />仅允许选中的模型</label>
      <div v-if="rules.restrict_models" class="max-h-56 overflow-auto rounded-lg border p-3 dark:border-dark-600"><label v-for="model in models" :key="model" class="mb-2 flex items-center gap-2"><input v-model="rules.models" type="checkbox" :value="model" />{{ model }}</label><p v-if="!models.length" class="text-sm text-gray-500">当前分组无可用模型。勾选限制后留空会拒绝所有模型调用。</p></div>
      <div><label class="input-label">视频分辨率限制（不选则沿用分组）</label><label v-for="r in ['480P','720P','768P','1080P','2K']" :key="r" class="mr-3 inline-flex items-center gap-1"><input v-model="rules.video_resolutions" type="checkbox" :value="r" />{{ r }}</label></div>
      <label class="block">视频最长时长（0 表示沿用分组）<input v-model.number="rules.video_max_duration" type="number" min="0" max="15" class="input" required /></label>
      <p v-if="error" role="alert" class="text-sm text-red-600">{{ error }}</p>
      <div class="flex justify-end gap-3"><button type="button" class="btn btn-secondary" @click="emit('close')">取消</button><button class="btn btn-primary" :disabled="loading">保存权限</button></div>
    </form>
  </section></div>
</template>
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { apiClient } from '@/api/client'
import channelsAPI from '@/api/channels'
import type { ApiKey } from '@/types'
import { extractApiErrorMessage } from '@/utils/apiError'
const props = defineProps<{ apiKey: ApiKey }>()
const emit = defineEmits<{ close: [] }>()
const rules = ref({ restrict_models: false, models: [] as string[], video_resolutions: [] as string[], video_max_duration: 0 })
const models = ref<string[]>([])
const error = ref('')
const loading = ref(true)
onMounted(async () => { try { const [access, catalog] = await Promise.all([apiClient.get(`/keys/${props.apiKey.id}/model-access`), channelsAPI.getAvailableModels()]); rules.value = { ...rules.value, ...access.data, models: access.data.models || [], video_resolutions: access.data.video_resolutions || [] }; models.value = [...new Set([...catalog.filter(m => m.group_id === props.apiKey.group_id).map(m => m.name), ...rules.value.models])] } catch (e) { error.value = extractApiErrorMessage(e, '权限加载失败') } finally { loading.value = false } })
async function save() { loading.value = true; try { await apiClient.put(`/keys/${props.apiKey.id}/model-access`, rules.value); emit('close') } catch (e) { error.value = extractApiErrorMessage(e, '保存失败') } finally { loading.value = false } }
</script>
