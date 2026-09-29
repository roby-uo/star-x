<template>
  <div class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4" @click.self="emit('close')"><section role="dialog" aria-modal="true" aria-labelledby="model-details" class="max-h-[85vh] w-full max-w-xl space-y-4 overflow-y-auto rounded-xl bg-white p-6 dark:bg-dark-800">
    <div class="flex justify-between"><h2 id="model-details" class="font-semibold">{{ model.name }} · 规格与价格</h2><button class="btn btn-secondary" @click="emit('close')">关闭</button></div>
    <p>{{ model.group_name }} · 基础倍率 {{ model.rate_multiplier }}×</p>
    <template v-if="model.policy?.kind === 'video'">
      <p>时长：{{ model.policy.min_duration }}～{{ model.policy.max_duration }} 秒</p>
      <p>生成方式：{{ videoModes.filter(m => model.policy?.modes?.includes(m.value)).map(m => m.label).join('、') }}</p>
      <table class="w-full text-sm"><thead><tr><th class="py-2 text-left">分辨率</th><th class="text-left">基础售价／秒</th><th class="text-left">当前倍率后／秒</th></tr></thead><tbody><tr v-for="r in model.policy.resolutions" :key="r"><td class="py-2">{{ r }}</td><td>${{ model.policy.prices?.[r] }}</td><td>${{ ((model.policy.prices?.[r] || 0) * model.rate_multiplier).toFixed(4) }}</td></tr></tbody></table>
    </template>
    <template v-else>
      <p v-if="model.policy?.image_sizes?.length">开放尺寸：{{ model.policy.image_sizes.join('、') }}</p>
      <p v-if="model.policy?.max_images">单次图片上限：{{ model.policy.max_images }}</p>
      <div v-if="model.pricing" class="space-y-2 text-sm">
        <p>计费方式：{{ ({ token: '按用量', image: '图片计费', per_request: '按次' } as Record<string, string>)[model.pricing.billing_mode] || model.pricing.billing_mode }}</p>
        <p v-if="model.pricing.input_price != null">输入基础价：${{ model.pricing.input_price * 1000000 }}／百万 token</p>
        <p v-if="model.pricing.output_price != null">输出基础价：${{ model.pricing.output_price * 1000000 }}／百万 token</p>
        <p v-if="model.pricing.per_request_price != null">基础按次价：${{ model.pricing.per_request_price }}</p>
        <p v-for="(tier, index) in model.pricing.intervals" :key="index">{{ tier.tier_label || `用量区间 ${tier.min_tokens}～${tier.max_tokens ?? '不限'}` }}<span v-if="tier.per_request_price != null">：${{ tier.per_request_price }}／次</span><span v-if="tier.input_price != null"> · 输入 ${{ tier.input_price * 1000000 }}／百万 token</span><span v-if="tier.output_price != null"> · 输出 ${{ tier.output_price * 1000000 }}／百万 token</span></p>
      </div>
      <p v-else class="text-sm text-gray-500">此模型沿用分组与系统定价，未设置渠道覆盖价。</p>
    </template>
    <p class="text-xs text-gray-500">价格币种为 USD。视频生成前会按选定密钥重新报价；文本与图片的最终费用还取决于实际用量、独立媒体倍率或峰时规则。密钥自身权限可能进一步限制可用规格。</p>
  </section></div>
</template>
<script setup lang="ts">
import type { UserAvailableModel } from '@/api/channels'
import { videoModes } from '@/types/modelService'
defineProps<{ model: UserAvailableModel }>()
const emit = defineEmits<{ close: [] }>()
</script>
