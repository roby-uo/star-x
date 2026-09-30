import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import ImageWorkbench from '../ImageWorkbench.vue'
import { list } from '@/api/keys'
import type { ApiKey } from '@/types'

vi.mock('@/api/keys', () => ({ list: vi.fn(async () => ({ items: [{ id: 1, name: '测试密钥', key: 'test-only', group_id: 2 }] })) }))
vi.mock('@/api/url', () => ({ buildGatewayUrl: (path: string) => path }))
const model = { name: 'doubao-seedream-4-5-251128', platform: 'openai', group_id: 2, group_name: '普通用户', rate_multiplier: 1.4 }

describe('ImageWorkbench', () => {
  const wrappers: VueWrapper[] = []
  afterEach(() => {
    wrappers.splice(0).forEach(wrapper => wrapper.unmount())
    vi.unstubAllGlobals()
    vi.clearAllMocks()
  })
  it('requires paid test consent and resets it when the request changes', async () => {
    const fetchMock = vi.fn(async () => ({ ok: true, json: async () => ({ data: [{ url: 'https://example.com/result.png' }] }) }))
    vi.stubGlobal('fetch', fetchMock)
    const wrapper = mount(ImageWorkbench, { props: { model } })
    wrappers.push(wrapper)
    await flushPromises()
    await wrapper.find('textarea').setValue('landscape')
    await wrapper.find('form').trigger('submit')
    expect(fetchMock).not.toHaveBeenCalled()
    await wrapper.find('input[type="checkbox"]').setValue(true)
    await wrapper.find('textarea').setValue('city')
    await wrapper.find('form').trigger('submit')
    expect(fetchMock).not.toHaveBeenCalled()
    await wrapper.find('input[type="checkbox"]').setValue(true)
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(wrapper.find('img').attributes('src')).toBe('https://example.com/result.png')
    expect(wrapper.text()).not.toContain('用量定价')
    expect(wrapper.find('input[type="checkbox"]').element).toHaveProperty('checked', false)
  })
  it('uses the shared detail key and notifies its busy state in embedded mode', async () => {
    const fetchMock = vi.fn(async (_path: string, _options?: RequestInit) => ({ ok: true, json: async () => ({ data: [{ url: 'javascript:alert(1)' }] }) }))
    vi.stubGlobal('fetch', fetchMock)
    const wrapper = mount(ImageWorkbench, { props: { model, embedded: true, selectedKeyId: 7, contextKeys: [{ id: 7, name: '选中密钥', key: 'outer-key', group_id: 2 } as ApiKey] } })
    wrappers.push(wrapper)
    await flushPromises()
    expect(list).not.toHaveBeenCalled()
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('调用密钥')
    await wrapper.find('textarea').setValue('landscape')
    await wrapper.find('input[type="checkbox"]').setValue(true)
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    const options = fetchMock.mock.calls[0]![1] as RequestInit
    expect(options.headers).toMatchObject({ Authorization: 'Bearer outer-key' })
    expect(JSON.parse(options.body as string)).toMatchObject({ model: model.name, prompt: 'landscape', size: '2K', response_format: 'url' })
    expect(wrapper.find('img').exists()).toBe(false)
    expect(wrapper.emitted('busy')?.some(event => event[0] === true)).toBe(true)
    expect(wrapper.emitted('busy')?.at(-1)?.[0]).toBe(false)
  })
})
