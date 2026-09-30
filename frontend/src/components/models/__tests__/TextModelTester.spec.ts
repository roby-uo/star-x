import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import TextModelTester from '../TextModelTester.vue'

afterEach(() => vi.unstubAllGlobals())

describe('text API test', () => {
  it('requires an explicit submission and uses the selected key and native endpoint', async () => {
    const fetchMock = vi.fn().mockResolvedValue({ ok: true, json: async () => ({ content: [{ type: 'text', text: '你好' }] }) })
    vi.stubGlobal('fetch', fetchMock)
    const wrapper = mount(TextModelTester, { props: { model: { name: 'claude-test', platform: 'anthropic', group_id: 1, group_name: '普通用户', rate_multiplier: 1 }, apiKey: 'selected-secret' } })
    expect(fetchMock).not.toHaveBeenCalled()
    await wrapper.find('form').trigger('submit')
    expect(fetchMock).not.toHaveBeenCalled()
    await wrapper.find('input[type="checkbox"]').setValue(true)
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(fetchMock.mock.calls[0][0]).toMatch(/\/v1\/messages$/)
    expect(fetchMock.mock.calls[0][1].headers.Authorization).toBe('Bearer selected-secret')
    expect(wrapper.text()).toContain('你好')
    expect(wrapper.text()).not.toContain('selected-secret')
    expect(wrapper.emitted('busy')).toEqual([[true], [false]])
    wrapper.unmount()
  })
})
