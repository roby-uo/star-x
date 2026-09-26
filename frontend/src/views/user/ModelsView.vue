<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div class="flex flex-1 flex-wrap items-center gap-3">
            <div class="relative w-full sm:w-80">
              <Icon name="search" size="md" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
              <input v-model="query" class="input pl-10" placeholder="搜索可用模型" />
            </div>
            <select v-model="type" class="input w-40">
              <option value="">全部类型</option>
              <option value="Chat">Chat</option>
              <option value="Images">Images</option>
              <option value="Videos">Videos</option>
              <option value="Audio">Audio</option>
            </select>
          </div>
          <button class="btn btn-secondary" :disabled="loading" title="刷新" @click="load"><Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" /></button>
        </div>
      </template>
      <template #table>
        <div class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-800"><tr><th v-for="heading in headings" :key="heading" class="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-500">{{ heading }}</th></tr></thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="row in filteredRows" :key="row.key" class="hover:bg-gray-50 dark:hover:bg-dark-800/60">
                <td class="px-4 py-3 font-medium text-gray-900 dark:text-white">{{ row.model }}</td>
                <td class="px-4 py-3"><span class="rounded-full bg-primary-50 px-2 py-1 text-xs text-primary-700 dark:bg-primary-900/20 dark:text-primary-300">{{ row.type }}</span></td>
                <td class="px-4 py-3 text-sm text-gray-600 dark:text-gray-300">{{ row.platform }}</td>
                <td class="px-4 py-3 text-sm text-gray-600 dark:text-gray-300">{{ row.channel }}</td>
                <td class="px-4 py-3 font-mono text-xs text-gray-500">{{ row.endpoint }}</td>
                <td class="px-4 py-3 text-right"><router-link to="/model-test" class="btn btn-secondary px-2 py-1 text-xs">测试</router-link></td>
              </tr>
              <tr v-if="!loading && filteredRows.length === 0"><td colspan="6" class="px-4 py-12 text-center text-sm text-gray-500">暂无可用模型</td></tr>
              <tr v-if="loading"><td colspan="6" class="px-4 py-12 text-center text-sm text-gray-500">加载中...</td></tr>
            </tbody>
          </table>
        </div>
      </template>
    </TablePageLayout>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import userChannelsAPI, { type UserAvailableChannel } from '@/api/channels'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

type ModelRow = { key: string; model: string; type: string; platform: string; channel: string; endpoint: string }
const channels = ref<UserAvailableChannel[]>([])
const loading = ref(false)
const query = ref('')
const type = ref('')
const headings = ['模型', '类型', '平台', '渠道', '端点', '操作']
const appStore = useAppStore()

const rows = computed<ModelRow[]>(() => channels.value.flatMap((channel) => channel.platforms.flatMap((platform) => platform.supported_models.map((model) => {
  const image = model.pricing?.billing_mode === 'image'
  const video = /video|seedance|kling|sora/i.test(model.name)
  const audio = /audio|tts|whisper/i.test(model.name)
  const modelType = image ? 'Images' : video ? 'Videos' : audio ? 'Audio' : 'Chat'
  return { key: `${channel.name}-${platform.platform}-${model.name}`, model: model.name, type: modelType, platform: platform.platform, channel: channel.name, endpoint: image ? '/v1/images/generations' : video ? '/v1/videos/generations' : '/v1/chat/completions' }
}))))
const filteredRows = computed(() => { const q = query.value.trim().toLowerCase(); return rows.value.filter((row) => (!type.value || row.type === type.value) && (!q || [row.model, row.platform, row.channel, row.endpoint].some((v) => v.toLowerCase().includes(q)))) })

async function load() { loading.value = true; try { channels.value = await userChannelsAPI.getAvailable() } catch (error) { appStore.showError(extractApiErrorMessage(error, '模型目录加载失败')) } finally { loading.value = false } }
onMounted(load)
</script>
