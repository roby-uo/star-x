<template>
  <section v-if="simpleBillingEnabled" class="rounded-xl border border-blue-100 bg-blue-50/50 p-4 dark:border-blue-900/50 dark:bg-blue-950/20">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h4 class="text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.settings.payment.cnyRechargeTitle') }}</h4>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.settings.payment.cnyRechargeHint') }}</p>
      </div>
      <div class="flex items-end gap-2">
        <label class="block text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.settings.payment.rechargeBonusMultiplier') }}
          <input v-model.number="bonusMultiplier" type="number" min="0.01" step="any" class="input mt-1 w-28" :disabled="!loaded || saving" />
        </label>
        <button type="button" class="btn btn-secondary" :disabled="!loaded || saving || !validMultiplier" @click="save">
          {{ saving ? t('common.saving') : t('common.save') }}
        </button>
      </div>
    </div>
    <p v-if="loaded && validMultiplier" class="mt-3 text-sm font-medium text-blue-700 dark:text-blue-300">
      {{ t('admin.settings.payment.cnyRechargePreview', { payment: formatCny(100), credit: formatCny(100 * bonusMultiplier) }) }}
    </p>
    <button v-if="loadFailed" type="button" class="mt-2 text-sm text-primary-600" @click="load">{{ t('common.retry') }}</button>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminPaymentAPI } from '@/api/admin/payment'
import { useAppStore } from '@/stores/app'
import { formatCny } from '@/utils/currency'

const emit = defineEmits<{ mode: [simpleBillingEnabled: boolean] }>()
const { t } = useI18n()
const appStore = useAppStore()
const simpleBillingEnabled = ref(true)
const bonusMultiplier = ref(1)
const loaded = ref(false)
const saving = ref(false)
const loadFailed = ref(false)
const validMultiplier = computed(() => Number.isFinite(bonusMultiplier.value) && bonusMultiplier.value > 0)

async function load() {
  loadFailed.value = false
  try {
    const { data } = await adminPaymentAPI.getConfig()
    simpleBillingEnabled.value = data.simple_billing_enabled !== false
    bonusMultiplier.value = data.recharge_bonus_multiplier ?? 1
    loaded.value = true
    emit('mode', simpleBillingEnabled.value)
  } catch {
    loadFailed.value = true
    appStore.showError(t('common.error'))
  }
}

async function save() {
  if (!validMultiplier.value || !loaded.value) return
  saving.value = true
  try {
    await adminPaymentAPI.updateConfig({ recharge_bonus_multiplier: bonusMultiplier.value })
    appStore.showSuccess(t('common.saved'))
  } catch {
    appStore.showError(t('common.error'))
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>
