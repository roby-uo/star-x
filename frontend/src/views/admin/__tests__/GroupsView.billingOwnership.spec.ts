import { defineComponent } from 'vue'
import { flushPromises, mount, RouterLinkStub } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { AdminGroup } from '@/types'
import GroupsView from '../GroupsView.vue'

const { getConfig, list, create, update, duplicate, showError } = vi.hoisted(() => ({
  getConfig: vi.fn(), list: vi.fn(), create: vi.fn(), update: vi.fn(),
  duplicate: vi.fn(), showError: vi.fn(),
}))
vi.mock('@/api/admin/billingManagement', () => ({ billingManagementAPI: { getConfig } }))
vi.mock('@/api/admin', () => ({
  adminAPI: {
    groups: {
      list, create, update, duplicate,
      getModelsListCandidates: vi.fn(async () => []),
      getUsageSummary: vi.fn(async () => []),
      getCapacitySummary: vi.fn(async () => []),
      getAll: vi.fn(async () => []),
    },
    accounts: { list: vi.fn(async () => ({ items: [] })), getById: vi.fn() },
  },
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess: vi.fn() }) }))
vi.mock('@/stores/onboarding', () => ({ useOnboardingStore: () => ({ isCurrentStep: () => false }) }))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key }),
}))

const hiddenPricingFields = [
  'rate_multiplier', 'image_rate_independent', 'image_rate_multiplier',
  'batch_image_discount_multiplier', 'batch_image_hold_multiplier',
  'image_price_1k', 'image_price_2k', 'image_price_4k',
  'video_rate_independent', 'video_rate_multiplier',
  'video_price_480p', 'video_price_720p', 'video_price_1080p',
  'web_search_price_per_call', 'peak_rate_enabled', 'peak_start', 'peak_end',
  'peak_rate_multiplier',
]
const group = (platform: AdminGroup['platform'] = 'gemini'): AdminGroup => ({
  id: 7, name: 'Standard', description: '', platform,
  rate_multiplier: 1.4, rpm_limit: 23, status: 'active', is_exclusive: false,
  subscription_type: 'subscription', daily_limit_usd: 10, weekly_limit_usd: null,
  monthly_limit_usd: null, allow_image_generation: true, allow_batch_image_generation: true,
  image_rate_independent: true, image_rate_multiplier: 3,
  batch_image_discount_multiplier: 0.8, batch_image_hold_multiplier: 0.9,
  image_price_1k: 0.01, image_price_2k: 0.02, image_price_4k: 0.03,
  video_rate_independent: true, video_rate_multiplier: 4,
  video_price_480p: 0.05, video_price_720p: 0.07, video_price_1080p: 0.25,
  web_search_price_per_call: 0.01, peak_rate_enabled: true,
  peak_start: '08:00', peak_end: '12:00', peak_rate_multiplier: 2,
  claude_code_only: false, fallback_group_id: null,
  fallback_group_id_on_invalid_request: null, allow_messages_dispatch: false,
  default_mapped_model: '', require_oauth_only: false, require_privacy_set: false,
  created_at: '', updated_at: '', model_routing: null, model_routing_enabled: false,
  mcp_xml_inject: true, supported_model_scopes: [], account_count: 1,
  active_account_count: 1, rate_limited_account_count: 0, sort_order: 0,
})

const TableStub = defineComponent({
  props: ['data', 'columns'],
  template: `<div><p data-test="columns">{{ columns.map(c => c.label).join(',') }}</p>
    <div v-for="row in data" :key="row.id"><slot name="cell-rate_multiplier" :value="row.rate_multiplier" /><slot name="cell-actions" :row="row" /></div></div>`,
})
const DialogStub = { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' }
const SelectStub = {
  props: ['modelValue', 'options'], emits: ['update:modelValue', 'change'],
  template: `<select :value="modelValue" @change="$emit('update:modelValue', $event.target.value); $emit('change')"><option v-for="o in options" :key="o.value" :value="o.value">{{ o.label }}</option></select>`,
}
const wrappers: ReturnType<typeof mount>[] = []
async function mountView() {
  const wrapper = mount(GroupsView, { global: { stubs: {
    AppLayout: { template: '<main><slot /></main>' },
    TablePageLayout: { template: '<section><slot name="filters" /><slot name="table" /></section>' },
    AdminWorkspaceTabs: true, RouterLink: RouterLinkStub, DataTable: TableStub,
    BaseDialog: DialogStub, Select: SelectStub, ConfirmDialog: true, EmptyState: true,
    Pagination: true, Icon: true, PlatformIcon: true, GroupCapacityBadge: true,
    GroupRateMultipliersModal: true, GroupRPMOverridesModal: true, VueDraggable: true,
  } } })
  wrappers.push(wrapper)
  await flushPromises()
  return wrapper
}
async function openEdit(wrapper: ReturnType<typeof mount>) {
  await wrapper.findAll('button').find(b => b.text() === 'common.edit')!.trigger('click')
  await flushPromises()
}

describe('GroupsView centralized billing ownership', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.clearAllMocks()
    getConfig.mockResolvedValue({ simple_billing_enabled: true })
    list.mockResolvedValue({ items: [group()], total: 1, pages: 1 })
    update.mockResolvedValue(group())
    create.mockResolvedValue(group())
    duplicate.mockResolvedValue({ ...group(), id: 8 })
  })
  afterEach(() => { wrappers.splice(0).forEach(w => w.unmount()); vi.restoreAllMocks() })

  it('shows one billing destination and retains image and batch permissions', async () => {
    const wrapper = await mountView()
    expect(wrapper.get('[data-test="columns"]').text()).toContain('授权倍率')
    expect(wrapper.getComponent('[data-testid="group-billing-settings"]').props('to')).toBe('/admin/billing-management')
    expect(wrapper.text()).not.toContain('admin.groups.rateMultipliers')
    await openEdit(wrapper)
    const form = wrapper.get('#edit-group-form')
    expect(form.find('[data-tour="group-form-multiplier"]').exists()).toBe(false)
    expect(form.text()).toContain('图片生成权限')
    expect(form.text()).toContain('admin.groups.imagePricing.allowImageGeneration')
    expect(form.text()).toContain('admin.groups.imagePricing.allowBatchImageGeneration')
    expect(form.text()).not.toContain('admin.groups.imagePricing.batchDiscountMultiplier')
    expect(form.text()).not.toContain('admin.groups.peakRate.enable')
    expect(form.text()).not.toContain('1K (¥)')
  })

  it('omits stale centralized prices while saving live group permissions and limits', async () => {
    const wrapper = await mountView()
    await openEdit(wrapper)
    await wrapper.get('[data-tour="edit-group-form-name"]').setValue('VIP')
    await wrapper.get('#edit-group-form').trigger('submit')
    await flushPromises()
    expect(update).toHaveBeenCalledTimes(1)
    const [id, payload] = update.mock.calls[0]!
    expect(id).toBe(7)
    for (const field of hiddenPricingFields) expect(payload).not.toHaveProperty(field)
    expect(payload).toMatchObject({ name: 'VIP', allow_image_generation: true,
      allow_batch_image_generation: true, rpm_limit: 23, daily_limit_usd: 10 })
  })

  it.each(['openai', 'grok'] as const)('hides %s legacy search or video pricing', async (platform) => {
    list.mockResolvedValue({ items: [group(platform)], total: 1, pages: 1 })
    const wrapper = await mountView()
    await openEdit(wrapper)
    const text = wrapper.get('#edit-group-form').text()
    expect(text).not.toContain('admin.groups.webSearchPricing.title')
    expect(text).not.toContain('admin.groups.videoPricing.title')
    expect(text).not.toContain('480p (¥/s)')
  })

  it('creates new groups with the required neutral authorization default only', async () => {
    const wrapper = await mountView()
    await wrapper.findAll('button').find(b => b.text() === 'admin.groups.createGroup')!.trigger('click')
    await wrapper.get('[data-tour="group-form-name"]').setValue('New')
    await wrapper.get('#create-group-form').trigger('submit')
    await flushPromises()
    expect(create).toHaveBeenCalledTimes(1)
    const payload = create.mock.calls[0]![0]
    expect(payload.rate_multiplier).toBe(1)
    for (const field of hiddenPricingFields.filter(f => f !== 'rate_multiplier')) expect(payload).not.toHaveProperty(field)
  })

  it('keeps legacy editing and payload semantics when centralized billing is disabled', async () => {
    getConfig.mockResolvedValue({ simple_billing_enabled: false })
    list.mockResolvedValue({ items: [group('grok')], total: 1, pages: 1 })
    const wrapper = await mountView()
    expect(wrapper.text()).toContain('admin.groups.rateMultipliers')
    await openEdit(wrapper)
    expect(wrapper.get('#edit-group-form').text()).toContain('480p (¥/s)')
    expect(wrapper.get('#edit-group-form').text()).toContain('admin.groups.peakRate.enable')
    await wrapper.get('[data-tour="group-form-multiplier"]').setValue('1.8')
    await wrapper.get('#edit-group-form').trigger('submit')
    await flushPromises()
    expect(update.mock.calls[0]![1]).toMatchObject({ rate_multiplier: 1.8,
      image_price_1k: 0.01, video_price_480p: 0.05, peak_rate_multiplier: 2 })
  })

  it('blocks ambiguous saves after a config error and recovers through retry', async () => {
    getConfig.mockRejectedValueOnce(new Error('network unavailable'))
    const wrapper = await mountView()
    expect(wrapper.text()).not.toContain('1.4x')
    await openEdit(wrapper)
    expect(wrapper.get('button[form="edit-group-form"]').attributes('disabled')).toBeDefined()
    expect(wrapper.find('[data-tour="group-form-multiplier"]').exists()).toBe(false)
    await wrapper.get('#edit-group-form').trigger('submit')
    expect(update).not.toHaveBeenCalled()
    await wrapper.get('[data-testid="group-billing-retry"]').trigger('click')
    await flushPromises()
    expect(getConfig).toHaveBeenCalledTimes(2)
    expect(wrapper.get('button[form="edit-group-form"]').attributes('disabled')).toBeUndefined()
    await wrapper.get('#edit-group-form').trigger('submit')
    await flushPromises()
    expect(update).toHaveBeenCalledTimes(1)
    expect(update.mock.calls[0]![1]).not.toHaveProperty('rate_multiplier')
  })

  it('duplicates through the server without rewriting source pricing', async () => {
    const wrapper = await mountView()
    await wrapper.get('[data-testid="group-duplicate"]').trigger('click')
    await flushPromises()
    expect(duplicate).toHaveBeenCalledWith(7)
    expect(update).not.toHaveBeenCalled()
    expect(create).not.toHaveBeenCalled()
  })
})
