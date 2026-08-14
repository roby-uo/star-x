<template>
  <span class="brand-icon" :class="`brand-${brand}`" aria-hidden="true">
    <PlatformIcon v-if="platform" :platform="platform" size="lg" />
    <svg v-else viewBox="0 0 24 24" fill="currentColor">
      <path v-if="brand === 'opencode'" d="M6 4h12v4H10v8h8v4H6V4zm6 6h6v4h-6v-4z" />
      <path v-else-if="brand === 'trae'" d="M24 20.541H3.428v-3.426H0V3.4h24V20.54zM3.428 17.115h17.144V6.827H3.428v10.288zm8.573-5.196l-2.425 2.424-2.424-2.424 2.424-2.424 2.425 2.424zm6.857-.001l-2.424 2.423-2.425-2.423 2.425-2.425 2.424 2.425z" />
      <path v-else-if="brand === 'tencent'" d="M9.976 1L24 9.8l-10.587.015L10.723 23H5.489L8.18 9.8H3.244L1 5.4h8.077L9.976 1z" />
      <path v-else d="M12 2a10 10 0 1010 10A10 10 0 0012 2zm4.2 14.5h-2.3l-1.9-3.1-1.9 3.1H7.8l3-4.7L8 7.5h2.3l1.7 2.8 1.7-2.8H16l-2.8 4.3z" />
    </svg>
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import type { GroupPlatform } from '@/types'

const props = defineProps<{ brand: string }>()
const platforms: Partial<Record<string, GroupPlatform>> = {
  claude: 'anthropic', openai: 'openai', gemini: 'gemini', antigravity: 'antigravity'
}
const platform = computed(() => platforms[props.brand])
</script>

<style scoped>
.brand-icon{display:grid;place-items:center;width:25px;height:25px;border-radius:7px;padding:4px;background:#f3f5f8;color:#171a20;flex:0 0 auto}.brand-icon svg{width:100%;height:100%}.brand-claude{background:#f7eee7;color:#d97757}.brand-openai{background:#e9f8f3;color:#10a37f}.brand-gemini{background:#edf3ff;color:#4285f4}.brand-antigravity{background:#f1edff;color:#7957e5}.brand-opencode{background:#f1ecec;color:#4b4646}.brand-hermes{background:#f4efff;color:#6d45b8}.brand-tencent{background:#eaf4ff;color:#1683e2}.brand-trae{background:#edf0ff;color:#4b57db}
</style>
