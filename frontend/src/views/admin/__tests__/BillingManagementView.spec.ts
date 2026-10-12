import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import BillingManagementView from '../BillingManagementView.vue'
import CostProfileEditor from '@/components/admin/billing/CostProfileEditor.vue'
import { account, config, group, profile } from '@/components/admin/billing/__tests__/fixtures'
import type { BillingManagementConfig } from '@/api/admin/billingManagement'

const { getConfig, updateConfig, preview, listAccounts, listChannels, getGroups, getPayment, updatePayment } = vi.hoisted(() => ({ getConfig: vi.fn(), updateConfig: vi.fn(), preview: vi.fn(), listAccounts: vi.fn(), listChannels: vi.fn(), getGroups: vi.fn(), getPayment: vi.fn(), updatePayment: vi.fn() }))
vi.mock('@/api/admin/billingManagement', () => ({ billingManagementAPI: { getConfig, updateConfig, preview } }))
vi.mock('@/api/admin/channels', () => ({ default: { list: listChannels } }))
vi.mock('@/api/admin/accounts', () => ({ accountsAPI: { list: listAccounts } }))
vi.mock('@/api/admin/groups', () => ({ getAllIncludingInactive: getGroups }))
vi.mock('@/api/admin/payment', () => ({ adminPaymentAPI: { getConfig: getPayment, updateConfig: updatePayment } }))
vi.mock('vue-router', () => ({ useRoute: () => ({ query: {} }) }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))
vi.mock('@/components/admin/ModelServiceEditor.vue', () => ({ default: { template: '<div />' } }))

const mounted: VueWrapper[] = []
afterEach(() => { mounted.forEach(wrapper => wrapper.unmount()); mounted.length = 0 })
beforeEach(() => {
  vi.clearAllMocks()
  getConfig.mockResolvedValue(config())
  updateConfig.mockImplementation(async (next: BillingManagementConfig) => ({ ...next, version: next.version + 1 }))
  listAccounts.mockResolvedValue({ items: [account()], total: 1 })
  listChannels.mockResolvedValue({ items: [], total: 0 })
  getGroups.mockResolvedValue([group()])
  getPayment.mockResolvedValue({ data: { enabled: true, balance_recharge_multiplier: 1, subscription_usd_to_cny_rate: 0, recharge_fee_rate: 2.5 } })
  updatePayment.mockResolvedValue({})
})

async function page() {
  const wrapper = mount(BillingManagementView, { global: { stubs: { AppLayout: { template: '<main><slot /></main>' }, Icon: true, ModelServiceEditor: true, BillingPreviewDialog: true, RouterLink: { template: '<a><slot /></a>' }, ConfirmDialog: { props: ['show', 'title'], emits: ['confirm', 'cancel'], template: '<section v-if="show" role="dialog" :aria-label="title"><button type="button" @click="$emit(\'confirm\')">确认应用</button></section>' } } } })
  mounted.push(wrapper)
  await flushPromises()
  return wrapper
}
function button(wrapper: VueWrapper, text: string) {
  const match = wrapper.findAll('button').find(item => item.text() === text)
  if (!match) throw new Error(`Missing button: ${text}`)
  return match
}
function field(wrapper: VueWrapper, label: string) {
  const match = wrapper.findAll('label').find(item => item.text().startsWith(label))
  if (!match) throw new Error(`Missing input: ${label}`)
  return match.get('input')
}

describe('BillingManagementView independent billing configuration', () => {
  it('saves procurement cost without changing exchange, recharge, or model sales fields', async () => {
    const stored = config({ cost_profiles: [profile(), profile({ id: 'historic-disabled', name: '历史停用方案', enabled: false }), profile({ id: 'historic-expired', name: '历史过期方案', valid_until: '2025-01-01T00:00:00Z' })] })
    getConfig.mockResolvedValue(stored)
    const wrapper = await page()
    await button(wrapper, '上游成本').trigger('click')
    const active = wrapper.findAll('article').find(item => item.text().includes('API 采购'))!
    await active.findAll('button').find(item => item.text() === '编辑')!.trigger('click')
    const editor = wrapper.findComponent(CostProfileEditor)
    await field(editor, '方案名称').setValue('只改采购名称')
    await editor.get('form').trigger('submit')
    await flushPromises()

    const saved = updateConfig.mock.calls[0]![0] as BillingManagementConfig
    expect(saved.version).toBe(4)
    expect(saved.usd_to_cny_rate).toBe(7)
    expect(saved.cost_profiles[0]?.name).toBe('只改采购名称')
    expect(saved.cost_profiles.slice(1)).toEqual(stored.cost_profiles.slice(1))
    expect(saved).not.toHaveProperty('balance_recharge_multiplier')
    expect(saved).not.toHaveProperty('model_pricing')
    expect(updatePayment).not.toHaveBeenCalled()
    expect(wrapper.findComponent(CostProfileEditor).exists()).toBe(false)
  })

  it('retains the actual editor and unsaved draft when the server returns 409', async () => {
    updateConfig.mockRejectedValue({ status: 409, message: '配置已被其他操作修改' })
    const wrapper = await page()
    await button(wrapper, '上游成本').trigger('click')
    await button(wrapper, '编辑').trigger('click')
    const editor = wrapper.findComponent(CostProfileEditor)
    await field(editor, '方案名称').setValue('需要保留的草稿')
    await field(editor, '单价采购折扣').setValue('0.75')
    await editor.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.findComponent(CostProfileEditor).exists()).toBe(true)
    expect((field(editor, '方案名称').element as HTMLInputElement).value).toBe('需要保留的草稿')
    expect((field(editor, '单价采购折扣').element as HTMLInputElement).value).toBe('0.75')
    expect(editor.get('[role="alert"]').text()).toContain('配置已被其他操作修改')
    expect(updatePayment).not.toHaveBeenCalled()
  })

  it('updates cost FX alone and retains other procurement and payment rules', async () => {
    const wrapper = await page()
    await button(wrapper, '充值与换算').trigger('click')
    await field(wrapper, '1 美元对应人民币').setValue('7.25')
    await field(wrapper, '汇率来源').setValue('实际结算汇率')
    await field(wrapper, '1 美元对应人民币').element.closest('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
    await flushPromises()
    expect(updateConfig).toHaveBeenCalledWith(expect.objectContaining({ version: 4, usd_to_cny_rate: 7.25, exchange_rate_source: '实际结算汇率', cost_profiles: config().cost_profiles }))
    expect(updateConfig.mock.calls[0]![0]).not.toHaveProperty('balance_recharge_multiplier')
    expect(updatePayment).not.toHaveBeenCalled()
  })

  it('preserves typed FX values after a version conflict', async () => {
    updateConfig.mockRejectedValue({ status: 409, message: '汇率版本冲突' })
    const wrapper = await page()
    await button(wrapper, '充值与换算').trigger('click')
    await field(wrapper, '1 美元对应人民币').setValue('7.4')
    await field(wrapper, '汇率来源').setValue('待保存汇率来源')
    field(wrapper, '1 美元对应人民币').element.closest('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
    await flushPromises()
    expect((field(wrapper, '1 美元对应人民币').element as HTMLInputElement).value).toBe('7.4')
    expect((field(wrapper, '汇率来源').element as HTMLInputElement).value).toBe('待保存汇率来源')
    expect(wrapper.get('[role="alert"]').text()).toContain('汇率版本冲突')
  })

  it('requires confirmation before applying recharge rules and updates only those fields', async () => {
    const wrapper = await page()
    await button(wrapper, '充值与换算').trigger('click')
    await field(wrapper, '入账倍率').setValue('0.2')
    await field(wrapper, '订阅价格 1 USD 对应 CNY').setValue('7.15')
    button(wrapper, '预览充值规则').element.closest('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
    await flushPromises()
    expect(updatePayment).not.toHaveBeenCalled()
    await wrapper.get('[aria-label="应用在线充值规则"] button').trigger('click')
    await flushPromises()
    expect(getPayment).toHaveBeenCalledTimes(2)
    expect(updatePayment).toHaveBeenCalledTimes(1)
    expect(updatePayment).toHaveBeenCalledWith({ balance_recharge_multiplier: 0.2, subscription_usd_to_cny_rate: 7.15 })
    expect(updateConfig).not.toHaveBeenCalled()
  })

  it('counts only effective procurement profiles as active cost coverage', async () => {
    getConfig.mockResolvedValue(config({ cost_profiles: [profile({ account_ids: [1] }), profile({ id: 'expired', account_ids: [2], valid_until: '2025-01-01T00:00:00Z' }), profile({ id: 'disabled', account_ids: [3], enabled: false })] }))
    listAccounts.mockResolvedValue({ items: [account(1), account(2), account(3)], total: 3 })
    const wrapper = await page()
    expect((wrapper.vm as any).coveredAccounts).toBe(1)
    expect((wrapper.vm as any).models[0].costSources).toBe(1)
    expect((wrapper.vm as any).models[0].accounts).toHaveLength(3)
  })

  it('rejects stale recharge configuration instead of overwriting another administrator change', async () => {
    const wrapper = await page()
    await button(wrapper, '充值与换算').trigger('click')
    await field(wrapper, '入账倍率').setValue('0.2')
    getPayment.mockResolvedValueOnce({ data: { balance_recharge_multiplier: 0.5, subscription_usd_to_cny_rate: 0 } })
    button(wrapper, '预览充值规则').element.closest('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
    await flushPromises()
    await wrapper.get('[aria-label="应用在线充值规则"] button').trigger('click')
    await flushPromises()
    expect(updatePayment).not.toHaveBeenCalled()
    expect(wrapper.get('[role="alert"]').text()).toContain('充值规则已被其他操作修改')
    expect((field(wrapper, '入账倍率').element as HTMLInputElement).value).toBe('0.2')
  })
})
