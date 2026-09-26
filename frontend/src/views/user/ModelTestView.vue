<template>
  <AppLayout>
    <main class="mx-auto w-full max-w-5xl space-y-5 px-4 py-6 sm:px-6">
      <div>
        <h1 class="text-xl font-semibold text-gray-900 dark:text-white">{{ t('modelTest.title') }}</h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('modelTest.description') }}</p>
      </div>

      <div class="flex flex-col gap-3 border-y border-gray-200 py-4 dark:border-dark-600 sm:flex-row sm:items-end">
        <label class="min-w-0 flex-1 text-sm font-medium text-gray-700 dark:text-gray-300">
          {{ t('modelTest.apiKey') }}
          <input v-model="apiKey" type="password" autocomplete="off" spellcheck="false" class="input mt-1 w-full font-mono" :placeholder="t('modelTest.apiKeyPlaceholder')" :disabled="loadingModels || Boolean(testingModel)" @input="clearModels" />
        </label>
        <button type="button" class="btn btn-primary h-10 shrink-0" :disabled="!apiKey.trim() || loadingModels || Boolean(testingModel)" @click="loadModels">
          <Icon name="refresh" size="sm" class="mr-2" :class="{ 'animate-spin': loadingModels }" />
          {{ loadingModels ? t('modelTest.loading') : t('modelTest.loadModels') }}
        </button>
      </div>

      <div v-if="loadError" role="alert" class="border-l-2 border-red-500 bg-red-50 px-3 py-2 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300">{{ loadError }}</div>

      <template v-if="models.length">
        <div class="flex flex-wrap items-center gap-3">
          <div class="inline-flex h-9 overflow-hidden rounded border border-gray-200 dark:border-dark-600" role="group" :aria-label="t('modelTest.endpoint')">
            <button v-for="item in endpoints" :key="item" type="button" class="px-3 text-sm disabled:cursor-not-allowed" :class="endpoint === item ? 'bg-primary-600 text-white' : 'bg-white text-gray-700 hover:bg-gray-50 dark:bg-dark-700 dark:text-gray-200 dark:hover:bg-dark-600'" :disabled="Boolean(testingModel)" @click="endpoint = item">
              {{ item }}
            </button>
          </div>
          <input v-model="search" type="search" class="input h-9 min-w-0 flex-1 sm:max-w-xs" :placeholder="t('modelTest.search')" />
          <span class="text-xs text-gray-500 dark:text-gray-400">{{ t('modelTest.count', { count: visibleModels.length }) }}</span>
        </div>

        <label v-if="endpoint === 'Images'" class="flex items-start gap-2 text-sm text-amber-800 dark:text-amber-300">
          <input v-model="imageChargeAccepted" type="checkbox" class="mt-1" />
          <span>{{ t('modelTest.imageCharge') }}</span>
        </label>
        <p v-else class="text-xs text-gray-500 dark:text-gray-400">{{ t('modelTest.textCharge') }}</p>

        <div class="divide-y divide-gray-200 border-y border-gray-200 dark:divide-dark-600 dark:border-dark-600">
          <div v-for="model in visibleModels" :key="model" class="flex min-h-14 items-center gap-3 py-2">
            <span class="min-w-0 flex-1 break-all font-mono text-sm text-gray-900 dark:text-white">{{ model }}</span>
            <span v-if="results[resultKey(model)]" class="max-w-[40%] truncate text-xs" :class="results[resultKey(model)]?.ok ? 'text-emerald-600 dark:text-emerald-400' : 'text-red-600 dark:text-red-400'" :title="results[resultKey(model)]?.message">
              {{ results[resultKey(model)]?.message }}
            </span>
            <button type="button" class="btn btn-secondary h-8 shrink-0 px-3 text-xs" :disabled="Boolean(testingModel) || (endpoint === 'Images' && !imageChargeAccepted)" @click="testModel(model)">
              {{ testingModel === model ? t('modelTest.testing') : t('modelTest.test') }}
            </button>
          </div>
          <div v-if="!visibleModels.length" class="py-8 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('modelTest.noModels') }}</div>
        </div>
      </template>
    </main>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { buildGatewayUrl } from '@/api/url'
import { buildModelTestRequest, parseModelTestModels, type ModelTestEndpoint } from '@/utils/modelTest'

type TestResult = { ok: boolean; message: string }
const { t } = useI18n()
const endpoints: ModelTestEndpoint[] = ['Chat', 'Responses', 'Images']
const endpoint = ref<ModelTestEndpoint>('Chat')
const apiKey = ref('')
const models = ref<string[]>([])
const search = ref('')
const loadingModels = ref(false)
const testingModel = ref('')
const imageChargeAccepted = ref(false)
const loadError = ref('')
const results = ref<Record<string, TestResult>>({})

const visibleModels = computed(() => models.value.filter(model =>
  model.toLowerCase().includes(search.value.trim().toLowerCase())
))
const resultKey = (model: string) => `${endpoint.value}:${model}`

function clearModels() {
  models.value = []
  results.value = {}
  loadError.value = ''
  imageChargeAccepted.value = false
}

async function loadModels() {
  clearModels()
  loadingModels.value = true
  try {
    const response = await fetch(buildGatewayUrl('/v1/models'), {
      headers: { Authorization: `Bearer ${apiKey.value.trim()}` },
      cache: 'no-store'
    })
    const payload = await response.json()
    if (!response.ok) throw new Error(payload?.error?.message || t('modelTest.loadFailed'))
    models.value = parseModelTestModels(payload)
    if (!models.value.length) loadError.value = t('modelTest.noModels')
  } catch (error) {
    loadError.value = error instanceof Error ? error.message : t('modelTest.loadFailed')
  } finally {
    loadingModels.value = false
  }
}

async function testModel(model: string) {
  if (!apiKey.value.trim() || (endpoint.value === 'Images' && !imageChargeAccepted.value)) return
  testingModel.value = model
  const selectedEndpoint = endpoint.value
  const key = resultKey(model)
  const { path, body } = buildModelTestRequest(selectedEndpoint, model)
  const started = performance.now()
  const controller = new AbortController()
  const timeout = window.setTimeout(() => controller.abort(), selectedEndpoint === 'Images' ? 120_000 : 60_000)
  try {
    const response = await fetch(buildGatewayUrl(path), {
      method: 'POST',
      headers: { Authorization: `Bearer ${apiKey.value.trim()}`, 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
      cache: 'no-store',
      signal: controller.signal
    })
    if (!response.ok) {
      const payload = await response.json().catch(() => null)
      throw new Error(payload?.error?.message || `HTTP ${response.status}`)
    }
    await response.body?.cancel()
    results.value[key] = { ok: true, message: t('modelTest.success', { seconds: ((performance.now() - started) / 1000).toFixed(1) }) }
  } catch (error) {
    results.value[key] = { ok: false, message: error instanceof Error ? error.message : t('modelTest.failed') }
  } finally {
    window.clearTimeout(timeout)
    testingModel.value = ''
  }
}
</script>
