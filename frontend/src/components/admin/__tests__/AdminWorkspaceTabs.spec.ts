import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import { createMemoryHistory, createRouter } from 'vue-router'
import { reactive } from 'vue'
import AdminWorkspaceTabs from '../AdminWorkspaceTabs.vue'
import { useOnboardingStore } from '@/stores/onboarding'
import { getAdminSteps } from '@/components/Guide/steps'
import zh from '@/i18n/locales/zh/common'

const mode = vi.hoisted(() => ({ auth: { isSimpleMode: false } }))
vi.mock('@/stores/auth', async () => {
  const { reactive } = await import('vue')
  return { useAuthStore: () => reactive(mode.auth) }
})
const authState = reactive(mode.auth)
const wrappers: VueWrapper[] = []
const messages = { nav: Object.fromEntries(Object.entries(zh.nav).map(([key, value]) => [key, () => value])) }

afterEach(() => {
  wrappers.forEach(wrapper => wrapper.unmount())
  wrappers.length = 0
  authState.isSimpleMode = false
  document.body.innerHTML = ''
})

async function renderWorkspace(path = '/admin/models') {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/:pathMatch(.*)*', component: { template: '<div />' } }]
  })
  await router.push(path)
  const pinia = createPinia()
  const wrapper = mount(AdminWorkspaceTabs, {
    props: { area: 'billing' },
    attachTo: document.body,
    global: {
      plugins: [router, pinia, createI18n({ legacy: false, locale: 'zh', messages: { zh: messages } })]
    }
  })
  wrappers.push(wrapper)
  return { wrapper, router, onboarding: useOnboardingStore(pinia) }
}

describe('model management workspace navigation', () => {
  it('keeps models and upstreams visible while secondary settings start closed', async () => {
    const { wrapper, router } = await renderWorkspace()
    expect(wrapper.findAll('a').map(link => link.text())).toEqual(['模型', '上游'])
    expect(wrapper.find('[aria-current="page"]').text()).toBe('模型')
    expect(wrapper.find('#model-management-settings-trigger').attributes('aria-expanded')).toBe('false')
    await wrapper.find('a[href="/admin/accounts"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/admin/accounts')
    expect(wrapper.find('[aria-current="page"]').text()).toBe('上游')
  })

  it('opens billing and group settings without turning them into main tabs', async () => {
    const { wrapper, router } = await renderWorkspace('/admin/billing-management')
    expect(wrapper.find('[aria-label="当前管理页面"]').text().replace(/\s/g, '')).toBe('管理设置/计费设置')
    expect(wrapper.findAll('a').map(link => link.text())).toEqual(['模型', '上游'])
    await wrapper.find('#model-management-settings-trigger').trigger('click')
    expect(wrapper.find('a[href="/admin/billing-management"]').attributes('aria-current')).toBe('page')
    expect(wrapper.text()).toContain('高级设置')
    await wrapper.find('a[href="/admin/groups"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/admin/groups')
    expect(wrapper.find('[aria-label="当前管理页面"]').text().replace(/\s/g, '')).toBe('管理设置/用户分组')
    expect(wrapper.find('#model-management-settings-menu').exists()).toBe(false)
  })

  it('closes the settings popup on Escape, an outside click, or navigation', async () => {
    const { wrapper, router } = await renderWorkspace()
    const trigger = wrapper.find('#model-management-settings-trigger')
    await trigger.trigger('click')
    await wrapper.find('#model-management-settings-menu').trigger('keydown', { key: 'Escape' })
    expect(wrapper.find('#model-management-settings-menu').exists()).toBe(false)
    expect(document.activeElement).toBe(trigger.element)
    await trigger.trigger('click')
    document.body.click()
    await flushPromises()
    expect(wrapper.find('#model-management-settings-menu').exists()).toBe(false)
    await trigger.trigger('click')
    await router.push('/admin/channels/monitor')
    await flushPromises()
    expect(wrapper.find('#model-management-settings-menu').exists()).toBe(false)
    expect(wrapper.find('[aria-label="当前管理页面"]').text().replace(/\s/g, '')).toBe('管理设置/连接监控')
  })

  it('opens the visible settings trigger before the guided group link', async () => {
    const { wrapper, onboarding, router } = await renderWorkspace()
    let currentStep = '#model-management-settings-trigger'
    const nextStep = vi.fn(async () => { currentStep = '#upstream-pools-tab' })
    onboarding.setControlMethods({ isCurrentStep: selector => selector === currentStep, nextStep })
    expect(wrapper.find('#upstream-pools-tab').exists()).toBe(false)
    await wrapper.find(currentStep).trigger('click')
    expect(wrapper.find(currentStep).isVisible()).toBe(true)
    expect(nextStep).toHaveBeenCalledOnce()
    await wrapper.find(currentStep).trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/admin/groups')
    expect(nextStep).toHaveBeenCalledTimes(2)
  })

  it('keeps the model workspace entry before the upstream step in simple mode', () => {
    const steps = getAdminSteps(key => key, true)
    const targets = steps.map(step => step.element).filter(Boolean)
    expect(targets[0]).toBe('#sidebar-model-manage')
    expect(targets[1]).toBe('#sidebar-channel-manage')
    expect(targets).not.toContain('#model-management-settings-trigger')
    expect(targets).not.toContain('#upstream-pools-tab')
  })

  it('hides restricted user groups in simple mode and keeps accessible settings usable', async () => {
    authState.isSimpleMode = true
    const { wrapper, router } = await renderWorkspace()
    await wrapper.find('#model-management-settings-trigger').trigger('click')
    expect(wrapper.find('a[href="/admin/groups"]').exists()).toBe(false)
    expect(wrapper.find('a[href="/admin/billing-management"]').exists()).toBe(true)
    await wrapper.find('a[href="/admin/billing-management"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/admin/billing-management')
  })

  it('updates group availability when the operating mode changes', async () => {
    const { wrapper } = await renderWorkspace()
    await wrapper.find('#model-management-settings-trigger').trigger('click')
    expect(wrapper.find('a[href="/admin/groups"]').exists()).toBe(true)
    authState.isSimpleMode = true
    await flushPromises()
    expect(wrapper.find('a[href="/admin/groups"]').exists()).toBe(false)
    authState.isSimpleMode = false
    await flushPromises()
    expect(wrapper.find('a[href="/admin/groups"]').exists()).toBe(true)
  })
})
