import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import PlanEditDialog from '../PlanEditDialog.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) => {
      if (key === 'payment.admin.subscriptionCnyPayPreview') return `preview ${params?.amount}`
      if (key === 'payment.admin.subscriptionCnyPayPreviewWithFee') return `fee ${params?.feeRate} ${params?.total}`
      return key
    },
  }),
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
  }),
}))

vi.mock('@/api/admin/payment', () => ({
  adminPaymentAPI: {
    createPlan: vi.fn(),
    updatePlan: vi.fn(),
  },
}))

function mountDialog(paymentConfig: Record<string, unknown> | null) {
  return mount(PlanEditDialog, {
    props: {
      show: true,
      plan: null,
      groups: [],
      paymentConfig,
    },
    global: {
      stubs: {
        BaseDialog: {
          props: ['show'],
          template: '<div v-if="show"><slot /><slot name="footer" /></div>',
        },
        Select: true,
        Icon: true,
        GroupBadge: true,
      },
    },
  })
}

describe('PlanEditDialog subscription CNY payment preview', () => {
  it('treats a new plan price as CNY and applies the cash fee without the legacy subscription exchange rate', async () => {
    const wrapper = mountDialog({
      subscription_usd_to_cny_rate: 7.15,
      recharge_fee_rate: 2.5,
    })

    await wrapper.find('input[type="number"]').setValue('9.99')

    expect(wrapper.text()).toContain('preview')
    expect(wrapper.text()).toContain('¥9.99')
    expect(wrapper.text()).toContain('fee 2.5')
    expect(wrapper.text()).toContain('¥10.24')
  })

  it('still quotes an explicit CNY price when no legacy subscription rate is configured', async () => {
    const wrapper = mountDialog({
      subscription_usd_to_cny_rate: 0,
      recharge_fee_rate: 2.5,
    })

    await wrapper.find('input[type="number"]').setValue('9.99')

    expect(wrapper.text()).toContain('preview ¥9.99')
    expect(wrapper.text()).toContain('fee 2.5 ¥10.24')
    expect(wrapper.text()).not.toContain('¥71.43')
  })
})
