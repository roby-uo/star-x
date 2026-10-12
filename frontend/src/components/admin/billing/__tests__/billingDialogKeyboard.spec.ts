import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import CostProfileEditor from '../CostProfileEditor.vue'
import { account, profile } from './fixtures'

describe('billing editor keyboard behavior', () => {
  it('keeps tab navigation inside the editor and restores the opener and scrolling on close', async () => {
    const beforeOverflow = document.body.style.overflow
    document.body.style.overflow = 'auto'
    const opener = document.createElement('button')
    document.body.appendChild(opener)
    opener.focus()
    const wrapper = mount(CostProfileEditor, { attachTo: document.body, props: { existing: profile(), accounts: [account()], saving: false, error: '' } })
    try {
      await nextTick()
      const closeButton = wrapper.get('[aria-label="关闭成本配置"]').element as HTMLButtonElement
      const saveButton = wrapper.get('button[form="cost-profile"]').element as HTMLButtonElement
      expect(document.activeElement).toBe(closeButton)
      expect(document.body.style.overflow).toBe('hidden')
      saveButton.focus()
      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', cancelable: true }))
      expect(document.activeElement).toBe(closeButton)
      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', shiftKey: true, cancelable: true }))
      expect(document.activeElement).toBe(saveButton)
    } finally {
      wrapper.unmount()
    }
    expect(document.activeElement).toBe(opener)
    expect(document.body.style.overflow).toBe('auto')
    opener.remove()
    document.body.style.overflow = beforeOverflow
  })

  it('blocks Escape while saving, then allows it once the save completes', async () => {
    const wrapper = mount(CostProfileEditor, { props: { existing: profile(), accounts: [account()], saving: true, error: '' } })
    try {
      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', cancelable: true }))
      expect(wrapper.emitted('close')).toBeUndefined()
      await wrapper.setProps({ saving: false })
      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', cancelable: true }))
      expect(wrapper.emitted('close')).toHaveLength(1)
    } finally {
      wrapper.unmount()
    }
  })
})
