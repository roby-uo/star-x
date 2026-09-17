<template>
  <div class="mb-4 flex items-center justify-between rounded-lg bg-primary-50 p-3 dark:bg-primary-900/20">
    <div class="flex flex-wrap items-center gap-2">
      <span v-if="selectedIds.length > 0" class="text-sm font-medium text-primary-900 dark:text-primary-100">
        {{ t('admin.accounts.bulkActions.selected', { count: selectedIds.length }) }}
      </span>
      <span v-else class="text-sm font-medium text-primary-900 dark:text-primary-100">
        {{ t('admin.accounts.bulkEdit.title') }}
      </span>
      <template v-if="selectedIds.length > 0">
      <button
        @click="$emit('select-page')"
        class="text-xs font-medium text-primary-700 hover:text-primary-800 dark:text-primary-300 dark:hover:text-primary-200"
      >
        {{ t('admin.accounts.bulkActions.selectCurrentPage') }}
      </button>
      <span class="text-gray-300 dark:text-primary-800">•</span>
      <button
        @click="$emit('clear')"
        class="text-xs font-medium text-primary-700 hover:text-primary-800 dark:text-primary-300 dark:hover:text-primary-200"
      >
        {{ t('admin.accounts.bulkActions.clear') }}
      </button>
      </template>
    </div>
    <div class="flex items-start gap-2">
      <template v-if="selectedIds.length > 0">
        <button @click="$emit('delete')" class="btn btn-danger btn-sm">{{ t('admin.accounts.bulkActions.delete') }}</button>
        <button @click="$emit('reset-status')" class="btn btn-secondary btn-sm">{{ t('admin.accounts.bulkActions.resetStatus') }}</button>
        <button @click="$emit('refresh-token')" class="btn btn-secondary btn-sm">{{ t('admin.accounts.bulkActions.refreshToken') }}</button>
        <button @click="$emit('probe-upstream-billing')" class="btn btn-secondary btn-sm">{{ t('admin.accounts.bulkActions.probeUpstreamBilling') }}</button>
        <button @click="$emit('toggle-schedulable', true)" :disabled="autoRotationEnabled" class="btn btn-success btn-sm">{{ t('admin.accounts.bulkActions.enableScheduling') }}</button>
        <button @click="$emit('toggle-schedulable', false)" :disabled="autoRotationEnabled" class="btn btn-warning btn-sm">{{ t('admin.accounts.bulkActions.disableScheduling') }}</button>
        <button @click="$emit('edit-selected')" class="btn btn-primary btn-sm">{{ t('admin.accounts.bulkActions.edit') }}</button>
      </template>
      <select
        data-test="auto-rotation-interval"
        class="btn btn-secondary btn-sm cursor-pointer"
        :value="autoRotationIntervalSeconds"
        :disabled="autoRotationLoading"
        :aria-label="t('admin.accounts.autoRotation.interval')"
        @change="handleIntervalChange"
      >
        <option
          v-if="!rotationIntervals.some((option) => option.seconds === autoRotationIntervalSeconds)"
          :value="autoRotationIntervalSeconds"
          disabled
        >
          {{ t('admin.accounts.autoRotation.intervalOption', { hours: formatHours(autoRotationIntervalSeconds / 3600) }) }}
        </option>
        <option v-for="option in rotationIntervals" :key="option.seconds" :value="option.seconds">
          {{ t('admin.accounts.autoRotation.intervalOption', { hours: formatHours(option.hours) }) }}
        </option>
      </select>
      <div class="flex min-w-[9.5rem] flex-col items-end gap-1">
        <button
          data-test="auto-rotation-toggle"
          class="btn btn-sm"
          :class="autoRotationEnabled ? 'btn-success' : 'btn-secondary'"
          :disabled="autoRotationLoading"
          @click="$emit('toggle-auto-rotation')"
        >
          {{ autoRotationEnabled ? t('admin.accounts.autoRotation.enabled') : t('admin.accounts.autoRotation.enable') }}
        </button>
        <span
          v-if="autoRotationEnabled"
          data-test="auto-rotation-countdown"
          class="max-w-[16rem] text-right text-[11px] leading-4 text-primary-700 dark:text-primary-300"
        >
          {{ autoRotationCountdown || t('admin.accounts.autoRotation.waitingForEligible') }}
        </span>
      </div>
      <button @click="$emit('edit-filtered')" class="btn btn-primary btn-sm">
        {{ t('admin.accounts.bulkEdit.submit') }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const props = withDefaults(defineProps<{
  selectedIds: number[]
  autoRotationEnabled?: boolean
  autoRotationLoading?: boolean
  autoRotationCountdown?: string
  autoRotationIntervalSeconds?: number
}>(), {
  autoRotationEnabled: false,
  autoRotationLoading: false,
  autoRotationCountdown: '',
  autoRotationIntervalSeconds: 18000
})
const emit = defineEmits([
  'delete',
  'edit-selected',
  'edit-filtered',
  'clear',
  'select-page',
  'toggle-schedulable',
  'reset-status',
  'refresh-token',
  'probe-upstream-billing',
  'toggle-auto-rotation',
  'update-auto-rotation-interval'
])

const { t } = useI18n()
const rotationIntervals = Array.from({ length: 8 }, (_, index) => {
  const hours = (index + 1) / 2
  return { hours, seconds: hours * 3600 }
})

const formatHours = (hours: number) => Number.isInteger(hours) ? String(hours) : hours.toFixed(1)

const handleIntervalChange = (event: Event) => {
  const seconds = Number((event.target as HTMLSelectElement).value)
  if (Number.isFinite(seconds) && seconds !== props.autoRotationIntervalSeconds) {
    emit('update-auto-rotation-interval', seconds)
  }
}
</script>
