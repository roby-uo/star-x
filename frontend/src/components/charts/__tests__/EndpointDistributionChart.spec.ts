import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import EndpointDistributionChart from '../EndpointDistributionChart.vue'

vi.mock('vue-i18n', async () => ({ ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('vue-chartjs', () => ({ Doughnut: { props: ['data'], template: '<div class="chart-data">{{ JSON.stringify(data) }}</div>' } }))

describe('EndpointDistributionChart user privacy', () => {
  const endpointStats = [{ endpoint: '/v1/chat/completions', requests: 2, total_tokens: 120, cost: 5.6789, actual_cost: 4.5678 }]
  it('uses tokens and hides cost metrics and amounts in user mode', () => {
    const wrapper = mount(EndpointDistributionChart, { props: { endpointStats, showCosts: false, showMetricToggle: true, metric: 'actual_cost' } })
    expect(JSON.parse(wrapper.find('.chart-data').text()).datasets[0].data).toEqual([120])
    expect(wrapper.findAll('th')).toHaveLength(3)
    expect(wrapper.text()).not.toContain('$')
    expect(wrapper.text()).not.toContain('¥')
    expect(wrapper.text()).not.toContain('metricActualCost')
    expect(wrapper.text()).not.toContain('admin.dashboard.standard')
    const options = (wrapper.vm as any).$?.setupState.doughnutOptions
    expect(options.plugins.tooltip.callbacks.label({ label: 'chat', raw: 120, dataset: { data: [120] } })).toBe('chat: 120 (100.0%)')
  })
  it('converts administrator monetary figures to CNY at the fixed 6.2 rate', () => {
    const wrapper = mount(EndpointDistributionChart, { props: { endpointStats } })
    expect(wrapper.findAll('th')).toHaveLength(5)
    expect(wrapper.text()).toContain('¥28.3204')
    expect(wrapper.text()).toContain('¥35.2092')
    expect(wrapper.text()).not.toContain('$')
  })
})
