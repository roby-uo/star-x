<template>
  <AppLayout>
    <div class="mx-auto max-w-7xl space-y-6">
      <UserAccessWorkspace />
      <header>
        <h1 class="text-lg font-semibold text-gray-900 dark:text-white">选择模型</h1>
        <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">选择模型后获取接入配置，或发送一次在线测试。</p>
      </header>
      <div v-if="loadingError" role="alert" class="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-900/10 dark:text-red-300">
        <p>{{ loadingError }}{{ catalogLoaded ? '，当前保留上次加载的模型。' : '，请重试。' }}</p>
        <button type="button" class="btn btn-secondary text-xs" :disabled="loading" @click="load">重新加载</button>
      </div>
      <p v-if="contextNotice" role="status" class="rounded-xl bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:bg-amber-900/20 dark:text-amber-200">{{ contextNotice }}</p>
      <section class="space-y-4 rounded-2xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800 sm:p-5" aria-label="模型筛选">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div class="relative min-w-0 flex-1 sm:max-w-md">
            <Icon name="search" size="md" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
            <input v-model="query" type="search" class="input pl-10" placeholder="搜索模型名称、服务商" aria-label="搜索模型" />
          </div>
          <button class="btn btn-secondary" :disabled="loading" title="刷新模型" aria-label="刷新模型" @click="load"><Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" /></button>
        </div>
        <div class="flex flex-wrap gap-2" aria-label="模型类型">
          <button v-for="category in categories" :key="category.value" type="button" class="rounded-lg px-3 py-2 text-sm transition-colors" :class="type === category.value ? 'bg-primary-50 font-medium text-primary-700 dark:bg-primary-900/30 dark:text-primary-300' : 'text-gray-600 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-dark-700'" :aria-pressed="type === category.value" @click="type = category.value">
            {{ category.label }}<span class="ml-2 text-xs opacity-70">{{ category.value ? items.filter(item => item.type === category.value).length : items.length }}</span>
          </button>
        </div>
        <p class="text-xs leading-5 text-gray-500 dark:text-gray-400">{{ categoryDescription }}</p>
      </section>
      <p v-if="loading" class="py-12 text-center text-sm text-gray-500" role="status">正在加载模型…</p>
      <section v-else-if="filteredItems.length" class="grid gap-4 md:grid-cols-2 xl:grid-cols-3" aria-label="可用模型">
        <article v-for="item in filteredItems" :key="item.key" class="flex min-w-0 flex-col rounded-2xl border border-gray-200 bg-white p-5 dark:border-dark-700 dark:bg-dark-800">
          <div class="mb-3 flex items-center justify-between gap-2">
            <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ modelSupplier(item.name, item.platform) }}</span>
            <span class="rounded-full bg-gray-100 px-2.5 py-1 text-xs text-gray-600 dark:bg-dark-700 dark:text-gray-300">{{ catalogTypeLabels[item.type] }}</span>
          </div>
          <h2 class="break-words text-base font-semibold text-gray-900 dark:text-white">{{ item.name }}</h2>
          <p class="mt-2 text-xs font-medium text-primary-600 dark:text-primary-300">{{ modelAccessLabels[item.type] }}</p>
          <p class="mb-5 mt-4 text-xs text-gray-500 dark:text-gray-400">{{ item.groups.map(group => group.group_name).join(' · ') }}</p>
          <div class="mt-auto flex flex-wrap gap-2 border-t border-gray-100 pt-4 dark:border-dark-700">
            <button class="btn btn-primary flex-1 text-sm" @click="open(item, 'api')">API 接入</button>
            <button class="btn btn-secondary flex-1 text-sm" @click="open(item, 'test')">在线测试</button>
          </div>
        </article>
      </section>
      <section v-else-if="!loadingError || catalogLoaded" class="rounded-2xl border border-dashed border-gray-200 bg-white px-5 py-14 text-center dark:border-dark-700 dark:bg-dark-800">
        <p class="font-medium text-gray-700 dark:text-gray-200">{{ query || type ? '没有符合筛选条件的模型' : '暂无可用模型' }}</p>
        <p class="mt-2 text-sm text-gray-500">{{ query || type ? '调整搜索词或模型类型后再试。' : '模型会根据你可使用的分组自动显示。' }}</p>
        <button v-if="query || type" class="btn btn-secondary mt-4" @click="query = ''; type = ''">清除筛选</button>
      </section>
    </div>
    <ModelDetails v-if="selected" :key="selected.key" :item="selected" :initial-tab="initialTab" :initial-group-id="detailGroupId" :initial-key-id="detailKeyId" @context-change="rememberContext" @close="closeDetails" />
  </AppLayout>
</template>
<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import ModelDetails from '@/components/models/ModelDetails.vue'
import UserAccessWorkspace from '@/components/models/UserAccessWorkspace.vue'
import Icon from '@/components/icons/Icon.vue'
import userChannelsAPI, { type UserAvailableModel } from '@/api/channels'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { CatalogModelType } from '@/utils/modelCatalog'
import { aggregateUserModels, catalogTypeLabels, modelAccessLabels, modelSupplier, type UserCatalogItem } from '@/components/models/userModelCatalog'
import { buildUserAccessQuery, parseUserAccessContext } from '@/utils/userAccessContext'
const models = ref<UserAvailableModel[]>([])
const loading = ref(false)
const loadingError = ref('')
const catalogLoaded = ref(false)
const contextNotice = ref('')
const query = ref('')
const type = ref<CatalogModelType | ''>('')
const selected = ref<UserCatalogItem | null>(null)
const initialTab = ref<'api' | 'test'>('api')
const detailGroupId = ref<number>()
const detailKeyId = ref<number>()
const appStore = useAppStore()
const route = useRoute()
const router = useRouter()
const categories: { value: CatalogModelType | ''; label: string }[] = [{ value: '', label: '全部' }, { value: 'Chat', label: '文本' }, { value: 'Images', label: '图像' }, { value: 'Videos', label: '视频' }]
const capabilityDescriptions = {
  Chat: '用于对话或 Agent 推理。接入客户端前，请确认接口协议和工具调用能力。',
  Images: '通过图片接口或独立工具生成图片，不填写为 Agent 主模型。',
  Videos: '通过异步视频接口或独立工具生成视频，不填写为 Agent 主模型。'
}
const categoryDescription = computed(() => type.value ? capabilityDescriptions[type.value] : '文本用于对话和 Agent 推理；图像、视频通过对应接口或独立工具调用。')
const items = computed(() => aggregateUserModels(models.value))
const filteredItems = computed(() => {
  const search = query.value.trim().toLowerCase()
  return items.value.filter(item => (!type.value || item.type === type.value) && (!search || [item.name, item.platform, modelSupplier(item.name, item.platform), ...item.groups.map(group => group.group_name)].some(value => value.toLowerCase().includes(search))))
})
function open(item: UserCatalogItem, tab: 'api' | 'test') {
  contextNotice.value = ''
  initialTab.value = tab
  detailGroupId.value = item.groups[0]?.group_id
  detailKeyId.value = undefined
  selected.value = item
  void router.push({ path: '/model-catalog', query: buildUserAccessQuery({ model: item.name, platform: item.platform, kind: item.type, groupId: detailGroupId.value, tab }) })
}
function closeDetails() {
  selected.value = null
  const query = { ...route.query }
  for (const key of ['model', 'platform', 'kind', 'group', 'tab', 'key', 'create']) delete query[key]
  void router.replace({ path: '/model-catalog', query })
}
function rememberContext(context: { groupId: number; keyId?: number; tab: 'api' | 'test' }) {
  if (!selected.value || route.path !== '/model-catalog') return
  detailGroupId.value = context.groupId
  detailKeyId.value = context.keyId
  initialTab.value = context.tab
  const query = buildUserAccessQuery({ model: selected.value.name, platform: selected.value.platform, kind: selected.value.type, ...context })
  if (Object.keys(route.query).length === Object.keys(query).length && Object.entries(query).every(([key, value]) => route.query[key] === value)) return
  void router.replace({ path: '/model-catalog', query })
}
function restoreContext() {
  if (!catalogLoaded.value || loading.value) return
  const context = parseUserAccessContext(route.query)
  if (!context) { selected.value = null; contextNotice.value = ''; return }
  const item = items.value.find(item => item.name === context.model && item.platform === context.platform && item.type === context.kind)
  if (!item) {
    selected.value = null
    contextNotice.value = '这个模型当前不可用，请重新选择。'
    return
  }
  const groupAvailable = !context.groupId || item.groups.some(group => group.group_id === context.groupId)
  if (!groupAvailable) {
    selected.value = null
    contextNotice.value = '原调用分组已无法使用这个模型，请重新选择调用分组。'
    return
  }
  contextNotice.value = ''
  initialTab.value = context.tab
  detailGroupId.value = context.groupId
  detailKeyId.value = context.keyId
  selected.value = item
}
async function load() {
  loading.value = true
  loadingError.value = ''
  try { models.value = await userChannelsAPI.getAvailableModels(); catalogLoaded.value = true }
  catch (error) { loadingError.value = extractApiErrorMessage(error, '模型加载失败'); appStore.showError(loadingError.value) }
  finally { loading.value = false; restoreContext() }
}
watch(() => route.fullPath, restoreContext)
onMounted(load)
</script>
