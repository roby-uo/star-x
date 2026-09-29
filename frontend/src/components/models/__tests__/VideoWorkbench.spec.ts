import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import VideoWorkbench from '../VideoWorkbench.vue'

vi.mock('@/api/keys', () => ({ list: vi.fn(async () => ({ items: [{ id: 1, name: '测试密钥', key: 'test-only', group_id: 2 }] })) }))
vi.mock('@/api/client', () => ({ apiClient: { get: vi.fn(async () => ({ data: { restrict_models: false, video_resolutions: ['768P'], video_max_duration: 5 } })) } }))
vi.mock('@/api/url', () => ({ buildGatewayUrl: (path: string) => path }))

describe('VideoWorkbench', () => {
  afterEach(() => vi.unstubAllGlobals())
  it('narrows choices using key permissions and invalidates the quote when specifications change', async () => {
    const fetchMock = vi.fn(async (_path: string, _options?: unknown) => ({ ok: true, json: async () => ({ currency: 'USD', unit_price: 0.08, multiplier: 1.4, total: 0.56, version: 'quote-version' }) }))
    vi.stubGlobal('fetch', fetchMock)
    const wrapper = mount(VideoWorkbench, { props: { model: { name: 'MiniMax-H3', platform: 'openai', group_id: 2, group_name: '普通用户', rate_multiplier: 1.4 } } })
    await flushPromises()
    const selects = wrapper.findAll('select')
    expect(selects[2]!.findAll('option').map(o => o.text())).toEqual(['768P'])
    expect(selects[3]!.findAll('option').map(o => o.text())).toEqual(['4', '5'])
    const generate = () => wrapper.findAll('button').find(b => b.text().startsWith('生成视频'))!
    expect(generate().attributes('disabled')).toBeDefined()
    await wrapper.find('textarea').setValue('sunrise')
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(wrapper.text()).toContain('$0.5600')
    expect(generate().attributes('disabled')).toBeUndefined()
    await selects[3]!.setValue('4')
    expect(generate().attributes('disabled')).toBeDefined()
    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(fetchMock.mock.calls[0]![0]).toBe('/v2/video_generation/quote')
    wrapper.unmount()
  })
})
