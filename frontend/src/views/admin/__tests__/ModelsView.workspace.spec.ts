import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import ModelsView from '../ModelsView.vue'
import { account, config, group } from '@/components/admin/billing/__tests__/fixtures'

const mocks = vi.hoisted(() => ({
  getConfig: vi.fn(), catalog: vi.fn(), listAccounts: vi.fn(), listChannels: vi.fn(), getGroups: vi.fn(),
  showError: vi.fn(), showSuccess: vi.fn()
}))
vi.mock('@/api/admin/billingManagement', () => ({ billingManagementAPI: { getConfig: mocks.getConfig, catalog: mocks.catalog } }))
vi.mock('@/api/admin/accounts', () => ({ accountsAPI: { list: mocks.listAccounts } }))
vi.mock('@/api/admin/channels', () => ({ default: { list: mocks.listChannels } }))
vi.mock('@/api/admin/groups', () => ({ getAllIncludingInactive: mocks.getGroups }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: mocks.showError, showSuccess: mocks.showSuccess }) }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))
vi.mock('@/components/admin/AdminWorkspaceTabs.vue', () => ({ default: { template: '<div />' } }))

const mounted: VueWrapper[] = []
afterEach(() => { mounted.forEach(wrapper => wrapper.unmount()); mounted.length = 0 })
beforeEach(() => {
  vi.clearAllMocks()
  mocks.getConfig.mockResolvedValue(config())
  mocks.catalog.mockResolvedValue([{
    key: 'm3-7', platform: 'openai', model: 'MiniMax-M3', account_ids: [1], group_id: 7,
    route: 'api', source: 'official_reference', status: 'ready', multiplier: 1.4,
    base_prices: [{ label: '输入', unit: 'per_million_tokens', amount_cny: 2.1 }]
  }])
  mocks.listAccounts.mockResolvedValue({ items: [account()], total: 1 })
  mocks.listChannels.mockResolvedValue({ items: [{
    id: 2, name: '旧价格范围', status: 'active', group_ids: [7], features_config: {},
    model_pricing: [{ platform: 'openai', models: ['MiniMax-M3'], billing_mode: 'token', input_price: 999, intervals: [] }]
  }], total: 1 })
  mocks.getGroups.mockResolvedValue([group()])
})
async function page() {
  const wrapper = mount(ModelsView, { global: { stubs: {
    AppLayout: { template: '<main><slot /></main>' },
    TablePageLayout: { template: '<div><slot name="filters" /><slot name="table" /></div>' },
    AdminWorkspaceTabs: true, Icon: true, ModelServiceEditor: true,
    BillingPreviewDialog: { name: 'BillingPreviewDialog', props: ['model', 'accounts', 'groups', 'profiles', 'models', 'initialAccountId', 'initialGroupId'], template: '<div role="dialog" />' },
    RouterLink: { template: '<a><slot /></a>' }
  } } })
  mounted.push(wrapper); await flushPromises(); return wrapper
}

describe('model management workspace', () => {
  it('shows current procurement readiness instead of obsolete channel sale prices', async () => {
    const wrapper = await page()
    expect(wrapper.get('table').text()).toContain('基准价已匹配')
    expect(wrapper.get('table').text()).toContain('API 1.4×')
    expect(wrapper.text()).not.toContain('999')
    expect(wrapper.text()).not.toContain('基础售价')
    expect(wrapper.text()).not.toContain('沿用系统／分组定价')
  })

  it('distinguishes an open model from an incompletely confirmed procurement price', async () => {
    const ready = (await mocks.catalog())[0]
    mocks.catalog.mockResolvedValue([ready, { ...ready, key: 'm3-unknown', status: 'unknown', base_prices: [] }])
    const wrapper = await page()
    expect(wrapper.get('table').text()).toContain('已开放')
    expect(wrapper.get('table').text()).toContain('部分规格待确认')
  })

  it('keeps the model directory usable when price loading fails and recovers on retry', async () => {
    mocks.getConfig.mockRejectedValueOnce(new Error('采购价暂不可用'))
    const wrapper = await page()
    expect(wrapper.get('table').text()).toContain('MiniMax-M3')
    expect(wrapper.get('[role="alert"]').text()).toContain('采购价暂不可用')
    expect(wrapper.get('[aria-label="试算 MiniMax-M3"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[role="alert"] button').trigger('click'); await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    expect(wrapper.get('[aria-label="试算 MiniMax-M3"]').attributes('disabled')).toBeUndefined()
  })

  it('identifies paused source accounts without presenting a model as ready for dispatch', async () => {
    mocks.listAccounts.mockResolvedValue({ items: [{ ...account(), schedulable: false }], total: 1 })
    mocks.catalog.mockResolvedValue([])
    const wrapper = await page()
    expect(wrapper.get('table').text()).toContain('上游暂停')
    expect(wrapper.get('table').text()).toContain('来源暂停')
    expect(wrapper.get('table').text()).toContain('0 / 1 个来源可调度')
    expect(wrapper.get('table').text()).not.toContain('基准价已匹配')
  })

  it('opens an in-place quote with only the selected model sources, without a model call', async () => {
    const wrapper = await page()
    await wrapper.get('[aria-label="试算 MiniMax-M3"]').trigger('click')
    const dialog = wrapper.findComponent({ name: 'BillingPreviewDialog' })
    expect(dialog.exists()).toBe(true)
    expect(dialog.props()).toMatchObject({ model: 'MiniMax-M3', accounts: [account()], models: ['MiniMax-M3'], initialAccountId: 1, initialGroupId: 7 })
    expect(mocks.getConfig).toHaveBeenCalledTimes(1)
  })

  it('identifies the legacy pricing mode without claiming current API multiplier readiness', async () => {
    mocks.getConfig.mockResolvedValue(config({ simple_billing_enabled: false }))
    const wrapper = await page()
    expect(wrapper.get('table').text()).toContain('自定义定价')
    expect(wrapper.get('table').text()).not.toContain('API 1.4×')
  })
})
