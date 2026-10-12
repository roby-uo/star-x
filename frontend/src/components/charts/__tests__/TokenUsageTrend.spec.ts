import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { Line } from 'vue-chartjs'

import TokenUsageTrend from '../TokenUsageTrend.vue'

const messages: Record<string, string> = {
  'admin.dashboard.tokenUsageTrend': 'Token Usage Trend',
  'admin.dashboard.noDataAvailable': 'No data available',
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key,
    }),
  }
})

vi.mock('vue-chartjs', () => ({
  Line: {
    props: ['data', 'options'],
    template: '<div class="chart-data">{{ JSON.stringify(data) }}</div>',
  },
}))

describe('TokenUsageTrend', () => {
  const usagePoint = {
    date: '2026-10-12', requests: 1, input_tokens: 10, output_tokens: 20,
    cache_creation_tokens: 0, cache_read_tokens: 0,
  }

  it('keeps user tooltip labels usable when the API omits financial fields', () => {
    const wrapper = mount(TokenUsageTrend, {
      props: { trendData: [usagePoint] as any, showCosts: false },
    })
    const callbacks = wrapper.findComponent(Line).props('options').plugins.tooltip.callbacks

    expect(callbacks.label({ dataset: { label: 'Input' }, raw: 10 })).toBe('Input: 10')
    expect(() => callbacks.footer([{ dataIndex: 0 }])).not.toThrow()
    expect(callbacks.footer([{ dataIndex: 0 }])).toBe('')
    wrapper.unmount()
  })

  it('omits financial tooltip details in user mode even if legacy responses contain costs', () => {
    const wrapper = mount(TokenUsageTrend, {
      props: { trendData: [{ ...usagePoint, cost: 9.87, actual_cost: 8.76 }], showCosts: false },
    })
    const callbacks = wrapper.findComponent(Line).props('options').plugins.tooltip.callbacks

    expect(callbacks.footer([{ dataIndex: 0 }])).toBe('')
    wrapper.unmount()
  })

  it('continues showing financial tooltip details in administrator mode by default', () => {
    const wrapper = mount(TokenUsageTrend, {
      props: { trendData: [{ ...usagePoint, cost: 9.87, actual_cost: 8.76 }] },
    })
    const callbacks = wrapper.findComponent(Line).props('options').plugins.tooltip.callbacks

    expect(callbacks.footer([{ dataIndex: 0 }])).toBe('Actual: $8.76 | Standard: $9.87')
    wrapper.unmount()
  })

  it('does not invent a zero cost or crash when financial fields are absent in administrator mode', () => {
    const wrapper = mount(TokenUsageTrend, { props: { trendData: [usagePoint] as any } })
    const callbacks = wrapper.findComponent(Line).props('options').plugins.tooltip.callbacks

    expect(() => callbacks.footer([{ dataIndex: 0 }])).not.toThrow()
    expect(callbacks.footer([{ dataIndex: 0 }])).toBe('')
    wrapper.unmount()
  })

  it('calculates cache hit rate against all prompt tokens', () => {
    const wrapper = mount(TokenUsageTrend, {
      props: {
        trendData: [
          {
            date: '2026-05-08',
            requests: 1,
            input_tokens: 500,
            output_tokens: 100,
            cache_creation_tokens: 0,
            cache_read_tokens: 1500,
            cost: 0.01,
            actual_cost: 0.005,
          },
        ],
      },
      global: {
        stubs: {
          LoadingSpinner: true,
        },
      },
    })

    const chartData = JSON.parse(wrapper.find('.chart-data').text())
    const hitRateDataset = chartData.datasets.find(
      (ds: any) => ds.label === 'Cache Hit Rate'
    )
    // Hit rate = 1500 / (500 + 1500 + 0) * 100 = 75%
    expect(hitRateDataset.data[0]).toBe(75)
  })

  it('returns 0 hit rate when all prompt tokens are zero', () => {
    const wrapper = mount(TokenUsageTrend, {
      props: {
        trendData: [
          {
            date: '2026-05-08',
            requests: 0,
            input_tokens: 0,
            output_tokens: 0,
            cache_creation_tokens: 0,
            cache_read_tokens: 0,
            cost: 0,
            actual_cost: 0,
          },
        ],
      },
      global: {
        stubs: {
          LoadingSpinner: true,
        },
      },
    })

    const chartData = JSON.parse(wrapper.find('.chart-data').text())
    const hitRateDataset = chartData.datasets.find(
      (ds: any) => ds.label === 'Cache Hit Rate'
    )
    expect(hitRateDataset.data[0]).toBe(0)
  })

  it('includes cache_creation_tokens in denominator for Anthropic models', () => {
    const wrapper = mount(TokenUsageTrend, {
      props: {
        trendData: [
          {
            date: '2026-05-08',
            requests: 1,
            input_tokens: 200,
            output_tokens: 50,
            cache_creation_tokens: 300,
            cache_read_tokens: 500,
            cost: 0.02,
            actual_cost: 0.01,
          },
        ],
      },
      global: {
        stubs: {
          LoadingSpinner: true,
        },
      },
    })

    const chartData = JSON.parse(wrapper.find('.chart-data').text())
    const hitRateDataset = chartData.datasets.find(
      (ds: any) => ds.label === 'Cache Hit Rate'
    )
    // Hit rate = 500 / (200 + 500 + 300) * 100 = 50%
    expect(hitRateDataset.data[0]).toBe(50)
  })
})
