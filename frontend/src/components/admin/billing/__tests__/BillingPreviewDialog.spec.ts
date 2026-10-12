import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import BillingPreviewDialog from '../BillingPreviewDialog.vue'
import { account, deferred, group, preview, profile } from './fixtures'
import type { BillingPreview } from '@/api/admin/billingManagement'
import { money } from '../billingManagement'

const { post, get, put } = vi.hoisted(() => ({ post: vi.fn(), get: vi.fn(), put: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { post, get, put } }))

const mounted: VueWrapper[] = []
afterEach(() => { mounted.forEach(wrapper => wrapper.unmount()); mounted.length = 0; vi.unstubAllGlobals() })
beforeEach(() => { vi.clearAllMocks(); post.mockResolvedValue({ data: preview() }); vi.stubGlobal('fetch', vi.fn()) })

async function dialog() {
  const wrapper = mount(BillingPreviewDialog, { props: { model: 'MiniMax-M3', accounts: [account()], groups: [group()], profiles: [profile()], models: ['MiniMax-M3'] } })
  mounted.push(wrapper)
  await field(wrapper, '调用分组', 'select').setValue('7')
  return wrapper
}
function field(wrapper: VueWrapper, label: string, selector = 'input') {
  const item = wrapper.findAll('label').find(row => row.text().startsWith(label))
  if (!item) throw new Error(`Missing input: ${label}`)
  return item.get(selector)
}

describe('BillingPreviewDialog estimate boundaries', () => {
  it('sends one administrative preview request and never invokes a model or gateway', async () => {
    const wrapper = await dialog()
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(post).toHaveBeenCalledTimes(1)
    expect(post).toHaveBeenCalledWith('/admin/billing-management/preview', expect.objectContaining({ model: 'MiniMax-M3', group_id: 7, modality: 'chat', input_tokens: 1000, output_tokens: 200 }))
    expect(get).not.toHaveBeenCalled()
    expect(put).not.toHaveBeenCalled()
    expect(fetch).not.toHaveBeenCalled()
    expect(wrapper.find('[aria-label="算费结果"]').exists()).toBe(true)
  })

  it('renders unknown cost as pending and does not calculate a zero cost or margin', async () => {
    post.mockResolvedValue({ data: preview({ cost: { ...preview().cost, status: 'unknown', amount: null, amount_cny: null, reason: '未登记采购成本' } }) })
    const wrapper = await dialog()
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    const result = wrapper.get('[aria-label="算费结果"]').text()
    expect(result).toContain('待配置')
    expect(result).toContain('未登记采购成本')
    expect(result).not.toContain(money(0))
    expect(result).not.toContain('扣除已配置采购成本后')
    expect(result).not.toContain('到账美元额度')
  })

  it('does not allocate a monthly procurement charge as a single-request margin', async () => {
    post.mockResolvedValue({ data: preview({ cost: { ...preview().cost, status: 'period_only', amount: null, amount_cny: null, period_cost: 199, reason: '按月记录' } }) })
    const wrapper = await dialog()
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[aria-label="算费结果"]').text()).toContain(money(199))
    expect(wrapper.text()).not.toContain('按充值比例试算收支')
  })

  it('uses a supplied yuan amount directly and converts a legacy dollar amount only once', async () => {
    post.mockResolvedValueOnce({ data: preview({ sale: { ...preview().sale, amount_cny: 2.7776 } }) })
    const wrapper = await dialog()
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.text()).toContain(money(2.7776))
    expect(wrapper.text()).not.toContain(money(2.7776, 'USD'))
    expect(wrapper.text()).not.toContain('到账美元额度')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(wrapper.text()).toContain(money(0.448, 'USD'))
  })

  it('ignores an old successful response after the form changes', async () => {
    const pending = deferred<{ data: BillingPreview }>()
    post.mockReturnValue(pending.promise)
    const wrapper = await dialog()
    await wrapper.get('form').trigger('submit')
    await field(wrapper, '未缓存输入 tokens').setValue('5000')
    pending.resolve({ data: preview() })
    await flushPromises()
    expect(wrapper.find('[aria-label="算费结果"]').exists()).toBe(false)
  })

  it('invalidates the old request even when a form edit is later reverted', async () => {
    const pending = deferred<{ data: BillingPreview }>()
    post.mockReturnValue(pending.promise)
    const wrapper = await dialog()
    await wrapper.get('form').trigger('submit')
    await field(wrapper, '未缓存输入 tokens').setValue('5000')
    await field(wrapper, '未缓存输入 tokens').setValue('1000')
    pending.resolve({ data: preview() })
    await flushPromises()
    expect(wrapper.find('[aria-label="算费结果"]').exists()).toBe(false)
  })

  it('ignores an old error after the form changes', async () => {
    const pending = deferred<{ data: BillingPreview }>()
    post.mockReturnValue(pending.promise)
    const wrapper = await dialog()
    await wrapper.get('form').trigger('submit')
    await field(wrapper, '未缓存输入 tokens').setValue('5000')
    pending.reject({ status: 400, message: '旧请求模型不支持' })
    await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
  })

  it('passes image token subcounts as separate values to the administrative preview', async () => {
    const wrapper = await dialog()
    await field(wrapper, '模型').setValue('gpt-image-2')
    await field(wrapper, '未缓存输入 tokens').setValue('100')
    await field(wrapper, '输出 tokens').setValue('80')
    await field(wrapper, '其中图像输入 tokens').setValue('20')
    await field(wrapper, '其中图像输出 tokens').setValue('40')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(post).toHaveBeenCalledWith('/admin/billing-management/preview', expect.objectContaining({ modality: 'image', model: 'gpt-image-2', input_tokens: 100, output_tokens: 80, image_input_tokens: 20, image_output_tokens: 40 }))
    expect(fetch).not.toHaveBeenCalled()
  })

  it('checks visual input pricing on a language model without adding tokens twice', async () => {
    const wrapper = await dialog()
    await field(wrapper, '未缓存输入 tokens').setValue('1000')
    await field(wrapper, '其中图像输入 tokens').setValue('200')
    post.mockRejectedValueOnce({ message: '图片输入基准价尚未确认' })
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(post).toHaveBeenCalledWith('/admin/billing-management/preview', expect.objectContaining({ modality: 'chat', input_tokens: 1000, image_input_tokens: 200 }))
    expect(wrapper.get('[role="alert"]').text()).toContain('图片输入基准价尚未确认')
    expect(wrapper.find('[aria-label="算费结果"]').exists()).toBe(false)
    expect(fetch).not.toHaveBeenCalled()
  })
})
