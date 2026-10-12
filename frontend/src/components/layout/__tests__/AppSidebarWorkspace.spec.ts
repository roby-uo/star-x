import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { createMemoryHistory, createRouter } from 'vue-router'
import { ref } from 'vue'
import AppSidebar from '../AppSidebar.vue'
import zh from '@/i18n/locales/zh/common'

const mocks = vi.hoisted(() => ({
  app: {
    sidebarCollapsed: false, mobileOpen: false, siteName: 'star-X', siteLogo: '',
    siteVersion: 'test', publicSettingsLoaded: true, sidebarScrollTop: 0,
    cachedPublicSettings: {}, backendModeEnabled: false,
    toggleSidebar: vi.fn(), setMobileOpen: vi.fn()
  },
  auth: { isAdmin: true, isSimpleMode: false },
  adminSettings: { opsMonitoringEnabled: false, paymentEnabled: false, customMenuItems: [], fetch: vi.fn() },
  onboarding: { isCurrentStep: vi.fn(() => false), nextStep: vi.fn() }
}))

vi.mock('@/stores', () => ({
  useAppStore: () => mocks.app,
  useAuthStore: () => mocks.auth,
  useAdminSettingsStore: () => mocks.adminSettings,
  useOnboardingStore: () => mocks.onboarding
}))
vi.mock('@/utils/featureFlags', () => ({
  FeatureFlags: { availableChannels: {}, affiliate: {}, riskControl: {} },
  makeSidebarFlag: () => () => false
}))
vi.mock('@/composables/useBatchImageAccess', () => ({
  useBatchImageAccess: () => ({ canUseBatchImage: ref(false), refreshBatchImageAccess: vi.fn() })
}))

const wrappers: VueWrapper[] = []
const messages = { nav: Object.fromEntries(Object.entries(zh.nav).map(([key, value]) => [key, () => value])) }
afterEach(() => {
  wrappers.forEach(wrapper => wrapper.unmount())
  wrappers.length = 0
  mocks.auth.isSimpleMode = false
})

async function renderSidebar(path: string) {
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
  return wrapper
}

describe('sidebar model management workspace', () => {
  it.each([
    '/admin/models', '/admin/accounts', '/admin/groups', '/admin/billing-management',
    '/admin/channels/pricing', '/admin/channels/monitor', '/admin/accounts/123'
  ])('highlights the single model entry for %s', async path => {
    const wrapper = await renderSidebar(path)
    const modelLink = wrapper.find('a[href="/admin/models"]')
    expect(modelLink.text()).toBe('模型管理')
    expect(modelLink.classes()).toContain('sidebar-link-active')
    expect(wrapper.find('a[href="/admin/accounts"]').exists()).toBe(false)
    expect(wrapper.find('a[href="/admin/groups"]').exists()).toBe(false)
    expect(wrapper.find('a[href="/admin/billing-management"]').exists()).toBe(false)
  })

  it.each(['/admin/users', '/admin/accounts-archive', '/admin/billing-management-extra'])('does not highlight the model entry for unrelated %s', async path => {
    const wrapper = await renderSidebar(path)
    expect(wrapper.find('a[href="/admin/models"]').classes()).not.toContain('sidebar-link-active')
  })

  it('keeps the workspace entry available in simple mode', async () => {
    mocks.auth.isSimpleMode = true
    const wrapper = await renderSidebar('/admin/billing-management')
    expect(wrapper.find('#sidebar-model-manage').text()).toBe('模型管理')
    expect(wrapper.find('#sidebar-model-manage').classes()).toContain('sidebar-link-active')
  })
})
