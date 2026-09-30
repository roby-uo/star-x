import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ModelDetails from '../ModelDetails.vue'
import { aggregateUserModels } from '../userModelCatalog'
import type { ApiKey } from '@/types'

const { listKeys } = vi.hoisted(() => ({ listKeys: vi.fn() }))
vi.mock('@/api/keys', () => ({ list: listKeys }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ apiBaseUrl: '' }) }))

const key = (group_id: number) => ({ id: group_id * 10, group_id, name: `Key ${group_id}`, key: `secret-${group_id}`, status: 'active' }) as ApiKey
const item = aggregateUserModels([1, 2].map(group_id => ({ name: 'MiniMax-H3', platform: 'openai', group_id, group_name: group_id === 1 ? '普通用户' : 'VIP用户', rate_multiplier: group_id, policy: { model: 'MiniMax-H3', platform: 'openai', kind: 'video' as const, state: 'published' as const, resolutions: group_id === 1 ? ['768P'] : ['2K'], min_duration: 4, max_duration: 15, prices: { '768P': 0.08, '2K': 0.13 } } })))[0]
const workbench = { name: 'VideoWorkbench', props: ['model', 'selectedKeyId', 'contextKeys', 'embedded'], emits: ['busy'], template: '<div data-test="workbench">Current test</div>' }
const mountDetails = () => mount(ModelDetails, { props: { item, initialTab: 'test' }, global: { stubs: { RouterLink: { template: '<a><slot /></a>' }, VideoWorkbench: workbench } } })

beforeEach(() => {
  listKeys.mockReset()
  listKeys.mockImplementation(async (_page, _size, filter) => ({ items: [key(filter.group_id)], pages: 1 }))
})

describe('model connection details', () => {
  it('keeps the selected group policy and its key together, without exposing secrets or price panels', async () => {
    const wrapper = mountDetails()
    await flushPromises()
    const current = wrapper.findComponent({ name: 'VideoWorkbench' })
    expect(current.props('selectedKeyId')).toBe(10)
    expect(current.props('model').policy.resolutions).toEqual(['768P'])
    expect(wrapper.text()).not.toContain('secret-1')
    expect(wrapper.text()).not.toContain('价格与限制')
    expect(wrapper.text()).not.toContain('我的视频任务')
    await wrapper.findAll('select')[0].setValue(2)
    await flushPromises()
    expect(wrapper.findComponent({ name: 'VideoWorkbench' }).props('selectedKeyId')).toBe(20)
    expect(wrapper.findComponent({ name: 'VideoWorkbench' }).props('model').policy.resolutions).toEqual(['2K'])
    wrapper.unmount()
  })

  it('preserves the current test when viewing API examples and locks context during requests', async () => {
    const wrapper = mountDetails()
    await flushPromises()
    const current = wrapper.findComponent({ name: 'VideoWorkbench' }).vm
    await wrapper.findAll('button').find(button => button.text() === 'API 接入')!.trigger('click')
    expect(wrapper.findComponent({ name: 'VideoWorkbench' }).vm).toBe(current)
    wrapper.findComponent({ name: 'VideoWorkbench' }).vm.$emit('busy', true)
    await wrapper.vm.$nextTick()
    expect(wrapper.findAll('select').every(select => select.attributes('disabled') !== undefined)).toBe(true)
    expect(wrapper.find('button[aria-label="关闭模型详情"]').attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })

  it('ignores a stale key lookup after the user changes groups', async () => {
    let resolveFirst: (value: { items: ApiKey[]; pages: number }) => void = () => {}
    listKeys.mockImplementation((_page, _size, filter) => filter.group_id === 1 ? new Promise(resolve => { resolveFirst = resolve }) : Promise.resolve({ items: [key(2)], pages: 1 }))
    const wrapper = mountDetails()
    await wrapper.findAll('select')[0].setValue(2)
    await flushPromises()
    resolveFirst({ items: [key(1)], pages: 1 })
    await flushPromises()
    expect(wrapper.findComponent({ name: 'VideoWorkbench' }).props('selectedKeyId')).toBe(20)
    wrapper.unmount()
  })
})
