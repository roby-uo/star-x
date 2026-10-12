import { afterEach, describe, expect, it } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import CostProfileEditor from '../CostProfileEditor.vue'
import { account, profile } from './fixtures'

const mounted: VueWrapper[] = []
afterEach(() => { mounted.forEach(wrapper => wrapper.unmount()); mounted.length = 0 })

function editor(existing = profile()) {
  const wrapper = mount(CostProfileEditor, { props: { existing, accounts: [account(1), account(2, 'oauth')], saving: false, error: '' } })
  mounted.push(wrapper)
  return wrapper
}
function field(wrapper: VueWrapper, label: string, selector = 'input') {
  const item = wrapper.findAll('label').find(row => row.text().startsWith(label))
  if (!item) throw new Error(`Missing input: ${label}`)
  return item.get(selector)
}

describe('CostProfileEditor procurement rules', () => {
  it('edits procurement cost independently of account credentials and preserves the input profile', async () => {
    const original = profile()
    const snapshot = JSON.parse(JSON.stringify(original))
    const wrapper = editor(original)
    expect(wrapper.text()).toContain('API 接入')
    expect(wrapper.text()).toContain('账号授权')
    await field(wrapper, '方案名称').setValue('修改后的 API 采购')
    await field(wrapper, '采购原币', 'select').setValue('USD')
    await field(wrapper, '单价采购折扣').setValue('0.8')
    await wrapper.get('form').trigger('submit')

    const saved = wrapper.emitted('save')?.[0]?.[0] as ReturnType<typeof profile>
    expect(saved).toMatchObject({ name: '修改后的 API 采购', currency: 'USD', discount_multiplier: 0.8, account_ids: [1], purchase_mode: 'payg' })
    expect(saved.prices[0]?.input_price).toBeCloseTo(2 / 6.2)
    expect(saved.prices[0]?.output_price).toBeCloseTo(8 / 6.2)
    expect(saved.prices[0]).toMatchObject({ cache_read_price: 0, cache_write_price: 0 })
    expect(saved).not.toHaveProperty('rate_multiplier')
    expect(saved).not.toHaveProperty('balance_recharge_multiplier')
    expect(saved).not.toHaveProperty('credentials')
    expect(original).toEqual(snapshot)
  })

  it('requires an explicit value for every token price instead of treating missing cost as zero', async () => {
    const original = profile()
    original.prices[0].input_price = null
    const wrapper = editor(original)
    await wrapper.get('form').trigger('submit')
    expect(wrapper.emitted('save')).toBeUndefined()
    expect(wrapper.get('[role="alert"]').text()).toContain('成本单价')
    await field(wrapper, '输入（').setValue('0')
    await wrapper.get('form').trigger('submit')
    expect((wrapper.emitted('save')?.[0]?.[0] as ReturnType<typeof profile>).prices[0]?.input_price).toBe(0)
  })

  it('preserves disabled state and expired validity when editing an unrelated field', async () => {
    const original = profile({ enabled: false, valid_from: '2025-01-01T00:00:00Z', valid_until: '2025-02-01T00:00:00Z' })
    const wrapper = editor(original)
    await field(wrapper, '方案名称').setValue('保留已停用的历史方案')
    await wrapper.get('form').trigger('submit')
    const saved = wrapper.emitted('save')?.[0]?.[0] as ReturnType<typeof profile>
    expect(saved.enabled).toBe(false)
    expect(Date.parse(saved.valid_from!)).toBe(Date.parse(original.valid_from!))
    expect(Date.parse(saved.valid_until!)).toBe(Date.parse(original.valid_until!))
  })

  it('keeps typed values after a version conflict is passed back from the page', async () => {
    const wrapper = editor()
    await field(wrapper, '方案名称').setValue('尚未保存的采购名称')
    await field(wrapper, '输入（').setValue('9.2')
    await wrapper.setProps({ error: '配置版本冲突，请刷新后重试。' })
    expect((field(wrapper, '方案名称').element as HTMLInputElement).value).toBe('尚未保存的采购名称')
    expect((field(wrapper, '输入（').element as HTMLInputElement).value).toBe('9.2')
    expect(wrapper.get('[role="alert"]').text()).toContain('版本冲突')
  })

  it('records monthly costs separately and retains an explicit account reference price', async () => {
    const wrapper = editor()
    await field(wrapper, '采购模式', 'select').setValue('subscription')
    await field(wrapper, '周期实付费用').setValue('199')
    await wrapper.get('form').trigger('submit')
    expect(wrapper.emitted('save')?.[0]?.[0]).toMatchObject({ purchase_mode: 'subscription', period_cost: 199 })
    expect((wrapper.emitted('save')?.[0]?.[0] as ReturnType<typeof profile>).prices).toHaveLength(1)
  })

  it('preserves existing validity precision when only the profile name changes', async () => {
    const original = profile({ valid_from: '2025-01-01T00:00:23.123Z', valid_until: '2025-02-01T00:00:45.456Z' })
    const wrapper = editor(original)
    await field(wrapper, '方案名称').setValue('仅修改名称')
    await wrapper.get('form').trigger('submit')
    const saved = wrapper.emitted('save')?.[0]?.[0] as ReturnType<typeof profile>
    expect(Date.parse(saved.valid_from!)).toBe(Date.parse(original.valid_from!))
    expect(Date.parse(saved.valid_until!)).toBe(Date.parse(original.valid_until!))
  })

  it('saves distinct image token unit prices without folding them into text prices', async () => {
    const wrapper = editor()
    await field(wrapper, '模型类型', 'select').setValue('image')
    await field(wrapper, '计费单位', 'select').setValue('per_million_tokens')
    await field(wrapper, '图像输入（').setValue('2.4')
    await field(wrapper, '图像输出（').setValue('6.5')
    await wrapper.get('form').trigger('submit')
    const saved = wrapper.emitted('save')?.[0]?.[0] as ReturnType<typeof profile>
    expect(saved.prices[0]).toMatchObject({ modality: 'image', billing_unit: 'per_million_tokens', input_price: 2, output_price: 8, image_input_price: 2.4, image_output_price: 6.5 })
  })

  it('edits a dollar procurement price in yuan and preserves its source unit on save', async () => {
    const original = profile({ currency: 'USD' })
    const wrapper = editor(original)
    expect(Number((field(wrapper, '输入（').element as HTMLInputElement).value)).toBeCloseTo(12.4)
    await field(wrapper, '输入（').setValue('31')
    await wrapper.get('form').trigger('submit')
    const saved = wrapper.emitted('save')?.[0]?.[0] as ReturnType<typeof profile>
    expect(saved.currency).toBe('USD')
    expect(saved.prices[0]?.input_price).toBeCloseTo(5)
    expect(original.prices[0]?.input_price).toBe(2)
  })
})
