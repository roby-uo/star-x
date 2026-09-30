import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import type { Account, AdminGroup } from '@/types'
import type { Channel } from '@/api/admin/channels'
import channelsAPI from '@/api/admin/channels'
import { getAllIncludingInactive } from '@/api/admin/groups'
import ModelServiceEditor from '../ModelServiceEditor.vue'

vi.mock('@/api/admin/channels', () => ({ default: { getById: vi.fn(), update: vi.fn(), create: vi.fn() } }))
vi.mock('@/api/admin/groups', () => ({ getAllIncludingInactive: vi.fn() }))

const channel = (id = 4, groupID = 1): Channel => ({
  id, name: `范围 ${id}`, status: 'active', group_ids: [groupID], model_mapping: {}, restrict_models: false,
  model_pricing: [{ models: ['alpha', 'beta'], platform: 'openai', billing_mode: 'token', input_price: 0.000001,
    output_price: 0.000002, cache_read_price: null, cache_write_price: null, image_input_price: null,
    image_output_price: null, per_request_price: null, intervals: [] }],
  features_config: { other_feature: true, model_services: [{ model: 'alpha', platform: 'openai', kind: 'chat', state: 'published', groups: { [groupID]: { enabled: true, resolutions: [], max_duration: 15, modes: [] } } }] }
} as Channel)

async function editor(channels = [channel()]) {
  const wrapper = mount(ModelServiceEditor, { props: { model: 'alpha', platform: 'openai', kind: 'chat', channels,
    accounts: [{ id: 1, name: '测试账号', platform: 'openai', status: 'active', schedulable: true, group_ids: [1, 2], configured_models: { alpha: 'alpha' } }] as Account[] },
    global: { plugins: [createI18n({ legacy: false, locale: 'zh', messages: { zh: {} } })], stubs: { RouterLink: { template: '<a><slot /></a>' } } } })
  await flushPromises()
  return wrapper
}

describe('model service publishing flow', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(getAllIncludingInactive).mockResolvedValue([{ id: 1, name: '普通用户', platform: 'openai', status: 'active', rate_multiplier: 1.4, allow_image_generation: true },
      { id: 2, name: 'VIP', platform: 'openai', status: 'active', rate_multiplier: 1, allow_image_generation: true }] as AdminGroup[])
    vi.mocked(channelsAPI.getById).mockResolvedValue(channel())
    vi.mocked(channelsAPI.update).mockResolvedValue(channel())
  })

  it('requires preview confirmation and keeps published state while splitting the edited price', async () => {
    const wrapper = await editor()
    expect(wrapper.text()).not.toContain('保存草稿')
    await wrapper.findAll('input[placeholder="沿用默认"]')[0]!.setValue('9')
    await wrapper.findAll('button').find(button => button.text() === '预览并应用')!.trigger('click')
    expect(channelsAPI.update).not.toHaveBeenCalled()
    expect(wrapper.get('[role="alertdialog"]').text()).toContain('普通用户')
    await wrapper.findAll('button').find(button => button.text() === '确认')!.trigger('click')
    await flushPromises()
    expect(channelsAPI.update).toHaveBeenCalledWith(4, expect.objectContaining({
      features_config: expect.objectContaining({ other_feature: true, model_services: [expect.objectContaining({ state: 'published' })] }),
      model_pricing: [expect.objectContaining({ models: ['beta'], input_price: 0.000001 }), expect.objectContaining({ models: ['alpha'], input_price: 0.000009 })]
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
})
