import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { createPinia } from 'pinia'
import { describe, expect, it } from 'vitest'
import GroupOptionItem from '../GroupOptionItem.vue'

function mountItem(showRate = true) {
  return mount(GroupOptionItem, {
    props: {
      name: 'Open AI',
      platform: 'openai',
      rateMultiplier: 2,
      userRateMultiplier: 1.5,
      peakRateEnabled: true,
      peakStart: '09:00',
      peakEnd: '18:00',
      peakRateMultiplier: 1.2,
      showRate
    },
    global: {
      plugins: [
        createI18n({
          legacy: false,
          locale: 'zh',
          messages: { zh: { common: { peakRateTooltip: () => '高峰倍率' } } }
        }),
        createPinia()
      ],
      stubs: { PlatformIcon: true }
    }
  })
}

describe('GroupOptionItem', () => {
  it('默认显示分组倍率', () => {
    const wrapper = mountItem()

    expect(wrapper.text()).toContain('2x')
    expect(wrapper.text()).toContain('1.5x')
  })

  it('用户侧隐藏默认、专属和高峰倍率', () => {
    const wrapper = mountItem(false)

    expect(wrapper.text()).toContain('Open AI')
    expect(wrapper.text()).not.toContain('2x')
    expect(wrapper.text()).not.toContain('1.5x')
    expect(wrapper.text()).not.toContain('09:00')
    expect(wrapper.text()).not.toContain('18:00')
  })
})
