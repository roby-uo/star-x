import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import BillingManagementView from '../BillingManagementView.vue'
import CostProfileEditor from '@/components/admin/billing/CostProfileEditor.vue'
import { account, config, group, profile } from '@/components/admin/billing/__tests__/fixtures'
import type { BillingManagementConfig } from '@/api/admin/billingManagement'

const { getConfig, updateConfig, preview, catalog, listAccounts, listChannels, getGroups, updateGroup } = vi.hoisted(() => ({ getConfig: vi.fn(), updateConfig: vi.fn(), preview: vi.fn(), catalog: vi.fn(), listAccounts: vi.fn(), listChannels: vi.fn(), getGroups: vi.fn(), updateGroup: vi.fn() }))
vi.mock('@/api/admin/billingManagement', () => ({ billingManagementAPI: { getConfig, updateConfig, preview, catalog } }))
vi.mock('@/api/admin/channels', () => ({ default: { list: listChannels } }))
vi.mock('@/api/admin/accounts', () => ({ accountsAPI: { list: listAccounts } }))
vi.mock('@/api/admin/groups', () => ({ getAllIncludingInactive: getGroups, update: updateGroup }))
vi.mock('vue-router', () => ({ useRoute: () => ({ query: {} }) }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))
vi.mock('@/components/admin/AdminWorkspaceTabs.vue', () => ({ default: { template: '<div />' } }))
const mounted: VueWrapper[] = []
afterEach(() => { mounted.forEach(wrapper => wrapper.unmount()); mounted.length = 0 })
beforeEach(() => {
  vi.clearAllMocks()
  getConfig.mockResolvedValue(config())
  catalog.mockResolvedValue([])
  updateConfig.mockImplementation(async (next: BillingManagementConfig) => ({ ...next, version: next.version + 1 }))
  listAccounts.mockResolvedValue({ items: [account()], total: 1 })
  listChannels.mockResolvedValue({ items: [], total: 0 })
  getGroups.mockResolvedValue([group()])
  updateGroup.mockImplementation(async (id, data) => ({ ...group(), id, ...data }))
})
async function page() {
  const wrapper = mount(BillingManagementView, { global: { stubs: { AdminWorkspaceTabs: true, Icon: true, BillingPreviewDialog: true, RouterLink: { template: '<a><slot /></a>' }, ConfirmDialog: true } } })
  mounted.push(wrapper); await flushPromises(); return wrapper
}
function field(wrapper: VueWrapper, label: string) { const row = wrapper.findAll('label').find(item => item.text().startsWith(label)); if (!row) throw new Error(label); return row.get('input') }
async function editProfile(wrapper: VueWrapper) { await wrapper.findAll('button').find(item => item.text() === '编辑')!.trigger('click'); return wrapper.findComponent(CostProfileEditor) }

describe('BillingManagementView simplified pricing', () => {
  it('saves one API multiplier without changing procurement or group rates', async () => {
    const wrapper = await page()
    await wrapper.get('#api-sale-multiplier').setValue('2')
    await wrapper.get('#api-sale-multiplier').element.closest('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
    await flushPromises()
    expect(updateConfig).toHaveBeenCalledWith(expect.objectContaining({ version: 4, simple_billing_enabled: true, api_sale_multiplier: 2, usd_to_cny_rate: 6.2, cost_profiles: config().cost_profiles }))
    expect(updateGroup).not.toHaveBeenCalled()
  })
  it('saves the account group rate without changing API pricing', async () => {
    const wrapper = await page()
    const rate = wrapper.get('[aria-label="账号分组倍率"]')
    await rate.setValue('1.2')
    await rate.element.closest('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
    await flushPromises()
    expect(updateGroup).toHaveBeenCalledWith(7, { rate_multiplier: 1.2 })
    expect(updateConfig).not.toHaveBeenCalled()
  })
  it('preserves disabled and expired supplier configurations when editing one profile', async () => {
    const stored = config({ cost_profiles: [profile(), profile({ id: 'historic-disabled', enabled: false }), profile({ id: 'historic-expired', valid_until: '2025-01-01T00:00:00Z' })] })
    getConfig.mockResolvedValue(stored)
    const wrapper = await page()
    const editor = await editProfile(wrapper)
    await field(editor, '方案名称').setValue('只改采购名称')
    await editor.get('form').trigger('submit'); await flushPromises()
    const saved = updateConfig.mock.calls[0]![0] as BillingManagementConfig
    expect(saved.api_sale_multiplier).toBe(1.4)
    expect(saved.cost_profiles[0]?.name).toBe('只改采购名称')
    expect(saved.cost_profiles.slice(1)).toEqual(stored.cost_profiles.slice(1))
    expect(wrapper.findComponent(CostProfileEditor).exists()).toBe(false)
  })
  it('retains the unsaved procurement draft on a version conflict', async () => {
    updateConfig.mockRejectedValue({ status: 409, message: '配置已被其他操作修改' })
    const wrapper = await page(); const editor = await editProfile(wrapper)
    await field(editor, '方案名称').setValue('需要保留的草稿')
    await editor.get('form').trigger('submit'); await flushPromises()
    expect(wrapper.findComponent(CostProfileEditor).exists()).toBe(true)
    expect((field(editor, '方案名称').element as HTMLInputElement).value).toBe('需要保留的草稿')
    expect(editor.get('[role="alert"]').text()).toContain('配置已被其他操作修改')
  })
  it('rejects an invalid API rate without sending a mutation', async () => {
    const wrapper = await page(); await wrapper.get('#api-sale-multiplier').setValue('0')
    await wrapper.get('#api-sale-multiplier').element.closest('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true })); await flushPromises()
    expect(updateConfig).not.toHaveBeenCalled()
    expect(wrapper.get('[role="alert"]').text()).toContain('大于 0')
  })
  it('does not present an unconfigured source as confirmed or free', async () => {
    getConfig.mockResolvedValue(config({ cost_profiles: [] }))
    const wrapper = await page()
    expect(wrapper.text()).toContain('价格待核对')
    expect(wrapper.text()).not.toContain('¥0.00')
    expect(wrapper.find('nav').exists()).toBe(false)
  })
  it('renders the server catalogue in yuan with exactly one applied multiplier', async () => {
    catalog.mockResolvedValue([{ key:'account:7:MiniMax-M3', platform:'openai', model:'MiniMax-M3', upstream_model:'MiniMax-M3', modality:'chat', account_ids:[1], group_id:7, route:'api', source:'official_reference', provider:'minimax', region:'cn', status:'ready', currency:'CNY', config_version:4, multiplier:1.4, base_prices:[{label:'输入',unit:'per_million_tokens',amount_cny:2.1}], reason:'官方参考价' }])
    const wrapper = await page()
    const table = wrapper.get('table').text()
    expect(table).toContain('¥2.1')
    expect(table).toContain('¥2.94')
    expect(table).toContain('API 1.4×')
    expect(table).not.toContain('¥4.116')
  })
  it('does not fall back to obsolete channel prices when the live catalogue is missing', async () => {
    listAccounts.mockResolvedValue({ items: [account(1, 'oauth')], total: 1 })
    listChannels.mockResolvedValue({ items: [{ id: 2, name: '旧规则', status: 'active', group_ids: [7], features_config: {}, model_pricing: [{ platform: 'openai', models: ['MiniMax-M3'], billing_mode: 'token', input_price: 999, intervals: [] }] }], total: 1 })
    const wrapper = await page()
    expect(wrapper.get('table').text()).toContain('待核对 · 选择上游后试算')
    expect(wrapper.get('table').text()).not.toContain('999')
    expect(wrapper.get('table').text()).not.toContain('分组倍率 1.4')
  })
})
