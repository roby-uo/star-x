import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ModelDetails from '../ModelDetails.vue'
import { aggregateUserModels } from '../userModelCatalog'
import type { ApiKey } from '@/types'
import type { UserCatalogItem } from '../userModelCatalog'

const { listKeys } = vi.hoisted(() => ({ listKeys: vi.fn() }))
vi.mock('@/api/keys', () => ({ list: listKeys }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ apiBaseUrl: '' }) }))

const key = (group_id: number) => ({ id: group_id * 10, group_id, name: `Key ${group_id}`, key: `secret-${group_id}`, status: 'active' }) as ApiKey
const item = aggregateUserModels([1, 2].map(group_id => ({ name: 'MiniMax-H3', platform: 'openai', group_id, group_name: group_id === 1 ? '普通用户' : 'VIP用户', rate_multiplier: group_id, policy: { model: 'MiniMax-H3', platform: 'openai', kind: 'video' as const, state: 'published' as const, resolutions: group_id === 1 ? ['768P'] : ['2K'], min_duration: 4, max_duration: 15, prices: { '768P': 0.08, '2K': 0.13 } } })))[0]
const workbench = { name: 'VideoWorkbench', props: ['model', 'selectedKeyId', 'contextKeys', 'embedded'], emits: ['busy'], template: '<div data-test="workbench">Current test</div>' }
const linkStub = { name: 'RouterLink', props: ['to'], template: '<a><slot /></a>' }
const mountDetails = (extraProps: { item?: UserCatalogItem; initialTab?: 'api' | 'test'; initialGroupId?: number; initialKeyId?: number } = {}) => mount(ModelDetails, { props: { item, initialTab: 'test', ...extraProps }, global: { stubs: { RouterLink: linkStub, VideoWorkbench: workbench } } })

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
    expect(wrapper.text()).toContain('视频生成 · API / 工具接入')
    expect(wrapper.text()).toContain('MiniMax 视频接口 · 异步任务')
    expect(wrapper.text()).not.toContain('接口 · OpenAI 兼容')
    expect(wrapper.get('[data-testid="model-client-purpose"]').text()).toContain('不能填入 Codex、OpenCode、WorkBuddy')
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
    expect(wrapper.findComponent({ name: 'RouterLink' }).exists()).toBe(false)
    expect(wrapper.text()).toContain('管理密钥')
    wrapper.unmount()
  })

  it('restores only an available group and active key from the same group', async () => {
    listKeys.mockResolvedValue({ items: [key(1), key(2), { ...key(2), id: 21 }, { ...key(2), id: 22, status: 'inactive' }], pages: 1 })
    const wrapper = mountDetails({ initialGroupId: 2, initialKeyId: 21 })
    await flushPromises()
    expect((wrapper.findAll('select')[0].element as HTMLSelectElement).value).toBe('2')
    expect(wrapper.findComponent({ name: 'VideoWorkbench' }).props('selectedKeyId')).toBe(21)
    expect(wrapper.findComponent({ name: 'VideoWorkbench' }).props('contextKeys').map((entry: ApiKey) => entry.id)).toEqual([20, 21])
    expect(wrapper.findComponent({ name: 'RouterLink' }).props('to')).toEqual({ path: '/keys', query: { model: 'MiniMax-H3', platform: 'openai', kind: 'Videos', tab: 'test', group: '2', key: '21' } })
    wrapper.unmount()
  })

  it('falls back safely when the requested group or key is unavailable', async () => {
    const wrapper = mountDetails({ initialGroupId: 99, initialKeyId: 20 })
    await flushPromises()
    expect(listKeys).toHaveBeenCalledWith(1, 100, { group_id: 1, status: 'active' })
    expect(wrapper.findComponent({ name: 'VideoWorkbench' }).props('selectedKeyId')).toBe(10)
    expect(wrapper.get('pre').text()).not.toContain('secret-1')
    wrapper.unmount()
  })

  it('takes a user with no key to creation while preserving the model, group and selected tab', async () => {
    listKeys.mockResolvedValue({ items: [], pages: 1 })
    const wrapper = mountDetails({ initialGroupId: 2 })
    await flushPromises()
    expect(wrapper.findComponent({ name: 'RouterLink' }).text()).toBe('创建密钥')
    expect(wrapper.findComponent({ name: 'RouterLink' }).props('to')).toEqual({ path: '/keys', query: { model: 'MiniMax-H3', platform: 'openai', kind: 'Videos', tab: 'test', group: '2', create: '1' } })
    await wrapper.findAll('button').find(button => button.text() === 'API 接入')!.trigger('click')
    expect(wrapper.findComponent({ name: 'RouterLink' }).props('to').query.tab).toBe('api')
    expect(wrapper.findComponent({ name: 'VideoWorkbench' }).exists()).toBe(false)
    wrapper.unmount()
  })

  it('retries key loading in place while preserving the chosen group and detail tab', async () => {
    listKeys.mockRejectedValueOnce(new Error('network')).mockResolvedValueOnce({ items: [key(2)], pages: 1 })
    const wrapper = mountDetails({ initialGroupId: 2, initialKeyId: 20 })
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('调用密钥加载失败')
    expect(wrapper.findComponent({ name: 'RouterLink' }).exists()).toBe(false)
    await wrapper.findAll('button').find(button => button.text() === '重新加载')!.trigger('click')
    await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    expect(wrapper.findComponent({ name: 'VideoWorkbench' }).props('selectedKeyId')).toBe(20)
    expect(wrapper.findComponent({ name: 'RouterLink' }).props('to').query).toMatchObject({ group: '2', tab: 'test' })
    wrapper.unmount()
  })

  it('emits valid selections when the user changes group, key or tab without any secret', async () => {
    listKeys.mockImplementation(async (_page, _size, filter) => ({ items: [key(filter.group_id), { ...key(filter.group_id), id: filter.group_id * 10 + 1 }], pages: 1 }))
    const wrapper = mountDetails()
    await flushPromises()
    expect(wrapper.emitted('contextChange')?.at(-1)).toEqual([{ groupId: 1, keyId: 10, tab: 'test' }])
    await wrapper.findAll('select')[0].setValue(2)
    await flushPromises()
    expect(wrapper.emitted('contextChange')?.at(-1)).toEqual([{ groupId: 2, keyId: 20, tab: 'test' }])
    await wrapper.findAll('select')[1].setValue(21)
    expect(wrapper.emitted('contextChange')?.at(-1)).toEqual([{ groupId: 2, keyId: 21, tab: 'test' }])
    await wrapper.findAll('button').find(button => button.text() === 'API 接入')!.trigger('click')
    expect(wrapper.emitted('contextChange')?.at(-1)).toEqual([{ groupId: 2, keyId: 21, tab: 'api' }])
    expect(JSON.stringify(wrapper.emitted('contextChange'))).not.toContain('secret-')
    wrapper.unmount()
  })

  it('restores changed external context on the same model and avoids redundant key requests or test resets', async () => {
    listKeys.mockImplementation(async (_page, _size, filter) => ({ items: [key(filter.group_id), { ...key(filter.group_id), id: filter.group_id * 10 + 1 }], pages: 1 }))
    const wrapper = mountDetails()
    await flushPromises()
    await wrapper.setProps({ initialGroupId: 2, initialKeyId: 21, initialTab: 'api' })
    await flushPromises()
    expect(wrapper.findComponent({ name: 'VideoWorkbench' }).props('selectedKeyId')).toBe(21)
    expect(wrapper.findComponent({ name: 'VideoWorkbench' }).props('model').group_id).toBe(2)
    const existingTest = wrapper.findComponent({ name: 'VideoWorkbench' }).vm
    const calls = listKeys.mock.calls.length
    await wrapper.setProps({ initialTab: 'test' })
    await flushPromises()
    expect(wrapper.findComponent({ name: 'VideoWorkbench' }).vm).toBe(existingTest)
    expect(listKeys).toHaveBeenCalledTimes(calls)
    await wrapper.setProps({ initialKeyId: 20 })
    await flushPromises()
    expect(wrapper.findComponent({ name: 'VideoWorkbench' }).props('selectedKeyId')).toBe(20)
    expect(listKeys).toHaveBeenCalledTimes(calls)
    const currentTest = wrapper.findComponent({ name: 'VideoWorkbench' }).vm
    await wrapper.setProps({ initialGroupId: 2, initialKeyId: 20, initialTab: 'test' })
    await flushPromises()
    expect(wrapper.findComponent({ name: 'VideoWorkbench' }).vm).toBe(currentTest)
    expect(listKeys).toHaveBeenCalledTimes(calls)
    wrapper.unmount()
  })

  it('holds external context changes until the running request finishes', async () => {
    const wrapper = mountDetails()
    await flushPromises()
    const runningTest = wrapper.findComponent({ name: 'VideoWorkbench' }).vm
    runningTest.$emit('busy', true)
    await wrapper.vm.$nextTick()
    await wrapper.setProps({ initialGroupId: 2, initialKeyId: 20, initialTab: 'api' })
    await flushPromises()
    expect(wrapper.findComponent({ name: 'VideoWorkbench' }).vm).toBe(runningTest)
    expect(wrapper.findComponent({ name: 'VideoWorkbench' }).props('selectedKeyId')).toBe(10)
    expect(listKeys).toHaveBeenCalledTimes(1)
    expect(wrapper.findAll('button').find(button => button.text() === '在线测试')?.attributes('aria-pressed')).toBe('true')
    runningTest.$emit('busy', false)
    await flushPromises()
    expect(wrapper.findComponent({ name: 'VideoWorkbench' }).props('selectedKeyId')).toBe(20)
    expect(wrapper.findAll('button').find(button => button.text() === 'API 接入')?.attributes('aria-pressed')).toBe('true')
    expect(listKeys).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })

  it('uses the latest external key selection when the group lookup is still pending', async () => {
    let resolveKeys: (value: { items: ApiKey[]; pages: number }) => void = () => {}
    listKeys.mockImplementation(() => new Promise(resolve => { resolveKeys = resolve }))
    const wrapper = mountDetails({ initialGroupId: 2, initialKeyId: 20 })
    await wrapper.setProps({ initialKeyId: 21 })
    resolveKeys({ items: [key(2), { ...key(2), id: 21 }], pages: 1 })
    await flushPromises()
    expect(wrapper.findComponent({ name: 'VideoWorkbench' }).props('selectedKeyId')).toBe(21)
    expect(listKeys).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })

  it('omits expired or exhausted active keys and accepts a future unlimited key', async () => {
    listKeys.mockResolvedValue({ items: [
      { ...key(1), id: 11, expires_at: new Date(Date.now() - 1000).toISOString() },
      { ...key(1), id: 12, quota: 10, quota_used: 10 },
      { ...key(1), id: 13, quota: 10, quota_used: 11 },
      { ...key(1), id: 14, expires_at: new Date(Date.now() + 60_000).toISOString(), quota: 0, quota_used: 50 },
      { ...key(2), id: 21 }
    ], pages: 1 })
    const wrapper = mountDetails({ initialGroupId: 1, initialKeyId: 11 })
    await flushPromises()
    expect(wrapper.findComponent({ name: 'VideoWorkbench' }).props('selectedKeyId')).toBe(14)
    expect(wrapper.findComponent({ name: 'VideoWorkbench' }).props('contextKeys').map((entry: ApiKey) => entry.id)).toEqual([14])
    await wrapper.setProps({ initialKeyId: 12 })
    await flushPromises()
    expect(wrapper.findComponent({ name: 'VideoWorkbench' }).props('selectedKeyId')).toBe(14)
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

  it('offers H3-Max API generation and testing with its own minimum specification', async () => {
    const maxItem = aggregateUserModels([{ name: 'MiniMax-H3-Max', platform: 'openai', group_id: 1, group_name: '普通用户', rate_multiplier: 1 }])[0]
    const wrapper = mount(ModelDetails, { props: { item: maxItem, initialTab: 'test' }, global: { stubs: { RouterLink: { template: '<a><slot /></a>' }, VideoWorkbench: workbench } } })
    await flushPromises()
    expect(wrapper.findComponent({ name: 'VideoWorkbench' }).props('model').name).toBe('MiniMax-H3-Max')
    const example = wrapper.get('pre').text()
    expect(example).toContain('/v2/video_generation/quote')
    expect(example).toContain('"resolution": "480P"')
    expect(example).toContain('"duration": 5')
    expect(example).not.toContain('"duration": 4')
    expect(wrapper.text()).not.toContain('价格与限制')
    wrapper.unmount()
  })

  it('does not offer calls or generation tests when catalog metadata says there is no adapter', async () => {
    const unsupported = aggregateUserModels([{ name: 'custom-picture', platform: 'openai', group_id: 1, group_name: '普通用户', model_type: 'image', supported_endpoints: [] }])[0]
    const wrapper = mount(ModelDetails, { props: { item: unsupported, initialTab: 'test' }, global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } } })
    await flushPromises()
    expect(wrapper.text()).toContain('暂无调用适配')
    expect(wrapper.find('pre').exists()).toBe(false)
    expect(wrapper.findComponent({ name: 'ImageWorkbench' }).exists()).toBe(false)
    wrapper.unmount()
  })

  it('clarifies text model protocol and image-input limitations without promising client compatibility', async () => {
    const textItem = aggregateUserModels([{ name: 'video-understanding-vl', platform: 'openai', group_id: 1, group_name: '普通用户', model_type: 'chat', supported_endpoints: ['/v1/chat/completions'] }])[0]
    const wrapper = mount(ModelDetails, { props: { item: textItem }, global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } } })
    await flushPromises()
    expect(wrapper.text()).toContain('文本推理 · Agent 主模型候选')
    expect(wrapper.get('[data-testid="model-client-purpose"]').text()).toContain('协议和工具调用能力')
    expect(wrapper.get('[data-testid="model-client-purpose"]').text()).toContain('支持图片输入表示能理解图片，不代表能生成图片')
    expect(wrapper.get('pre').text()).toContain('/v1/chat/completions')
    wrapper.unmount()
  })

  it('keeps unknown text models callable when an empty endpoint list means unverified', async () => {
    const textItem = aggregateUserModels([{ name: 'MiniMax-M3', platform: 'openai', group_id: 1, group_name: '普通用户', model_type: 'unknown', supported_endpoints: [] }])[0]
    const wrapper = mount(ModelDetails, { props: { item: textItem, initialTab: 'test' }, global: { stubs: { RouterLink: { template: '<a><slot /></a>' }, TextModelTester: { name: 'TextModelTester', template: '<div>Text test</div>' } } } })
    await flushPromises()
    expect(wrapper.get('pre').text()).toContain('/v1/chat/completions')
    expect(wrapper.findComponent({ name: 'TextModelTester' }).exists()).toBe(true)
    expect(wrapper.text()).not.toContain('暂未适配')
    wrapper.unmount()
  })
})
