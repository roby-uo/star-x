<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <AdminWorkspaceTabs area="models" />
        <div class="mb-4 rounded-lg border border-blue-200 bg-blue-50 px-4 py-3 text-sm text-blue-900 dark:border-blue-900 dark:bg-blue-950/30 dark:text-blue-200">
          此处汇总上游账号中明确配置的模型，并标出已配置定价的对外服务。账号设置为“支持全部模型”时，不会凭空推断上游实际支持哪些模型；请在上游连接中配置模型，或用“同步上游支持的模型”获取实时列表。
        </div>
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div class="flex flex-1 flex-wrap items-center gap-3">
            <div class="relative w-full sm:w-80">
              <Icon name="search" size="md" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
              <input v-model="query" class="input pl-10" placeholder="搜索模型、服务商或渠道" />
            </div>
            <select v-model="type" class="input w-40">
              <option value="">全部类型</option>
              <option value="chat">Chat</option>
              <option value="image">Images</option>
              <option value="video">Videos</option>
              <option value="audio">Audio</option>
            </select>
          </div>
          <div class="flex items-center gap-2">
            <router-link to="/admin/channels/pricing" class="btn btn-primary">配置对外服务与定价</router-link>
            <button class="btn btn-secondary" :disabled="loading" title="刷新" @click="load">
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
          </div>
        </div>
      </template>

      <template #table>
        <div class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-800">
              <tr>
                <th v-for="heading in headings" :key="heading" class="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-500">{{ heading }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="row in filteredRows" :key="row.key" class="hover:bg-gray-50 dark:hover:bg-dark-800/60">
                <td class="px-4 py-3 font-medium text-gray-900 dark:text-white">{{ row.model }}</td>
                <td class="px-4 py-3"><span class="rounded-full bg-primary-50 px-2 py-1 text-xs text-primary-700 dark:bg-primary-900/20 dark:text-primary-300">{{ row.type }}</span></td>
                <td class="px-4 py-3 text-sm text-gray-600 dark:text-gray-300">{{ row.platform }}</td>
                <td class="px-4 py-3 text-sm text-gray-600 dark:text-gray-300">{{ row.accounts || '—' }}</td>
                <td class="px-4 py-3 font-mono text-xs text-gray-500">{{ row.targets || '—' }}</td>
                <td class="px-4 py-3 text-sm text-gray-600 dark:text-gray-300">{{ row.channels || '未配置对外服务' }}</td>
              </tr>
              <tr v-if="!loading && filteredRows.length === 0"><td colspan="6" class="px-4 py-12 text-center text-sm text-gray-500">
                <span v-if="rows.length">没有符合筛选条件的模型。</span>
                <span v-else>尚无明确配置的模型。请先到 <router-link class="text-primary-600 underline" to="/admin/accounts">上游连接</router-link> 配置模型白名单；定价仅在需要对外提供服务时配置。</span>
              </td></tr>
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
import AdminWorkspaceTabs from '@/components/admin/AdminWorkspaceTabs.vue'
import Icon from '@/components/icons/Icon.vue'
import channelsAPI, { type Channel } from '@/api/admin/channels'
import { accountsAPI } from '@/api/admin/accounts'
import type { Account } from '@/types'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { isSeedreamModel } from '@/utils/modelTest'

type ModelRow = { key: string; model: string; type: string; platform: string; accounts: string; channels: string; targets: string }
const appStore = useAppStore()
const channels = ref<Channel[]>([])
const accounts = ref<Account[]>([])
const loading = ref(false)
const query = ref('')
const type = ref('')
const headings = ['对外模型', '类型', '协议/平台', '上游账号', '上游模型', '对外服务']

const rows = computed<ModelRow[]>(() => {
  const inventory = new Map<string, { model: string; platform: string; accounts: Set<string>; channels: Set<string>; targets: Set<string>; image: boolean }>()
  const add = (platform: string, model: string, account?: string, channel?: string, image = false, target?: string) => {
    const key = `${platform}:${model}`
    const row = inventory.get(key) || { model, platform, accounts: new Set<string>(), channels: new Set<string>(), targets: new Set<string>(), image }
    if (account) row.accounts.add(account)
    if (channel) row.channels.add(channel)
    if (target) row.targets.add(target)
    row.image ||= image
    inventory.set(key, row)
  }
  for (const account of accounts.value) {
    for (const [model, target] of Object.entries(account.configured_models || {})) add(account.platform, model, account.name, undefined, false, target)
  }
  for (const channel of channels.value) {
    for (const pricing of channel.model_pricing || []) {
      for (const model of pricing.models) add(pricing.platform, model, undefined, channel.name, pricing.billing_mode === 'image')
    }
  }
  return [...inventory.entries()].map(([key, row]) => {
    const image = row.image || isSeedreamModel(row.model) || /^gpt-image/i.test(row.model)
    const video = /video|seedance|kling|sora/i.test(row.model)
    const audio = /audio|tts|whisper/i.test(row.model)
    return {
      key,
      model: row.model,
      type: image ? 'Images' : video ? 'Videos' : audio ? 'Audio' : 'Chat',
      platform: row.platform,
      accounts: [...row.accounts].join('、'),
      channels: [...row.channels].join('、'),
      targets: [...row.targets].join('、')
    }
  }).sort((a, b) => a.platform.localeCompare(b.platform) || a.model.localeCompare(b.model))
})

const filteredRows = computed(() => {
  const q = query.value.trim().toLowerCase()
  return rows.value.filter((row) => (!type.value || row.type.toLowerCase() === type.value) && (!q || [row.model, row.platform, row.accounts, row.channels, row.targets].some((v) => v.toLowerCase().includes(q))))
})

async function load() {
  loading.value = true
  try {
    const fetchChannels = async () => {
      const all: Channel[] = []
      for (let page = 1; ; page++) {
        const result = await channelsAPI.list(page, 100)
        all.push(...result.items)
        if (!result.items.length || all.length >= result.total) return all
      }
    }
    const fetchAccounts = async () => {
      const all: Account[] = []
      for (let page = 1; ; page++) {
        const result = await accountsAPI.list(page, 100)
        all.push(...result.items)
        if (!result.items.length || all.length >= result.total) return all
      }
    }
    ;[channels.value, accounts.value] = await Promise.all([fetchChannels(), fetchAccounts()])
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, '模型目录加载失败'))
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>
