import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import AccountBulkActionsBar from '../AccountBulkActionsBar.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key })
}))

describe('AccountBulkActionsBar automatic rotation', () => {
  it('shows the countdown beside batch update and emits a toggle request', async () => {
    const wrapper = mount(AccountBulkActionsBar, {
      props: {
        selectedIds: [],
        autoRotationEnabled: true,
        autoRotationCountdown: 'A serving; 04:59:59 remaining'
      }
    })

    expect(wrapper.get('[data-test="auto-rotation-countdown"]').text()).toBe('A serving; 04:59:59 remaining')
    const buttons = wrapper.findAll('button')
    expect(buttons[0].attributes('data-test')).toBe('auto-rotation-toggle')
    expect(buttons[1].text()).toBe('admin.accounts.bulkEdit.submit')

    await wrapper.get('[data-test="auto-rotation-toggle"]').trigger('click')
    expect(wrapper.emitted('toggle-auto-rotation')).toHaveLength(1)
  })
})
