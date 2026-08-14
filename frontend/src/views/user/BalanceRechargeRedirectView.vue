<template>
  <div class="flex min-h-screen items-center justify-center bg-gray-50 dark:bg-dark-900">
    <p class="text-sm text-gray-500 dark:text-dark-300">{{ t('redeem.redirectingToStore') }}</p>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

onMounted(() => {
  window.location.replace('https://pay.ldxp.cn/shop/starX')
})
</script>
