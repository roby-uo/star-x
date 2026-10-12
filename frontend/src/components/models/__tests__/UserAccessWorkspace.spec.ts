import { afterEach, describe, expect, it } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { createMemoryHistory, createRouter } from 'vue-router'
import UserAccessWorkspace from '../UserAccessWorkspace.vue'
import zh from '@/i18n/locales/zh/common'
import en from '@/i18n/locales/en/common'

const wrappers: VueWrapper[] = []
afterEach(() => {
  wrappers.forEach(wrapper => wrapper.unmount())
  wrappers.length = 0
})

async function renderWorkspace(path = '/model-catalog', locale = 'zh') {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/:pathMatch(.*)*', component: { template: '<div />' } }]
  })
  await router.push(path)
  const messages = Object.fromEntries(Object.entries({ zh, en }).map(([language, source]) => [
    language,
    { nav: Object.fromEntries(Object.entries(source.nav).map(([key, value]) => [key, () => value])) }
  ]))
  const wrapper = mount(UserAccessWorkspace, {
    global: { plugins: [router, createI18n({ legacy: false, locale, messages })] }
  })
  wrappers.push(wrapper)
  return { wrapper, router }
}

describe('user API access workspace', () => {
  it('switches between models and keys using the existing routes', async () => {
    const { wrapper, router } = await renderWorkspace()
    expect(wrapper.findAll('a').map(link => link.text())).toEqual(['模型', '密钥'])
    expect(wrapper.find('[aria-current="page"]').text()).toBe('模型')
    expect(wrapper.find('h1').exists()).toBe(false)
    await wrapper.find('a[href="/keys"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/keys')
    expect(wrapper.find('[aria-current="page"]').text()).toBe('密钥')
    await wrapper.find('a[href="/model-catalog"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/model-catalog')
  })

  it('keeps model testing in the models area without a third navigation tab', async () => {
    const { wrapper } = await renderWorkspace('/model-test?model=MiniMax-H3')
    expect(wrapper.find('[aria-current="page"]').text()).toBe('模型')
    expect(wrapper.findAll('a')).toHaveLength(2)
  })

  it('marks a direct keys link as the selected page and renders English labels', async () => {
    const { wrapper } = await renderWorkspace('/keys?group_id=7', 'en')
    expect(wrapper.find('nav').attributes('aria-label')).toBe('API Access')
    expect(wrapper.findAll('a').map(link => link.text())).toEqual(['Models', 'Keys'])
    expect(wrapper.find('[aria-current="page"]').text()).toBe('Keys')
  })
})
