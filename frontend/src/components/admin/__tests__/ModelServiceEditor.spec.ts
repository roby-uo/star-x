import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import type { Account, AdminGroup } from '@/types'
import type { Channel } from '@/api/admin/channels'
import channelsAPI from '@/api/admin/channels'
import { getAllIncludingInactive } from '@/api/admin/groups'
import { billingManagementAPI } from '@/api/admin/billingManagement'
import ModelServiceEditor from '../ModelServiceEditor.vue'
import BillingPreviewDialog from '../billing/BillingPreviewDialog.vue'
import { config, preview, profile } from '../billing/__tests__/fixtures'

vi.mock('@/api/admin/channels', () => ({ default: { getById: vi.fn(), update: vi.fn(), create: vi.fn() } }))
vi.mock('@/api/admin/groups', () => ({ getAllIncludingInactive: vi.fn() }))
vi.mock('@/api/admin/billingManagement', () => ({ billingManagementAPI: { getConfig: vi.fn(), preview: vi.fn() } }))

const channel = (id = 4, groupID = 1): Channel => ({
  id, name: `范围 ${id}`, status: 'active', group_ids: [groupID], model_mapping: {}, restrict_models: false,
  model_pricing: [{ models: ['alpha', 'beta'], platform: 'openai', billing_mode: 'token', input_price: 0.000001,
    output_price: 0.000002, cache_read_price: null, cache_write_price: null, image_input_price: null,
    image_output_price: null, per_request_price: null, intervals: [] }],
  features_config: { other_feature: true, model_services: [{ model: 'alpha', platform: 'openai', kind: 'chat', state: 'published', groups: { [groupID]: { enabled: true, resolutions: [], max_duration: 15, modes: [] } } }] }
} as Channel)

async function editor(channels = [channel()], overrides: Partial<{ model: string; kind: 'chat' | 'image' | 'video'; accounts: Account[] }> = {}) {
  const wrapper = mount(ModelServiceEditor, { props: { model: 'alpha', platform: 'openai', kind: 'chat', channels,
    accounts: [{ id: 1, name: '测试账号', platform: 'openai', status: 'active', schedulable: true, group_ids: [1, 2], configured_models: { alpha: 'alpha' } }] as Account[], ...overrides }, attachTo: document.body,
    global: { plugins: [createI18n({ legacy: false, locale: 'zh', messages: { zh: {} } })], stubs: { RouterLink: { template: '<a><slot /></a>' } } } })
  await flushPromises()
  return wrapper
}

describe('model service publishing flow', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(billingManagementAPI.getConfig).mockResolvedValue({ version: 0, simple_billing_enabled: false, api_sale_multiplier: 1.4, usd_to_cny_rate: 6.2, exchange_rate_source: '', exchange_rate_effective_at: null, cost_profiles: [] })
    vi.mocked(getAllIncludingInactive).mockResolvedValue([{ id: 1, name: '普通用户', platform: 'openai', status: 'active', rate_multiplier: 1.4, allow_image_generation: true },
      { id: 2, name: 'VIP', platform: 'openai', status: 'active', rate_multiplier: 1, allow_image_generation: true }] as AdminGroup[])
    vi.mocked(channelsAPI.getById).mockResolvedValue(channel())
    vi.mocked(channelsAPI.update).mockResolvedValue(channel())
    vi.mocked(billingManagementAPI.preview).mockResolvedValue(preview())
  })

  it('requires preview confirmation and keeps published state while splitting the edited price', async () => {
    const wrapper = await editor()
    expect(wrapper.text()).not.toContain('保存草稿')
    expect(wrapper.text()).not.toContain('费用试算')
    await wrapper.findAll('input[placeholder="沿用默认"]')[0]!.setValue('9')
    await wrapper.findAll('button').find(button => button.text() === '预览并应用')!.trigger('click')
    expect(channelsAPI.update).not.toHaveBeenCalled()
    expect(wrapper.get('[role="alertdialog"]').text()).toContain('普通用户')
    await wrapper.findAll('button').find(button => button.text() === '确认')!.trigger('click')
    await flushPromises()
    expect(channelsAPI.update).toHaveBeenCalledWith(4, expect.objectContaining({
      features_config: expect.objectContaining({ other_feature: true, model_services: [expect.objectContaining({ state: 'published' })] }),
      model_pricing: [expect.objectContaining({ models: ['beta'], input_price: 0.000001 }), expect.objectContaining({ models: ['alpha'], input_price: 0.000001451612903 })]
    }))
    wrapper.unmount()
  })

  it('pauses explicitly without applying an unfinished price edit', async () => {
    const wrapper = await editor()
    await wrapper.findAll('input[placeholder="沿用默认"]')[0]!.setValue('9')
    await wrapper.findAll('button').find(button => button.text() === '暂停服务')!.trigger('click')
    await wrapper.findAll('button').find(button => button.text() === '确认')!.trigger('click')
    await flushPromises()
    const request = vi.mocked(channelsAPI.update).mock.calls[0]?.[1]
    expect(request).not.toHaveProperty('model_pricing')
    expect(request?.features_config?.model_services).toEqual([expect.objectContaining({ state: 'paused' })])
    wrapper.unmount()
  })

  it('asks for a scope before edits when more than one group range exists', async () => {
    const wrapper = await editor([channel(), channel(5, 2)])
    expect(wrapper.find('input[placeholder="沿用默认"]').exists()).toBe(false)
    expect(wrapper.get('#service-scope').text()).toContain('普通用户')
    expect(wrapper.get('#service-scope').text()).toContain('VIP')
    await wrapper.get('#service-scope').setValue('5')
    expect(wrapper.find('input[placeholder="沿用默认"]').exists()).toBe(true)
    expect(channelsAPI.update).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('keeps legacy-open models published and merges other models from the fresh server response', async () => {
    const legacy = channel()
    legacy.features_config = { other_feature: true }
    const fresh = channel()
    fresh.model_pricing.push({ ...fresh.model_pricing[0]!, models: ['gamma'], input_price: 0.000015 })
    fresh.features_config = { other_feature: false, model_services: [{ model: 'gamma', platform: 'openai', kind: 'chat', state: 'paused' }] }
    vi.mocked(channelsAPI.getById).mockResolvedValue(fresh)
    const wrapper = await editor([legacy])
    expect(wrapper.text()).toContain('当前状态：已开放')
    expect(wrapper.text()).not.toContain('保存草稿')
    await wrapper.findAll('button').find(button => button.text() === '预览并应用')!.trigger('click')
    expect(channelsAPI.update).not.toHaveBeenCalled()
    await wrapper.findAll('button').find(button => button.text() === '确认')!.trigger('click')
    await flushPromises()
    expect(channelsAPI.update).toHaveBeenCalledWith(4, expect.objectContaining({
      features_config: { other_feature: false, model_services: [expect.objectContaining({ model: 'gamma', state: 'paused' }), expect.objectContaining({ model: 'alpha', state: 'published' })] },
      model_pricing: [expect.objectContaining({ models: ['beta'] }), expect.objectContaining({ models: ['gamma'], input_price: 0.000015 }), expect.objectContaining({ models: ['alpha'] })]
    }))
    wrapper.unmount()
  })

  it('publishes H3-Max with independent specs and prices while preserving H3 configuration', async () => {
    const original = channel()
    const h3 = { model: 'MiniMax-H3', platform: 'openai', kind: 'video', state: 'published', resolutions: ['2K'], min_duration: 4, max_duration: 15, prices: { '2K': 0.13 } }
    original.features_config = { model_services: [h3] }
    vi.mocked(channelsAPI.getById).mockResolvedValue(original)
    const wrapper = mount(ModelServiceEditor, { props: { model: 'MiniMax-H3-Max', platform: 'openai', kind: 'video', channels: [original], accounts: [{ id: 1, name: 'MiniMax', platform: 'openai', status: 'active', schedulable: true, group_ids: [1], configured_models: { 'MiniMax-H3-Max': 'MiniMax-H3-Max' } }] as Account[] }, global: { plugins: [createI18n({ legacy: false, locale: 'zh', messages: { zh: {} } })], stubs: { RouterLink: { template: '<a><slot /></a>' } } } })
    await flushPromises()
    expect(wrapper.find('[aria-label="480P 每秒基础售价"]').exists()).toBe(true)
    expect(wrapper.find('[aria-label="2K 每秒基础售价"]').exists()).toBe(false)
    const durationFields = wrapper.findAll('input[type="number"][max="15"]')
    expect(durationFields.every(field => field.attributes('min') === '5')).toBe(true)
    await durationFields[0]!.setValue(4)
    await wrapper.findAll('button').find(button => button.text() === '预览并应用')!.trigger('click')
    expect(wrapper.get('[role="alert"]').text()).toContain('5–15 秒')
    expect(wrapper.find('[role="alertdialog"]').exists()).toBe(false)
    await durationFields[0]!.setValue(5)
    await wrapper.findAll('button').find(button => button.text() === '预览并应用')!.trigger('click')
    await wrapper.findAll('button').find(button => button.text() === '确认')!.trigger('click')
    await flushPromises()
    expect(channelsAPI.update).toHaveBeenCalledWith(4, { features_config: { model_services: [h3, expect.objectContaining({ model: 'MiniMax-H3-Max', resolutions: ['480P', '768P'], min_duration: 5, prices: { '480P': 0.05, '768P': 0.08 }, state: 'published' })] } })
    wrapper.unmount()
  })

  it('uses centralized billing in simple mode and preserves existing model prices when applying service permissions', async () => {
    vi.mocked(billingManagementAPI.getConfig).mockResolvedValue({ version: 3, simple_billing_enabled: true, api_sale_multiplier: 1.4, usd_to_cny_rate: 6.2, exchange_rate_source: '', exchange_rate_effective_at: null, cost_profiles: [] })
    const original = channel()
    const pricesBefore = structuredClone(original.model_pricing)
    const wrapper = await editor([original])

    expect(wrapper.find('input[placeholder="沿用默认"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('基础售价（人民币 / 秒）')
    expect(wrapper.text()).toContain('费用按实际上游和统一费率计算')
    expect(wrapper.find('a[to="/admin/billing-management?model=alpha"]').exists()).toBe(true)
    await wrapper.findAll('button').find(button => button.text() === '预览并应用')!.trigger('click')
    expect(channelsAPI.update).not.toHaveBeenCalled()
    await wrapper.findAll('button').find(button => button.text() === '确认')!.trigger('click')
    await flushPromises()

    const request = vi.mocked(channelsAPI.update).mock.calls[0]?.[1]
    expect(request).not.toHaveProperty('model_pricing')
    expect(request?.features_config?.model_services).toEqual([expect.objectContaining({ model: 'alpha', state: 'published' })])
    expect(original.model_pricing).toEqual(pricesBefore)
    wrapper.unmount()
  })

  it('previews saved prices in place and restores focus and unfinished image permissions on Escape', async () => {
    const costs = [profile()]
    vi.mocked(billingManagementAPI.getConfig).mockResolvedValue(config({ cost_profiles: costs }))
    const original = channel()
    original.features_config = { model_services: [{ model: 'gpt-image-2', platform: 'openai', kind: 'image', state: 'published', max_images: 2, image_sizes: ['2K'], groups: { 1: { enabled: true, resolutions: [], max_duration: 15, modes: [] } } }] }
    const sources = [{ id: 8, name: '图片 API', platform: 'openai', type: 'apikey', status: 'active', schedulable: true, group_ids: [1], configured_models: { 'gpt-image-2': 'gpt-image-2' } }] as Account[]
    const wrapper = await editor([original], { model: 'gpt-image-2', kind: 'image', accounts: sources })
    await wrapper.get('input[placeholder="2K,4K 或 1024x1024"]').setValue('4K')
    await wrapper.get('input[type="number"][max="15"]').setValue('7')
    await wrapper.findAll('button').find(button => button.text() === '开放分组')!.trigger('click')
    await wrapper.get('section.space-y-4 input[type="checkbox"]').setValue(false)
    expect(wrapper.text()).not.toContain('账号授权倍率')
    const opener = wrapper.findAll('button').find(button => button.text() === '费用试算')!
    ;(opener.element as HTMLButtonElement).focus()
    await opener.trigger('click')
    await flushPromises()

    const quote = wrapper.getComponent(BillingPreviewDialog)
    expect(quote.props()).toMatchObject({ model: 'gpt-image-2', accounts: sources, profiles: costs, initialAccountId: 8, initialGroupId: 1, models: ['gpt-image-2'] })
    expect(quote.text()).toContain('使用已保存的配置计算，不产生模型调用或扣费')
    expect(wrapper.get('[aria-labelledby="service-title"]').element.hasAttribute('inert')).toBe(true)
    expect(wrapper.get('[aria-labelledby="service-title"]').attributes('aria-hidden')).toBe('true')
    expect(quote.get('select[aria-label="调用分组"]').element).toHaveProperty('value', '1')
    expect(quote.findAll('select')[2]!.element).toHaveProperty('value', '8')
    await quote.get('form').trigger('submit')
    await flushPromises()
    expect(billingManagementAPI.preview).toHaveBeenCalledWith(expect.objectContaining({ model: 'gpt-image-2', account_id: 8, group_id: 1, modality: 'image' }))
    expect(billingManagementAPI.getConfig).toHaveBeenCalledTimes(1)
    expect(channelsAPI.update).not.toHaveBeenCalled()

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await flushPromises()
    expect(wrapper.findComponent(BillingPreviewDialog).exists()).toBe(false)
    expect(wrapper.emitted('close')).toBeUndefined()
    expect(document.activeElement).toBe(opener.element)
    expect(wrapper.get('section.space-y-4 input[type="checkbox"]').element).toHaveProperty('checked', false)
    expect(wrapper.get('[aria-labelledby="service-title"]').attributes('aria-hidden')).toBeUndefined()
    expect(wrapper.get('[aria-labelledby="service-title"]').attributes('inert')).toBeUndefined()
    await wrapper.findAll('button').find(button => button.text() === '服务规格')!.trigger('click')
    expect(wrapper.get('input[placeholder="2K,4K 或 1024x1024"]').element).toHaveProperty('value', '4K')
    expect(wrapper.get('input[type="number"][max="15"]').element).toHaveProperty('value', '7')
    wrapper.unmount()
  })

  it('leaves the preview group undecided when a unique source serves multiple groups', async () => {
    vi.mocked(billingManagementAPI.getConfig).mockResolvedValue(config())
    const wrapper = await editor()
    await wrapper.findAll('button').find(button => button.text() === '费用试算')!.trigger('click')
    const quote = wrapper.getComponent(BillingPreviewDialog)
    expect(quote.props('initialAccountId')).toBe(1)
    expect(quote.props('initialGroupId')).toBeUndefined()
    await quote.get('form').trigger('submit')
    expect(quote.get('[role="alert"]').text()).toContain('请选择调用分组')
    expect(billingManagementAPI.preview).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('distinguishes paused scheduling from an enabled usable source and does not preselect a paused source', async () => {
    vi.mocked(billingManagementAPI.getConfig).mockResolvedValue(config())
    const paused = { id: 1, name: '暂停账号', platform: 'openai', status: 'active', schedulable: false, group_ids: [1], configured_models: { alpha: 'alpha' } } as Account
    const wrapper = await editor([channel()], { accounts: [paused] })
    await wrapper.findAll('button').find(button => button.text() === '上游来源')!.trigger('click')
    expect(wrapper.text()).toContain('暂停调度')
    expect(wrapper.text()).not.toContain('账号启用')
    await wrapper.findAll('button').find(button => button.text() === '费用试算')!.trigger('click')
    expect(wrapper.getComponent(BillingPreviewDialog).props('initialAccountId')).toBeUndefined()
    wrapper.getComponent(BillingPreviewDialog).vm.$emit('close')
    await flushPromises()
    await wrapper.setProps({ accounts: [{ ...paused, schedulable: true }, { ...paused, id: 2, name: '停用账号', status: 'disabled' }] })
    expect(wrapper.text()).toContain('可调度')
    expect(wrapper.text()).toContain('未启用')
    wrapper.unmount()
  })
})
