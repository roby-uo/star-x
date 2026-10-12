import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { nextTick } from 'vue'

import type { ApiKey } from '@/types'
import KeysView from '../KeysView.vue'

const {
  listKeys,
  createKey,
  routerPush,
  routeState,
  getPublicSettings,
  getDashboardApiKeysUsage,
  getAvailableGroups,
  getUserGroupRates,
  showError,
  showSuccess,
  copyToClipboard,
  isCurrentStep,
  nextStep,
} = vi.hoisted(() => ({
  listKeys: vi.fn(),
  createKey: vi.fn(),
  routerPush: vi.fn(),
  routeState: { query: {} as Record<string, unknown>, fullPath: '/keys' },
  getPublicSettings: vi.fn(),
  getDashboardApiKeysUsage: vi.fn(),
  getAvailableGroups: vi.fn(),
  getUserGroupRates: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  copyToClipboard: vi.fn(),
  isCurrentStep: vi.fn(),
  nextStep: vi.fn(),
}))

const messages: Record<string, string> = {
  'common.actions': 'Actions',
  'common.name': 'Name',
  'common.refresh': 'Refresh',
  'common.status': 'Status',
  'keys.apiKey': 'API Key',
  'keys.allGroups': 'All Groups',
  'keys.allStatus': 'All Status',
  'keys.columnSettings': 'Column Settings',
  'keys.createKey': 'Create API Key',
  'keys.created': 'Created',
  'keys.expiresAt': 'Expires',
  'keys.group': 'Group',
  'keys.id': 'ID',
  'keys.currentConcurrency': 'Current Concurrency',
  'keys.lastUsedAt': 'Last Used',
  'keys.lastUsedIP': 'Last Used IP',
  'keys.rateLimitColumn': 'Rate Limit',
  'keys.searchPlaceholder': 'Search name or key...',
  'keys.status.active': 'Active',
  'keys.status.expired': 'Expired',
  'keys.status.inactive': 'Inactive',
  'keys.status.quota_exhausted': 'Quota exhausted',
  'keys.usage': 'Usage',
}

vi.mock('@/api', () => ({
  keysAPI: {
    list: listKeys,
    create: createKey,
    update: vi.fn(),
    delete: vi.fn(),
    toggleStatus: vi.fn(),
  },
  authAPI: {
    getPublicSettings,
  },
  usageAPI: {
    getDashboardApiKeysUsage,
  },
  userGroupsAPI: {
    getAvailable: getAvailableGroups,
    getUserGroupRates,
  },
}))

vi.mock('vue-router', () => ({
  useRoute: () => routeState,
  useRouter: () => ({ push: routerPush }),
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
  }),
}))

vi.mock('@/stores/onboarding', () => ({
  useOnboardingStore: () => ({
    isCurrentStep,
    nextStep,
  }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard,
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key,
    }),
  }
})

const createApiKey = (): ApiKey => ({
  id: 1,
  user_id: 1,
  key: 'sk-test-key',
  name: 'test-key',
  group_id: null,
  status: 'active',
  ip_whitelist: [],
  ip_blacklist: [],
  last_used_at: null,
  last_used_ip: null,
  quota: 0,
  quota_used: 0,
  expires_at: null,
  created_at: '2026-06-27T00:00:00Z',
  updated_at: '2026-06-27T00:00:00Z',
  current_concurrency: 3,
  rate_limit_5h: 0,
  rate_limit_1d: 0,
  rate_limit_7d: 0,
  usage_5h: 0,
  usage_1d: 0,
  usage_7d: 0,
  window_5h_start: null,
  window_1d_start: null,
  window_7d_start: null,
  reset_5h_at: null,
  reset_1d_at: null,
  reset_7d_at: null,
})

const AppLayoutStub = {
  template: '<div><slot /></div>',
}

const TablePageLayoutStub = {
  template: `
    <div>
      <slot name="filters" />
      <slot name="actions" />
      <slot name="table" />
      <slot name="pagination" />
    </div>
  `,
}

const DataTableStub = {
  name: 'DataTable',
  props: ['columns', 'data'],
  emits: ['sort'],
  template: `
    <div>
      <div data-test="columns">{{ columns.map((col) => col.key).join(',') }}</div>
      <div data-test="columns-meta">{{ JSON.stringify(columns.map((col) => ({ key: col.key, sortable: !!col.sortable }))) }}</div>
      <button data-test="sort-current-concurrency" @click="$emit('sort', 'current_concurrency', 'asc')">
        Sort Current Concurrency
      </button>
      <div v-for="row in data" :key="row.id">
        <div
          v-if="columns.some((col) => col.key === 'id')"
          data-test="key-id"
        >
          <slot name="cell-id" :value="row.id" :row="row" />
        </div>
        <slot name="cell-name" :value="row.name" :row="row" />
        <div data-test="token-usage"><slot name="cell-usage" :row="row" /></div>
        <div data-test="current-concurrency">
          <slot name="cell-current_concurrency" :value="row.current_concurrency" :row="row" />
        </div>
        <div
          v-if="columns.some((col) => col.key === 'last_used_ip')"
          data-test="last-used-ip"
        >
          <slot name="cell-last_used_ip" :value="row.last_used_ip" :row="row" />
        </div>
      </div>
      <slot name="empty" />
    </div>
  `,
}

const SelectStub = {
  name: 'Select',
  props: ['modelValue', 'options', 'disabled'],
  emits: ['update:modelValue'],
  template: '<select :value="modelValue" :disabled="disabled" @change="$emit(\'update:modelValue\', $event.target.value)"></select>',
}

const SearchInputStub = {
  name: 'SearchInput',
  props: ['modelValue'],
  emits: ['update:modelValue', 'search'],
  template: '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />',
}

const PaginationStub = {
  name: 'Pagination',
  props: ['page', 'total', 'pageSize'],
  emits: ['update:page', 'update:pageSize'],
  template: `
    <div>
      <button data-test="page-size-50" @click="$emit('update:pageSize', 50)">50</button>
    </div>
  `,
}

const IconStub = {
  props: ['name'],
  template: '<span data-test="icon">{{ name }}</span>',
}

const BaseDialogStub = {
  name: 'BaseDialog',
  props: ['show', 'title', 'showCloseButton', 'closeOnEscape', 'closeOnClickOutside'],
  emits: ['close'],
  template: '<section v-if="show" data-test="key-dialog"><button v-if="showCloseButton !== false" data-test="close-key-dialog" @click="$emit(\'close\')">Close</button><slot /><slot name="footer" /></section>',
}

const RouterLinkStub = {
  name: 'RouterLink',
  props: ['to'],
  template: '<a><slot /></a>',
}

const mountView = async () => {
  const wrapper = mount(KeysView, {
    global: {
      stubs: {
        AppLayout: AppLayoutStub,
        TablePageLayout: TablePageLayoutStub,
        DataTable: DataTableStub,
        Pagination: PaginationStub,
        BaseDialog: BaseDialogStub,
        ConfirmDialog: true,
        EmptyState: true,
        Select: SelectStub,
        SearchInput: SearchInputStub,
        Icon: IconStub,
        UseKeyModal: true,
        EndpointPopover: true,
        GroupBadge: true,
        GroupOptionItem: true,
        UserAccessWorkspace: true,
        RouterLink: RouterLinkStub,
        Teleport: true,
      },
    },
  })
  await flushPromises()
  await nextTick()
  return wrapper
}

const visibleColumnKeys = (wrapper: VueWrapper) =>
  wrapper.get('[data-test="columns"]').text().split(',').filter(Boolean)

const visibleColumnMeta = (wrapper: VueWrapper): Array<{ key: string; sortable: boolean }> =>
  JSON.parse(wrapper.get('[data-test="columns-meta"]').text())

const getButtonByText = (wrapper: VueWrapper, text: string) => {
  const button = wrapper.findAll('button').find((item) => item.text().includes(text))
  if (!button) {
    throw new Error(`Button not found: ${text}`)
  }
  return button
}

describe('user KeysView column settings', () => {
  beforeEach(() => {
    localStorage.clear()

    listKeys.mockReset()
    createKey.mockReset()
    routerPush.mockReset()
    routeState.query = {}
    routeState.fullPath = '/keys'
    getPublicSettings.mockReset()
    getDashboardApiKeysUsage.mockReset()
    getAvailableGroups.mockReset()
    getUserGroupRates.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
    copyToClipboard.mockReset()
    isCurrentStep.mockReset()
    nextStep.mockReset()

    listKeys.mockResolvedValue({
      items: [createApiKey()],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    getPublicSettings.mockResolvedValue({})
    getDashboardApiKeysUsage.mockResolvedValue({ stats: {} })
    getAvailableGroups.mockResolvedValue([])
    createKey.mockResolvedValue({ ...createApiKey(), id: 55, group_id: 42 })
    routerPush.mockResolvedValue(undefined)
    getUserGroupRates.mockResolvedValue({})
    isCurrentStep.mockReturnValue(false)
  })

  it('uses the default API key columns with low-frequency columns hidden', async () => {
    const wrapper = await mountView()

    expect(visibleColumnKeys(wrapper)).toEqual([
      'name',
      'key',
      'group',
      'current_concurrency',
      'usage',
      'expires_at',
      'status',
      'created_at',
      'actions',
    ])
    expect(visibleColumnKeys(wrapper)).not.toContain('rate_limit')
    expect(visibleColumnKeys(wrapper)).not.toContain('last_used_at')
    expect(visibleColumnKeys(wrapper)).not.toContain('last_used_ip')
    expect(visibleColumnKeys(wrapper)).not.toContain('id')
  })

  it('shows a hidden column when toggled and persists the preference', async () => {
    const wrapper = await mountView()

    await wrapper.get('button[title="Column Settings"]').trigger('click')
    await getButtonByText(wrapper, 'Rate Limit').trigger('click')
    await nextTick()

    expect(visibleColumnKeys(wrapper)).toContain('rate_limit')
    expect(localStorage.getItem('api-key-hidden-columns')).toBe(
      JSON.stringify(['id', 'last_used_at', 'last_used_ip'])
    )
    expect(localStorage.getItem('api-key-column-settings-version')).toBe('3')
  })

  it('shows the API key ID column when toggled', async () => {
    const wrapper = await mountView()

    await wrapper.get('button[title="Column Settings"]').trigger('click')
    await getButtonByText(wrapper, 'ID').trigger('click')
    await nextTick()

    expect(visibleColumnKeys(wrapper)).toContain('id')
    expect(wrapper.get('[data-test="key-id"]').text()).toBe('#1')
    expect(visibleColumnMeta(wrapper).find((column) => column.key === 'id')?.sortable).toBe(true)
  })

  it('shows the last used IP column when toggled', async () => {
    listKeys.mockResolvedValueOnce({
      items: [{ ...createApiKey(), last_used_ip: '203.0.113.10' }],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    const wrapper = await mountView()

    await wrapper.get('button[title="Column Settings"]').trigger('click')
    await getButtonByText(wrapper, 'Last Used IP').trigger('click')
    await nextTick()

    expect(visibleColumnKeys(wrapper)).toContain('last_used_ip')
    expect(wrapper.get('[data-test="last-used-ip"]').text()).toBe('203.0.113.10')
  })

  it('restores column preferences from localStorage on mount', async () => {
    localStorage.setItem('api-key-hidden-columns', JSON.stringify(['group', 'created_at']))
    localStorage.setItem('api-key-column-settings-version', '1')

    const wrapper = await mountView()

    expect(visibleColumnKeys(wrapper)).toEqual([
      'name',
      'key',
      'current_concurrency',
      'usage',
      'rate_limit',
      'expires_at',
      'status',
      'last_used_at',
      'actions',
    ])
    expect(localStorage.getItem('api-key-hidden-columns')).toBe(
      JSON.stringify(['group', 'created_at', 'last_used_ip', 'id'])
    )
    expect(localStorage.getItem('api-key-column-settings-version')).toBe('3')
  })

  it('does not include always-visible columns in the toggleable menu', async () => {
    const wrapper = await mountView()

    await wrapper.get('button[title="Column Settings"]').trigger('click')
    await nextTick()

    const columnMenuText = wrapper.text()
    expect(columnMenuText).toContain('API Key')
    expect(columnMenuText).toContain('ID')
    expect(columnMenuText).toContain('Current Concurrency')
    expect(columnMenuText).toContain('Rate Limit')
    expect(columnMenuText).toContain('Last Used IP')
    expect(columnMenuText).not.toContain('Name')
    expect(columnMenuText).not.toContain('Actions')
  })

  it('renders the current concurrency value', async () => {
    const wrapper = await mountView()

    expect(wrapper.get('[data-test="current-concurrency"]').text()).toBe('3')
  })

  it('shows real token counts for today and the last 30 days without costs or rate requests', async () => {
    getDashboardApiKeysUsage.mockResolvedValue({ stats: { 1: { api_key_id: 1, today_tokens: 1200, total_tokens: 23456, today_requests: 2, total_requests: 30, today_actual_cost: 9.99, total_actual_cost: 88.88 } } })
    const wrapper = await mountView()
    const usage = wrapper.get('[data-test="token-usage"]').text()
    expect(usage).toContain('1,200 tokens')
    expect(usage).toContain('23,456 tokens')
    expect(usage).not.toContain('$')
    expect(usage).not.toContain('88.88')
    expect(getUserGroupRates).not.toHaveBeenCalled()
  })

  it('marks current concurrency as sortable', async () => {
    const wrapper = await mountView()

    const currentConcurrencyColumn = visibleColumnMeta(wrapper).find(
      (column) => column.key === 'current_concurrency'
    )
    expect(currentConcurrencyColumn?.sortable).toBe(true)
  })

  it('keeps filters and selected page size when sorting by current concurrency', async () => {
    getAvailableGroups.mockResolvedValue([{ id: 42, name: 'OpenAI' }])
    const wrapper = await mountView()

    await wrapper.get('[data-test="page-size-50"]').trigger('click')
    await flushPromises()

    await wrapper.findComponent({ name: 'SearchInput' }).vm.$emit('update:modelValue', 'target')
    await wrapper.findComponent({ name: 'SearchInput' }).vm.$emit('search')
    await flushPromises()

    const selects = wrapper.findAllComponents({ name: 'Select' })
    await selects[0].vm.$emit('update:modelValue', 42)
    await flushPromises()
    await selects[1].vm.$emit('update:modelValue', 'active')
    await flushPromises()

    listKeys.mockClear()

    await wrapper.get('[data-test="sort-current-concurrency"]').trigger('click')
    await flushPromises()

    expect(listKeys).toHaveBeenLastCalledWith(
      1,
      50,
      {
        search: 'target',
        status: 'active',
        group_id: 42,
        sort_by: 'current_concurrency',
        sort_order: 'asc',
      },
      expect.objectContaining({ signal: expect.any(AbortSignal) })
    )
  })

  it('does not open or create a key when visiting the keys page directly', async () => {
    const wrapper = await mountView()
    expect(wrapper.find('[data-test="key-dialog"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="model-return-context"]').exists()).toBe(false)
    expect(createKey).not.toHaveBeenCalled()
  })

  it('waits for available groups before opening a contextual create form', async () => {
    routeState.query = { model: 'MiniMax-H3', platform: 'openai', kind: 'Videos', group: '42', tab: 'api', create: '1' }
    let resolveGroups!: (groups: unknown[]) => void
    getAvailableGroups.mockImplementationOnce(() => new Promise(resolve => { resolveGroups = resolve }))
    const wrapper = await mountView()
    expect(wrapper.find('[data-test="key-dialog"]').exists()).toBe(false)
    expect(createKey).not.toHaveBeenCalled()

    resolveGroups([{ id: 42, name: 'Accessible', platform: 'openai' }])
    await flushPromises()
    expect(wrapper.find('[data-test="key-dialog"]').exists()).toBe(true)
    const selector = wrapper.get('[data-tour="key-form-group"]')
    expect(selector.element.getAttribute('value') ?? (selector.element as HTMLSelectElement).value).toBe('42')
    expect(createKey).not.toHaveBeenCalled()
  })

  it('does not preselect an inaccessible query group and does not reopen after cancel', async () => {
    routeState.query = { model: 'MiniMax-H3', platform: 'openai', kind: 'Videos', group: '999', tab: 'test', create: '1' }
    getAvailableGroups.mockResolvedValue([{ id: 42, name: 'Accessible', platform: 'openai' }])
    const wrapper = await mountView()
    expect((wrapper.get('[data-tour="key-form-group"]').element as HTMLSelectElement).value).toBe('')
    await wrapper.get('[data-test="close-key-dialog"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-test="key-dialog"]').exists()).toBe(false)
    expect(createKey).not.toHaveBeenCalled()
  })

  it('returns to the selected model with the created key ID, never its secret', async () => {
    routeState.query = { model: 'MiniMax-H3', platform: 'openai', kind: 'Videos', group: '42', tab: 'test', create: '1', returnTo: 'https://evil.example' }
    getAvailableGroups.mockResolvedValue([{ id: 42, name: 'Accessible', platform: 'openai' }])
    const wrapper = await mountView()
    await wrapper.get('[data-tour="key-form-name"]').setValue('video-access')
    await wrapper.get('#key-form').trigger('submit')
    await flushPromises()

    expect(createKey).toHaveBeenCalledWith('video-access', 42, undefined, [], [], 0, undefined, { rate_limit_5h: 0, rate_limit_1d: 0, rate_limit_7d: 0 })
    expect(routerPush).toHaveBeenCalledWith({ path: '/model-catalog', query: { model: 'MiniMax-H3', platform: 'openai', kind: 'Videos', tab: 'test', group: '42', key: '55' } })
    expect(JSON.stringify(routerPush.mock.calls)).not.toContain('sk-test-key')
    expect(wrapper.find('[data-test="key-dialog"]').exists()).toBe(false)
  })

  it('preserves the form and model context on creation failure', async () => {
    routeState.query = { model: 'MiniMax-H3', platform: 'openai', kind: 'Videos', group: '42', tab: 'api', create: '1' }
    getAvailableGroups.mockResolvedValue([{ id: 42, name: 'Accessible', platform: 'openai' }])
    createKey.mockRejectedValue({ response: { data: { detail: 'Unable to create' } } })
    const wrapper = await mountView()
    await wrapper.get('[data-tour="key-form-name"]').setValue('keep-my-form')
    await wrapper.get('#key-form').trigger('submit')
    await flushPromises()

    expect(showError).toHaveBeenCalledWith('Unable to create')
    expect(wrapper.find('[data-test="key-dialog"]').exists()).toBe(true)
    expect((wrapper.get('[data-tour="key-form-name"]').element as HTMLInputElement).value).toBe('keep-my-form')
    expect(wrapper.get('[data-test="model-return-context"]').exists()).toBe(true)
    expect(routerPush).not.toHaveBeenCalled()
  })

  it('keeps contextual creation closed when groups fail, then opens after retry succeeds', async () => {
    routeState.query = { model: 'MiniMax-H3', platform: 'openai', kind: 'Videos', group: '42', tab: 'api', create: '1' }
    getAvailableGroups.mockRejectedValueOnce(new Error('Groups unavailable'))
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {})
    const wrapper = await mountView()
    expect(wrapper.find('[data-test="key-dialog"]').exists()).toBe(false)
    getAvailableGroups.mockResolvedValue([{ id: 42, name: 'Accessible', platform: 'openai' }])
    await wrapper.get('[data-test="model-return-context"] button').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-test="key-dialog"]').exists()).toBe(true)
    expect(createKey).not.toHaveBeenCalled()
    consoleError.mockRestore()
  })

  it('ignores malformed model contexts even when create is requested', async () => {
    routeState.query = { model: 'MiniMax-H3', platform: 'unknown', kind: 'Videos', group: '42', create: '1' }
    getAvailableGroups.mockResolvedValue([{ id: 42, name: 'Accessible', platform: 'openai' }])
    const wrapper = await mountView()
    expect(wrapper.find('[data-test="key-dialog"]').exists()).toBe(false)
    expect(createKey).not.toHaveBeenCalled()
  })

  it('locks pending creation against closing and duplicate submission and returns the submitted group with only the new key ID', async () => {
    routeState.query = { model: 'MiniMax-H3', platform: 'openai', kind: 'Videos', group: '42', tab: 'test', create: '1', key: '1' }
    getAvailableGroups.mockResolvedValue([{ id: 42, name: 'Accessible', platform: 'openai' }, { id: 99, name: 'Another', platform: 'openai' }])
    let resolveCreate!: (key: ApiKey) => void
    createKey.mockImplementationOnce(() => new Promise(resolve => { resolveCreate = resolve }))
    const wrapper = await mountView()
    await wrapper.get('[data-tour="key-form-name"]').setValue('pending-key')
    await wrapper.get('#key-form').trigger('submit')

    expect((wrapper.get('[data-test="key-form-fields"]').element as HTMLFieldSetElement).disabled).toBe(true)
    expect((wrapper.get('[data-test="cancel-key-form"]').element as HTMLButtonElement).disabled).toBe(true)
    const dialog = wrapper.findAllComponents({ name: 'BaseDialog' }).find(component => component.props('show'))!
    expect(dialog.props('closeOnEscape')).toBe(false)
    expect(dialog.props('showCloseButton')).toBe(false)
    expect(dialog.props('closeOnClickOutside')).toBe(false)
    await dialog.vm.$emit('close')
    await wrapper.get('[data-test="cancel-key-form"]').trigger('click')
    await wrapper.get('#key-form').trigger('submit')
    expect(wrapper.find('[data-test="key-dialog"]').exists()).toBe(true)
    expect(createKey).toHaveBeenCalledTimes(1)

    // Even a stale child event cannot change the return metadata of this request.
    const groupSelect = wrapper.findAllComponents({ name: 'Select' }).find(component => component.attributes('data-tour') === 'key-form-group')!
    expect(groupSelect.props('disabled')).toBe(true)
    groupSelect.vm.$emit('update:modelValue', 99)
    await nextTick()
    resolveCreate({ ...createApiKey(), id: 77, group_id: 42, key: 'sk-secret-new-key' })
    await flushPromises()

    expect(routerPush).toHaveBeenCalledTimes(1)
    expect(routerPush).toHaveBeenCalledWith({ path: '/model-catalog', query: { model: 'MiniMax-H3', platform: 'openai', kind: 'Videos', tab: 'test', group: '42', key: '77' } })
    expect(JSON.stringify(routerPush.mock.calls)).not.toContain('sk-secret-new-key')
    expect(wrapper.find('[data-test="key-dialog"]').exists()).toBe(false)
  })

  it('does not redirect a late creation result after the user has navigated to a different context', async () => {
    routeState.query = { model: 'MiniMax-H3', platform: 'openai', kind: 'Videos', group: '42', tab: 'api', create: '1' }
    getAvailableGroups.mockResolvedValue([{ id: 42, name: 'Accessible', platform: 'openai' }])
    let resolveCreate!: (key: ApiKey) => void
    createKey.mockImplementationOnce(() => new Promise(resolve => { resolveCreate = resolve }))
    const wrapper = await mountView()
    await wrapper.get('[data-tour="key-form-name"]').setValue('pending-key')
    await wrapper.get('#key-form').trigger('submit')
    routeState.query = { model: 'another-model', platform: 'openai', kind: 'Chat', tab: 'api' }
    routeState.fullPath = '/keys?model=another-model'
    resolveCreate({ ...createApiKey(), id: 88, group_id: 42 })
    await flushPromises()
    expect(routerPush).not.toHaveBeenCalled()
  })

  it('does not redirect or reload the old page when creation finishes after unmount', async () => {
    routeState.query = { model: 'MiniMax-H3', platform: 'openai', kind: 'Videos', group: '42', tab: 'api', create: '1' }
    getAvailableGroups.mockResolvedValue([{ id: 42, name: 'Accessible', platform: 'openai' }])
    let resolveCreate!: (key: ApiKey) => void
    createKey.mockImplementationOnce(() => new Promise(resolve => { resolveCreate = resolve }))
    const wrapper = await mountView()
    await wrapper.get('[data-tour="key-form-name"]').setValue('pending-key')
    await wrapper.get('#key-form').trigger('submit')
    const listCallCount = listKeys.mock.calls.length
    wrapper.unmount()
    resolveCreate({ ...createApiKey(), id: 89, group_id: 42 })
    await flushPromises()
    expect(routerPush).not.toHaveBeenCalled()
    expect(listKeys).toHaveBeenCalledTimes(listCallCount)
  })
})
