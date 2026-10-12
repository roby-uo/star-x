import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { describe, expect, it } from 'vitest'
import AmountInput from '../AmountInput.vue'

function render(currency: string) {
  return mount(AmountInput, {
    props: { modelValue: null, currency },
    global: { plugins: [createI18n({ legacy: false, locale: 'zh', messages: { zh: {} }, missingWarn: false })] },
  })
}

describe('recharge amount inputs', () => {
  it('submits a CNY payment unchanged', async () => {
    const wrapper = render('CNY')
    await wrapper.get('input').setValue('100')
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([100])
  })

  it('submits the native USD amount from the same CNY input', async () => {
    const wrapper = render('USD')
    await wrapper.get('input').setValue('100')
    expect(wrapper.emitted('update:modelValue')?.[0]?.[0]).toBeCloseTo(100 / 6.2, 12)
  })
})
