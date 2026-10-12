import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import UserDashboardStats from '../UserDashboardStats.vue'
import UserDashboardRecentUsage from '../UserDashboardRecentUsage.vue'
import UserDashboardCharts from '../UserDashboardCharts.vue'

vi.mock('vue-i18n', async () => ({ ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('vue-chartjs', () => ({ Doughnut: { props: ['data'], template: '<div class="chart-data">{{ JSON.stringify(data) }}</div>' } }))

describe('user dashboard billing privacy', () => {
  it('uses actual tokens for platform usage and keeps only the personal balance amount', () => {
    const stats = {
      total_api_keys: 1, active_api_keys: 1, total_requests: 8, today_requests: 2,
      total_input_tokens: 100, total_output_tokens: 200, total_cache_creation_tokens: 10, total_cache_read_tokens: 30, total_tokens: 340,
      today_input_tokens: 10, today_output_tokens: 20, today_cache_creation_tokens: 0, today_cache_read_tokens: 0, today_tokens: 30,
      average_duration_ms: 200, rpm: 1, tpm: 30,
      total_cost: 9.8765, total_actual_cost: 8.7654, today_cost: 1.2345, today_actual_cost: 1.1111,
      by_platform: [{ platform: 'openai', total_tokens: 340, today_tokens: 30, total_requests: 8, today_requests: 2, total_actual_cost: 8.7654, today_actual_cost: 1.1111 }],
    }
    const wrapper = mount(UserDashboardStats, { props: { stats, balance: 100, isSimple: false }, global: { stubs: { Icon: true } } })
    expect(wrapper.text()).toContain('$100.00')
    expect(wrapper.text()).toContain('340 tokens')
    expect(wrapper.text()).not.toContain('dashboard.todayCost')
    expect(wrapper.text()).not.toContain('dashboard.standard')
    expect(wrapper.text()).not.toContain('8.7654')
  })

  it('displays tokens and latency in recent requests without costs', () => {
    const log = { id: 1, model: 'example-model', created_at: '2026-10-12T00:00:00Z', input_tokens: 10, output_tokens: 20, cache_creation_tokens: 5, cache_read_tokens: 15, actual_cost: 9.8765, total_cost: 8.7654, duration_ms: 250 }
    const wrapper = mount(UserDashboardRecentUsage, { props: { data: [log] as any, loading: false }, global: { stubs: { Icon: true, RouterLink: { template: '<a><slot /></a>' } } } })
    expect(wrapper.text()).toContain('50 tokens')
    expect(wrapper.text()).toContain('0.25s')
    expect(wrapper.text()).not.toContain('$')
    expect(wrapper.text()).not.toContain('9.8765')
  })

  it('omits costs from the model distribution table', () => {
    const model = { model: 'example-model', requests: 2, input_tokens: 10, output_tokens: 20, cache_creation_tokens: 0, cache_read_tokens: 0, total_tokens: 30, cost: 9.8765, actual_cost: 8.7654 }
    const wrapper = mount(UserDashboardCharts, { props: { models: [model], trend: [], loading: false, startDate: '2026-10-12', endDate: '2026-10-12', granularity: 'day' }, global: { stubs: { DateRangePicker: true, Select: true, TokenUsageTrend: true } } })
    expect(wrapper.findAll('th')).toHaveLength(3)
    expect(wrapper.text()).toContain('example-model')
    expect(wrapper.text()).not.toContain('$')
    expect(wrapper.text()).not.toContain('dashboard.actual')
  })
})
