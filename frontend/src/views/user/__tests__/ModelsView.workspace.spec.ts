import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import ModelsView from '../ModelsView.vue'

const mocks = vi.hoisted(() => ({ getModels: vi.fn(), showError: vi.fn() }))
vi.mock('@/api/channels', () => ({ default: { getAvailableModels: mocks.getModels } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: mocks.showError }) }))
const details = {
  name: 'ModelDetails',
  props: ['item', 'initialTab', 'initialGroupId', 'initialKeyId'],
  emits: ['close', 'contextChange'],
  template: '<section data-test="details"><button @click="$emit(\'close\')">关闭详情</button></section>'
}
const models = [
  { name: 'MiniMax-H3', platform: 'openai', model_type: 'video', group_id: 7, group_name: '普通用户' },
  { name: 'MiniMax-H3', platform: 'openai', model_type: 'video', group_id: 10, group_name: 'VIP' },
  { name: 'MiniMax-M3', platform: 'openai', model_type: 'chat', group_id: 7, group_name: '普通用户' }
]
const wrappers: VueWrapper[] = []
beforeEach(() => { mocks.getModels.mockReset(); mocks.showError.mockReset(); mocks.getModels.mockResolvedValue(models) })
afterEach(() => { wrappers.forEach(wrapper => wrapper.unmount()); wrappers.length = 0 })

async function render(path = '/model-catalog') {
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/:pathMatch(.*)*', component: { template: '<div />' } }] })
  await router.push(path)
  const wrapper = mount(ModelsView, { global: { plugins: [router], stubs: {
    AppLayout: { template: '<main><slot /></main>' },
    UserAccessWorkspace: { template: '<nav>模型 密钥</nav>' },
    ModelDetails: details
  } } })
  wrappers.push(wrapper)
  await flushPromises()
  return { wrapper, router }
}

describe('user API access model flow', () => {
  it('returns from key creation to the same model, group, key and test tab', async () => {
    const { wrapper } = await render('/model-catalog?model=MiniMax-H3&platform=openai&kind=Videos&group=10&tab=test&key=42')
    const modal = wrapper.getComponent({ name: 'ModelDetails' })
    expect(modal.props('item').name).toBe('MiniMax-H3')
    expect(modal.props('initialGroupId')).toBe(10)
    expect(modal.props('initialKeyId')).toBe(42)
    expect(modal.props('initialTab')).toBe('test')
  })

  it.each([
    ['model=hidden-model&platform=openai&kind=Videos&group=7', '这个模型当前不可用'],
    ['model=MiniMax-H3&platform=openai&kind=Videos&group=999', '原调用分组已无法使用'],
    ['model=MiniMax-H3&platform=openai&kind=Images&group=7', '这个模型当前不可用']
  ])('does not reopen unavailable context %s', async (query, notice) => {
    const { wrapper } = await render(`/model-catalog?${query}`)
    expect(wrapper.findComponent({ name: 'ModelDetails' }).exists()).toBe(false)
    expect(wrapper.text()).toContain(notice)
  })

  it('opens a card with only non-sensitive connection context', async () => {
    const { wrapper, router } = await render()
    const video = wrapper.findAll('article').find(card => card.text().includes('MiniMax-H3'))!
    await video.findAll('button').find(button => button.text() === 'API 接入')!.trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.query).toEqual({ model: 'MiniMax-H3', platform: 'openai', kind: 'Videos', group: '7', tab: 'api' })
    expect(wrapper.getComponent({ name: 'ModelDetails' }).props('initialTab')).toBe('api')
    expect(wrapper.text()).not.toContain('成本')
    expect(wrapper.text()).not.toContain('倍率')
  })

  it('clears modal query on close so refreshing does not reopen it', async () => {
    const { wrapper, router } = await render('/model-catalog?model=MiniMax-H3&platform=openai&kind=Videos&group=7&tab=test&key=42')
    wrapper.getComponent({ name: 'ModelDetails' }).vm.$emit('close')
    await flushPromises()
    expect(wrapper.findComponent({ name: 'ModelDetails' }).exists()).toBe(false)
    expect(router.currentRoute.value.query).toEqual({})
  })

  it('keeps current selection in the URL without adding a history step', async () => {
    const { wrapper, router } = await render('/model-catalog?model=MiniMax-H3&platform=openai&kind=Videos&group=7&tab=api')
    const replace = vi.spyOn(router, 'replace')
    const modal = wrapper.getComponent({ name: 'ModelDetails' })
    modal.vm.$emit('contextChange', { groupId: 10, keyId: 42, tab: 'test' })
    await flushPromises()
    expect(replace).toHaveBeenCalledOnce()
    expect(router.currentRoute.value.query).toEqual({ model: 'MiniMax-H3', platform: 'openai', kind: 'Videos', group: '10', tab: 'test', key: '42' })
    expect(wrapper.getComponent({ name: 'ModelDetails' }).vm).toBe(modal.vm)
    expect(modal.props('initialGroupId')).toBe(10)
    expect(modal.props('initialKeyId')).toBe(42)
    expect(modal.props('initialTab')).toBe('test')
    modal.vm.$emit('contextChange', { groupId: 10, keyId: 42, tab: 'test' })
    await flushPromises()
    expect(replace).toHaveBeenCalledOnce()
  })

  it('shows a recoverable loading error instead of claiming the user has no models', async () => {
    mocks.getModels.mockRejectedValueOnce(new Error('网络连接失败'))
    const { wrapper } = await render()
    expect(wrapper.get('[role="alert"]').text()).toContain('网络连接失败')
    expect(wrapper.text()).not.toContain('暂无可用模型')
    await wrapper.findAll('button').find(button => button.text() === '重新加载')!.trigger('click')
    await flushPromises()
    expect(wrapper.findAll('article')).toHaveLength(2)
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
  })
})
