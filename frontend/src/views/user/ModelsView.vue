<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="mb-4 flex gap-2"><button class="btn" :class="tab === 'models' ? 'btn-primary' : 'btn-secondary'" @click="tab = 'models'">可用模型</button><button class="btn" :class="tab === 'tasks' ? 'btn-primary' : 'btn-secondary'" @click="tab = 'tasks'">我的视频任务</button></div>
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div class="flex flex-1 flex-wrap items-center gap-3">
            <div class="relative w-full sm:w-80">
              <Icon name="search" size="md" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
              <input v-model="query" class="input pl-10" placeholder="搜索可用模型" />
            </div>
            <select v-model="type" class="input w-40">
              <option value="">全部类型</option>
              <option value="Chat">文本</option>
              <option value="Images">图片</option>
              <option value="Videos">视频</option>
            </select>
          </div>
          <button class="btn btn-secondary" :disabled="loading" title="刷新" @click="load"><Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" /></button>
        </div>
      </template>
      <template #table>
        <MediaTaskList v-if="tab === 'tasks'" />
        <div v-else class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-800"><tr><th v-for="heading in headings" :key="heading" class="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-500">{{ heading }}</th></tr></thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="row in filteredRows" :key="row.key" class="hover:bg-gray-50 dark:hover:bg-dark-800/60">
                <td class="px-4 py-3 font-medium text-gray-900 dark:text-white">{{ row.model }}</td>
                <td class="px-4 py-3"><span class="rounded-full bg-primary-50 px-2 py-1 text-xs text-primary-700 dark:bg-primary-900/20 dark:text-primary-300">{{ row.type }}</span></td>
                <td class="px-4 py-3 text-sm text-gray-600 dark:text-gray-300">{{ row.platform }}</td>
                <td class="px-4 py-3 text-sm text-gray-600 dark:text-gray-300">{{ row.group }}</td>
                <td class="px-4 py-3 font-mono text-xs text-gray-500">{{ row.endpoint }}</td>
                <td class="px-4 py-3 text-right">
                  <button class="btn btn-secondary mr-2 px-2 py-1 text-xs" @click="details = models.find(m => `${m.group_id}-${m.platform}-${m.name}` === row.key) || null">规格与价格</button>
                  <button v-if="row.type === 'Images' && ['openai','grok'].includes(row.platform)" class="btn btn-primary px-2 py-1 text-xs" @click="selectedImage = models.find(m => `${m.group_id}-${m.platform}-${m.name}` === row.key) || null">生成图片</button>
                  <router-link v-else-if="row.testEndpoint" :to="{ path: '/model-test', query: { model: row.model, endpoint: row.testEndpoint } }" class="btn btn-secondary px-2 py-1 text-xs">{{ t('modelTest.test') }}</router-link>
                  <button v-else-if="row.model === 'MiniMax-H3'" class="btn btn-primary px-2 py-1 text-xs" @click="selected = models.find(m => `${m.group_id}-${m.platform}-${m.name}` === row.key) || null">选规格／生成</button>
                  <span v-else class="text-xs text-gray-400">{{ row.endpoint }}</span>
                </td>
              </tr>
              <tr v-if="!loading && filteredRows.length === 0"><td colspan="6" class="px-4 py-12 text-center text-sm text-gray-500">暂无可用模型</td></tr>
              <tr v-if="loading"><td colspan="6" class="px-4 py-12 text-center text-sm text-gray-500">加载中...</td></tr>
            </tbody>
          </table>
        </div>
      </template>
    </TablePageLayout>
    <ModelDetails v-if="details" :model="details" @close="details = null" />
    <ImageWorkbench v-if="selectedImage" :model="selectedImage" @close="selectedImage = null" />
    <VideoWorkbench v-if="selected" :model="selected" @close="selected = null" @submitted="tab = 'tasks'" />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import ModelDetails from '@/components/models/ModelDetails.vue'
import ImageWorkbench from '@/components/models/ImageWorkbench.vue'
import VideoWorkbench from '@/components/models/VideoWorkbench.vue'
import MediaTaskList from '@/components/models/MediaTaskList.vue'
import Icon from '@/components/icons/Icon.vue'
import userChannelsAPI, { type UserAvailableModel } from '@/api/channels'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { catalogModelEndpoint, catalogModelType, isAudioModel } from '@/utils/modelCatalog'
import type { ModelTestEndpoint } from '@/utils/modelTest'

type ModelRow = { key: string; model: string; type: string; platform: string; group: string; endpoint: string; testEndpoint: ModelTestEndpoint | null }
const { t } = useI18n()
const tab = ref('models')
const details = ref<UserAvailableModel | null>(null)
const selectedImage = ref<UserAvailableModel | null>(null)
const selected = ref<UserAvailableModel | null>(null)
const models = ref<UserAvailableModel[]>([])
const loading = ref(false)
const query = ref('')
const type = ref('')
const headings = ['模型', '类型', '平台', '可用分组', '端点', '操作']
const appStore = useAppStore()

const rows = computed<ModelRow[]>(() => models.value.filter((model) => !isAudioModel(model.name)).map((model) => {
  const modelType = catalogModelType(model.name, model.policy?.kind === 'image' ? 'image' : model.pricing?.billing_mode)
  return { key: `${model.group_id}-${model.platform}-${model.name}`, model: model.name, type: modelType, platform: model.platform, group: model.group_name, endpoint: catalogModelEndpoint(modelType, model.platform, model.name), testEndpoint: modelType === 'Videos' ? null : modelType as ModelTestEndpoint }
}))
const filteredRows = computed(() => { const q = query.value.trim().toLowerCase(); return rows.value.filter((row) => (!type.value || row.type === type.value) && (!q || [row.model, row.platform, row.group, row.endpoint].some((v) => v.toLowerCase().includes(q)))) })

async function load() { loading.value = true; try { models.value = await userChannelsAPI.getAvailableModels() } catch (error) { appStore.showError(extractApiErrorMessage(error, '模型目录加载失败')) } finally { loading.value = false } }
onMounted(load)
</script>
