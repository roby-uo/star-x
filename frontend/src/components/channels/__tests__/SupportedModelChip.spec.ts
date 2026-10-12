import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import SupportedModelChip from '../SupportedModelChip.vue'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

describe('SupportedModelChip pricing visibility', () => {
  const model = { name: 'example-model', platform: 'openai', pricing: { billing_mode: 'token' as const, input_price: 0.000002, output_price: 0.000005, cache_write_price: null, cache_read_price: null, image_input_price: null, image_output_price: null, per_request_price: null, intervals: [] } }

  it('omits the entire price tooltip in user mode, including legacy price data', async () => {
    const wrapper = mount(SupportedModelChip, { props: { model, showPricing: false }, global: { stubs: { PlatformIcon: true, Teleport: true } } })
    await wrapper.find('span').trigger('mouseenter')
    expect(wrapper.text()).toContain('example-model')
    expect(wrapper.find('[role="tooltip"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('inputPrice')
  })

  it('preserves the pricing tooltip for administrator callers', () => {
    const wrapper = mount(SupportedModelChip, { props: { model }, global: { stubs: { PlatformIcon: true, Teleport: true } } })
    expect(wrapper.find('[role="tooltip"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('inputPrice')
  })
})
