import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import ModelTestView from '@/views/user/ModelTestView.vue'

vi.mock('@/components/layout/AppLayout.vue', () => ({
  default: { template: '<div><slot /></div>' }
}))
vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => ({
    'nav.userApiAccess': 'API 接入', 'nav.userAccessModels': '模型', 'nav.userAccessKeys': '密钥'
  } as Record<string, string>)[key] || key })
}))

const wrappers: VueWrapper[] = []
afterEach(() => {
  wrappers.forEach(wrapper => wrapper.unmount())
  wrappers.length = 0
  vi.unstubAllGlobals()
})

describe('existing model test route in user access workspace', () => {
  it('keeps the manual test form while navigation alone makes no model request', async () => {
    const request = vi.fn()
    vi.stubGlobal('fetch', request)
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/:pathMatch(.*)*', component: { template: '<div />' } }]
    })
    await router.push('/model-test?model=MiniMax-M3&endpoint=Responses')
    const wrapper = mount(ModelTestView, { global: { plugins: [router], stubs: { Icon: true } } })
    wrappers.push(wrapper)
    expect(wrapper.find('[aria-current="page"]').text()).toBe('模型')
    expect(wrapper.find('input[type="password"]').exists()).toBe(true)
    expect(wrapper.find('h1').exists()).toBe(false)
    await wrapper.find('a[href="/keys"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/keys')
    expect(request).not.toHaveBeenCalled()
  })
})
