import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { createMemoryHistory, createRouter } from 'vue-router'
import { reactive, ref } from 'vue'
import AppSidebar from '../AppSidebar.vue'
import zh from '@/i18n/locales/zh/common'

const mocks = vi.hoisted(() => ({
  app: {
    sidebarCollapsed: false, mobileOpen: false, siteName: 'star-X', siteLogo: '',
    siteVersion: 'test', publicSettingsLoaded: true, sidebarScrollTop: 0,
    cachedPublicSettings: {}, backendModeEnabled: false,
    toggleSidebar: vi.fn(), setMobileOpen: vi.fn()
  },
  auth: { isAdmin: false, isSimpleMode: false },
  adminSettings: { opsMonitoringEnabled: false, paymentEnabled: false, customMenuItems: [], fetch: vi.fn() },
  onboarding: { isCurrentStep: vi.fn((selector: string) => selector === 'no-active-step'), nextStep: vi.fn() }
}))

vi.mock('@/stores', async () => {
  const { reactive } = await import('vue')
  return {
    useAppStore: () => reactive(mocks.app),
    useAuthStore: () => reactive(mocks.auth),
    useAdminSettingsStore: () => reactive(mocks.adminSettings),
    useOnboardingStore: () => mocks.onboarding
  }
})
vi.mock('@/utils/featureFlags', () => ({
  FeatureFlags: { availableChannels: {}, affiliate: {}, riskControl: {} },
  makeSidebarFlag: () => () => false
}))
vi.mock('@/composables/useBatchImageAccess', () => ({
  useBatchImageAccess: () => ({ canUseBatchImage: ref(false), refreshBatchImageAccess: vi.fn() })
}))

const app = reactive(mocks.app)
const auth = reactive(mocks.auth)
const wrappers: VueWrapper[] = []
const messages = { nav: Object.fromEntries(Object.entries(zh.nav).map(([key, value]) => [key, () => value])) }

beforeEach(() => {
  app.sidebarCollapsed = false
  app.backendModeEnabled = false
  auth.isAdmin = false
  auth.isSimpleMode = false
  mocks.onboarding.isCurrentStep.mockReturnValue(false)
  mocks.onboarding.nextStep.mockClear()
})
afterEach(() => {
  wrappers.forEach(wrapper => wrapper.unmount())
  wrappers.length = 0
})

async function renderSidebar(path = '/model-catalog') {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/:pathMatch(.*)*', component: { template: '<div />' } }]
  })
  await router.push(path)
  const wrapper = mount(AppSidebar, {
    global: {
      plugins: [router, createI18n({ legacy: false, locale: 'zh', messages: { zh: messages } })],
      stubs: { VersionBadge: true }
    }
  })
  wrappers.push(wrapper)
  await flushPromises()
  return { wrapper, router }
}

describe('single user API access sidebar entry', () => {
  it.each([
    { admin: false, simple: false }, { admin: true, simple: false },
    { admin: false, simple: true }, { admin: true, simple: true }
  ])('provides the same entry for admin=$admin simple=$simple', async ({ admin, simple }) => {
    auth.isAdmin = admin
    auth.isSimpleMode = simple
    const { wrapper, router } = await renderSidebar('/keys')
    const entries = wrapper.findAll('a[href="/model-catalog"]')
    expect(entries).toHaveLength(1)
    expect(entries[0].text()).toBe('API 接入')
    expect(entries[0].classes()).toContain('sidebar-link-active')
    expect(wrapper.find('a[href="/keys"]').exists()).toBe(false)
    await entries[0].trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/model-catalog')
  })

  it.each(['/model-catalog', '/model-test', '/keys'])('marks API access active for %s', async path => {
    const { wrapper } = await renderSidebar(path)
    expect(wrapper.find('a[href="/model-catalog"]').classes()).toContain('sidebar-link-active')
  })

  it.each(['/usage', '/keys-old', '/model-tests'])('does not select API access for %s', async path => {
    const { wrapper } = await renderSidebar(path)
    expect(wrapper.find('a[href="/model-catalog"]').classes()).not.toContain('sidebar-link-active')
  })

  it.each([false, true])('takes the key onboarding step to key creation for simple=$0', async simple => {
    auth.isAdmin = true
    auth.isSimpleMode = simple
    mocks.onboarding.isCurrentStep.mockImplementation(selector => selector === '[data-tour="sidebar-my-keys"]')
    const { wrapper, router } = await renderSidebar('/admin/models')
    await wrapper.find('[data-tour="sidebar-my-keys"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/keys')
    expect(mocks.onboarding.nextStep).toHaveBeenCalledOnce()
    expect(mocks.onboarding.nextStep).toHaveBeenCalledWith(500)
  })

  it('keeps the collapsed entry accessible by its tooltip', async () => {
    app.sidebarCollapsed = true
    const { wrapper } = await renderSidebar('/keys')
    expect(wrapper.find('a[href="/model-catalog"]').attributes('title')).toBe('API 接入')
  })

  it('hides user navigation in backend mode while preserving admin access', async () => {
    app.backendModeEnabled = true
    const { wrapper } = await renderSidebar('/keys')
    expect(wrapper.find('a[href="/model-catalog"]').exists()).toBe(false)
    auth.isAdmin = true
    await flushPromises()
    expect(wrapper.find('a[href="/model-catalog"]').exists()).toBe(true)
  })
})
