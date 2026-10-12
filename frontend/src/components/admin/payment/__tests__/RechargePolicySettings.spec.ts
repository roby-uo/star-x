import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import RechargePolicySettings from '../RechargePolicySettings.vue'

const mocks = vi.hoisted(() => ({ getConfig: vi.fn(), updateConfig: vi.fn(), showSuccess: vi.fn(), showError: vi.fn() }))
vi.mock('@/api/admin/payment', () => ({ adminPaymentAPI: { getConfig: mocks.getConfig, updateConfig: mocks.updateConfig } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: mocks.showSuccess, showError: mocks.showError }) }))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key }),
}))

describe('CNY recharge policy settings', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.getConfig.mockResolvedValue({ data: { simple_billing_enabled: true, recharge_bonus_multiplier: 1 } })
    mocks.updateConfig.mockResolvedValue({})
  })

  it('saves the bonus through the real payment configuration API without changing the old exchange rules', async () => {
    const wrapper = mount(RechargePolicySettings)
    await flushPromises()
    expect(wrapper.emitted('mode')?.[0]).toEqual([true])
    await wrapper.get('input').setValue('1.5')
    await wrapper.get('button').trigger('click')
    await flushPromises()
    expect(mocks.updateConfig).toHaveBeenCalledWith({ recharge_bonus_multiplier: 1.5 })
  })

  it('keeps legacy payment controls available when simple billing is disabled', async () => {
    mocks.getConfig.mockResolvedValue({ data: { simple_billing_enabled: false, recharge_bonus_multiplier: 1 } })
    const wrapper = mount(RechargePolicySettings)
    await flushPromises()
    expect(wrapper.emitted('mode')?.[0]).toEqual([false])
    expect(wrapper.find('input').exists()).toBe(false)
  })

  it('does not save a zero bonus multiplier', async () => {
    const wrapper = mount(RechargePolicySettings)
    await flushPromises()
    await wrapper.get('input').setValue('0')
    expect(wrapper.get('button').attributes('disabled')).toBeDefined()
    expect(mocks.updateConfig).not.toHaveBeenCalled()
  })
})
