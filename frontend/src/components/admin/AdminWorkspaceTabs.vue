<template>
  <div ref="workspaceNav" class="mb-5" @keydown.esc.prevent.stop="closeSettings(true)">
    <nav class="flex items-center justify-between gap-3 border-b border-gray-200 dark:border-dark-700" :aria-label="t('nav.modelManagement')">
      <div class="flex gap-5 sm:gap-7">
        <router-link
          v-for="tab in tabs"
          :key="tab.path"
          :to="tab.path"
          :id="tab.path === '/admin/accounts' ? 'sidebar-channel-manage' : undefined"
          class="relative -mb-px border-b-2 px-1 pb-3 pt-1 text-sm font-medium transition-colors"
          :class="route.path === tab.path
            ? 'border-primary-500 text-primary-700 dark:text-primary-300'
            : 'border-transparent text-gray-500 hover:text-gray-900 dark:text-gray-400 dark:hover:text-gray-100'"
          :aria-current="route.path === tab.path ? 'page' : undefined"
          @click="handleTabClick(tab.path)"
        >
          {{ t(tab.label) }}
        </router-link>
      </div>

      <div class="relative self-start">
        <button
          id="model-management-settings-trigger"
          ref="settingsTrigger"
          type="button"
          class="inline-flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-sm font-medium transition-colors hover:bg-gray-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 dark:hover:bg-dark-700"
          :class="currentSettingsPage ? 'text-primary-700 dark:text-primary-300' : 'text-gray-600 dark:text-gray-300'"
          :aria-expanded="settingsOpen"
          aria-controls="model-management-settings-menu"
          @click="toggleSettings"
        >
          {{ t('nav.modelManagementSettings') }}
          <svg class="h-4 w-4 transition-transform" :class="{ 'rotate-180': settingsOpen }" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
            <path d="m6 9 6 6 6-6" stroke-linecap="round" stroke-linejoin="round" />
          </svg>
        </button>
        <div
          v-if="settingsOpen"
          id="model-management-settings-menu"
          class="absolute right-0 z-40 mt-2 w-56 rounded-xl border border-gray-200 bg-white p-1.5 shadow-lg dark:border-dark-600 dark:bg-dark-800"
        >
          <router-link
            v-for="item in settingsItems"
            :key="item.path"
            :to="item.path"
            :id="item.path === '/admin/groups' ? 'upstream-pools-tab' : undefined"
            class="block rounded-lg px-3 py-2 text-sm transition-colors"
            :class="route.path === item.path
              ? 'bg-primary-50 text-primary-700 dark:bg-primary-900/20 dark:text-primary-300'
              : 'text-gray-700 hover:bg-gray-50 dark:text-gray-200 dark:hover:bg-dark-700'"
            :aria-current="route.path === item.path ? 'page' : undefined"
            @click="handleTabClick(item.path)"
          >
            {{ t(item.label) }}
          </router-link>
          <div class="my-1.5 border-t border-gray-100 dark:border-dark-700" />
          <p class="px-3 pb-1 pt-1 text-xs text-gray-400">{{ t('nav.modelManagementAdvanced') }}</p>
          <router-link
            v-for="item in advancedItems"
            :key="item.path"
            :to="item.path"
            class="block rounded-lg px-3 py-2 text-sm transition-colors"
            :class="route.path === item.path
              ? 'bg-primary-50 text-primary-700 dark:bg-primary-900/20 dark:text-primary-300'
              : 'text-gray-600 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-dark-700'"
            :aria-current="route.path === item.path ? 'page' : undefined"
            @click="closeSettings()"
          >
            {{ t(item.label) }}
          </router-link>
        </div>
      </div>
    </nav>
    <p v-if="currentSettingsPage" class="mt-4 text-sm text-gray-500 dark:text-gray-400" :aria-label="t('nav.modelManagementLocation')">
      {{ t('nav.modelManagementSettings') }}
      <span class="mx-2 text-gray-300 dark:text-gray-600" aria-hidden="true">/</span>
      <span class="font-medium text-gray-900 dark:text-gray-100">{{ t(currentSettingsPage.label) }}</span>
    </p>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useOnboardingStore } from '@/stores'

// Existing views retain their area prop while sharing one workspace navigation.
defineProps<{ area?: 'upstream' | 'models' | 'billing' }>()
const route = useRoute()
const { t } = useI18n()
const onboardingStore = useOnboardingStore()
const authStore = useAuthStore()
const workspaceNav = ref<HTMLElement | null>(null)
const settingsTrigger = ref<HTMLButtonElement | null>(null)
const settingsOpen = ref(false)

const tabs = [
  { path: '/admin/models', label: 'nav.modelManagementModels' },
  { path: '/admin/accounts', label: 'nav.modelManagementUpstreams' }
]
const settingsItems = computed(() => [
  { path: '/admin/billing-management', label: 'nav.modelManagementBilling' },
  ...(!authStore.isSimpleMode ? [{ path: '/admin/groups', label: 'nav.modelManagementGroups' }] : [])
])
const advancedItems = [
  { path: '/admin/channels/pricing', label: 'nav.advancedChannels' },
  { path: '/admin/channels/monitor', label: 'nav.upstreamMonitor' }
]
const currentSettingsPage = computed(() => [...settingsItems.value, ...advancedItems].find(item => item.path === route.path))

function closeSettings(returnFocus = false) {
  if (!settingsOpen.value) return
  settingsOpen.value = false
  if (returnFocus) settingsTrigger.value?.focus()
}

function toggleSettings() {
  settingsOpen.value = !settingsOpen.value
  if (settingsOpen.value && onboardingStore.isCurrentStep('#model-management-settings-trigger')) {
    void onboardingStore.nextStep(500)
  }
}

function handleTabClick(path: string) {
  closeSettings()
  const selector = path === '/admin/groups'
    ? '#upstream-pools-tab'
    : path === '/admin/accounts' ? '#sidebar-channel-manage' : undefined
  if (selector && onboardingStore.isCurrentStep(selector)) {
    void onboardingStore.nextStep(500)
  }
}

function handleOutsideClick(event: MouseEvent) {
  if (event.target instanceof Node && !workspaceNav.value?.contains(event.target)) closeSettings()
}

watch(() => route.fullPath, () => closeSettings())
onMounted(() => document.addEventListener('click', handleOutsideClick))
onBeforeUnmount(() => document.removeEventListener('click', handleOutsideClick))
</script>
