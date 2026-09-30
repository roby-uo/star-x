<template>
  <AppLayout>
    <div class="mx-auto max-w-7xl space-y-6">
      <header class="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 class="text-xl font-semibold text-gray-900 dark:text-white">选择模型，开始接入</h1>
          <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">查看可用模型、复制 API 示例，或用已有密钥在线测试。</p>
        </div>
        <router-link to="/keys" class="btn btn-secondary"><Icon name="key" size="sm" class="mr-2" />管理调用密钥</router-link>
      </header>
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
      </section>
      <p v-if="loading" class="py-12 text-center text-sm text-gray-500" role="status">正在加载模型…</p>
      <section v-else-if="filteredItems.length" class="grid gap-4 md:grid-cols-2 xl:grid-cols-3" aria-label="可用模型">
        <article v-for="item in filteredItems" :key="item.key" class="flex min-w-0 flex-col rounded-2xl border border-gray-200 bg-white p-5 dark:border-dark-700 dark:bg-dark-800">
          <div class="mb-3 flex items-center justify-between gap-2">
            <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ modelSupplier(item.name, item.platform) }}</span>
            <span class="rounded-full bg-gray-100 px-2.5 py-1 text-xs text-gray-600 dark:bg-dark-700 dark:text-gray-300">{{ catalogTypeLabels[item.type] }}</span>
          </div>
          <h2 class="break-words text-base font-semibold text-gray-900 dark:text-white">{{ item.name }}</h2>
          <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">{{ capabilityDescriptions[item.type] }}</p>
          <p class="mt-4 text-xs text-gray-500 dark:text-gray-400">{{ item.groups.map(group => group.group_name).join(' · ') }}</p>
          <div class="mt-5 flex flex-wrap gap-2 border-t border-gray-100 pt-4 dark:border-dark-700">
            <button class="btn btn-primary flex-1 text-sm" @click="open(item, 'api')">API 接入</button>
            <button class="btn btn-secondary flex-1 text-sm" @click="open(item, 'test')">在线测试</button>
          </div>
        </article>
      </section>
      <section v-else class="rounded-2xl border border-dashed border-gray-200 bg-white px-5 py-14 text-center dark:border-dark-700 dark:bg-dark-800">
        <p class="font-medium text-gray-700 dark:text-gray-200">{{ query || type ? '没有符合筛选条件的模型' : '暂无可用模型' }}</p>
        <p class="mt-2 text-sm text-gray-500">{{ query || type ? '调整搜索词或模型类型后再试。' : '模型会根据你可使用的分组自动显示。' }}</p>
        <button v-if="query || type" class="btn btn-secondary mt-4" @click="query = ''; type = ''">清除筛选</button>
      </section>
    </div>
    <ModelDetails v-if="selected" :item="selected" :initial-tab="initialTab" @close="selected = null" />
  </AppLayout>
</template>
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import ModelDetails from '@/components/models/ModelDetails.vue'
import Icon from '@/components/icons/Icon.vue'
import userChannelsAPI, { type UserAvailableModel } from '@/api/channels'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { CatalogModelType } from '@/utils/modelCatalog'
import { aggregateUserModels, catalogTypeLabels, modelSupplier, type UserCatalogItem } from '@/components/models/userModelCatalog'
const models = ref<UserAvailableModel[]>([])
const loading = ref(false)
const query = ref('')
const type = ref<CatalogModelType | ''>('')
const selected = ref<UserCatalogItem | null>(null)
const initialTab = ref<'api' | 'test'>('api')
const appStore = useAppStore()
const categories: { value: CatalogModelType | ''; label: string }[] = [{ value: '', label: '全部' }, { value: 'Chat', label: '文本' }, { value: 'Images', label: '图像' }, { value: 'Videos', label: '视频' }]
const capabilityDescriptions = { Chat: '文本生成与对话 · 同步调用', Images: '图像生成 · 通过 API 调用', Videos: '视频生成 · 异步 API 调用' }
const items = computed(() => aggregateUserModels(models.value))
const filteredItems = computed(() => {
  const search = query.value.trim().toLowerCase()
  return items.value.filter(item => (!type.value || item.type === type.value) && (!search || [item.name, item.platform, modelSupplier(item.name, item.platform), ...item.groups.map(group => group.group_name)].some(value => value.toLowerCase().includes(search))))
})
function open(item: UserCatalogItem, tab: 'api' | 'test') { selected.value = item; initialTab.value = tab }
async function load() {
  loading.value = true
  try { models.value = await userChannelsAPI.getAvailableModels() }
  catch (error) { appStore.showError(extractApiErrorMessage(error, '模型目录加载失败')) }
  finally { loading.value = false }
}
onMounted(load)
</script>
