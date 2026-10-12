import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import LedgerAmountInput from '../LedgerAmountInput.vue'

describe('LedgerAmountInput', () => {
  it('edits CNY and emits the existing ledger unit', async () => {
    const wrapper = mount(LedgerAmountInput, { props: { modelValue: 100 } })
    expect(wrapper.get('input').element.value).toBe('620')
    await wrapper.get('input').setValue('100')
    expect(wrapper.emitted('update:modelValue')?.[0]?.[0]).toBeCloseTo(100 / 6.2, 12)
  })

  it('preserves missing prices when an amount is cleared', async () => {
    const wrapper = mount(LedgerAmountInput, { props: { modelValue: 1 as number | null } })
    await wrapper.get('input').setValue('')
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([null])
  })

  it('does not convert a redeem code concurrency value', async () => {
    const wrapper = mount(LedgerAmountInput, { props: { modelValue: 10, convert: false } })
    expect(wrapper.get('input').element.value).toBe('10')
    await wrapper.get('input').setValue('20')
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([20])
  })
})
