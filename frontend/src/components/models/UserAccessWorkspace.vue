<template>
  <nav class="mb-5 flex gap-5 border-b border-gray-200 dark:border-dark-700 sm:gap-7" :aria-label="t('nav.userApiAccess')">
    <router-link
      v-for="tab in tabs"
      :key="tab.path"
      :to="tab.path"
      class="relative -mb-px border-b-2 px-1 pb-3 pt-1 text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500"
      :class="isActive(tab.path)
        ? 'border-primary-500 text-primary-700 dark:text-primary-300'
        : 'border-transparent text-gray-500 hover:text-gray-900 dark:text-gray-400 dark:hover:text-gray-100'"
      :aria-current="isActive(tab.path) ? 'page' : undefined"
    >
      {{ t(tab.label) }}
    </router-link>
  </nav>
</template>

<script setup lang="ts">
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'

const route = useRoute()
const { t } = useI18n()
const tabs = [
  { path: '/model-catalog', label: 'nav.userAccessModels' },
  { path: '/keys', label: 'nav.userAccessKeys' }
]

function isActive(path: string): boolean {
  return route.path === path || (path === '/model-catalog' && route.path === '/model-test')
}
</script>
