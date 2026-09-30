<template>
  <AppLayout>
    <div class="space-y-5">
      <div>
        <p class="text-sm text-gray-500">用于核对异步视频调用、异常任务和结算。文本与图像调用请在使用记录中查看。</p>
      </div>
      <section class="rounded-xl border border-gray-200 bg-white p-5 dark:border-dark-700 dark:bg-dark-800">
        <MediaTaskList admin />
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import AppLayout from '@/components/layout/AppLayout.vue'
import MediaTaskList from '@/components/models/MediaTaskList.vue'
</script>
