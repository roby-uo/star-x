<template>
  <input
    type="number"
    :value="displayValue"
    @input="updateAmount"
  />
</template>

<script setup lang="ts" generic="T extends number | string | null | undefined">
import { computed } from 'vue'
import { cnyToLedger, ledgerToCny } from '@/utils/currency'

const props = withDefaults(defineProps<{ modelValue: T; convert?: boolean }>(), { convert: true })
const emit = defineEmits<{ 'update:modelValue': [value: T] }>()
const displayValue = computed(() => {
  if (props.modelValue == null || props.modelValue === '') return ''
  return props.convert === false ? Number(props.modelValue) : Number(ledgerToCny(props.modelValue).toFixed(8))
})

function updateAmount(event: Event) {
  const raw = (event.target as HTMLInputElement).value
  const value = raw === '' ? null : props.convert === false ? Number(raw) : cnyToLedger(raw)
  emit('update:modelValue', value as T)
}
</script>
