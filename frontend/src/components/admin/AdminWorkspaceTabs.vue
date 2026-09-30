<template>
  <nav class="mb-4 flex flex-wrap gap-2" :aria-label="area === 'upstream' ? t('nav.upstreamManagement') : t('nav.modelServices')">
    <router-link
      v-for="tab in tabs"
      :key="tab.path"
      :to="tab.path"
      :id="tab.path === '/admin/groups' ? 'upstream-pools-tab' : undefined"
      class="rounded-lg border px-3 py-2 text-sm font-medium transition-colors"
      :class="route.path === tab.path
        ? 'border-primary-500 bg-primary-50 text-primary-700 dark:bg-primary-900/20 dark:text-primary-300'
        : 'border-gray-200 text-gray-600 hover:border-primary-300 hover:text-primary-700 dark:border-dark-600 dark:text-gray-300'"
      :aria-current="route.path === tab.path ? 'page' : undefined"
      @click="handleTabClick(tab.path)"
    >
      {{ t(tab.label) }}
    </router-link>
  </nav>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useOnboardingStore } from '@/stores'

const props = defineProps<{ area: 'upstream' | 'models' }>()
const route = useRoute()
const { t } = useI18n()
const onboardingStore = useOnboardingStore()

function handleTabClick(path: string) {
  if (path === '/admin/groups' && onboardingStore.isCurrentStep('#upstream-pools-tab')) {
    onboardingStore.nextStep(500)
  }
}

const tabs = computed(() => props.area === 'upstream'
  ? [
      { path: '/admin/accounts', label: 'nav.upstreamConnections' },
      { path: '/admin/groups', label: 'nav.groupRates' }
    ]
  : [
      { path: '/admin/models', label: 'nav.modelServices' }
    ])
</script>
