import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import BusinessRecordDialog from '../BusinessRecordDialog.vue'
import type { BusinessEvent, BusinessRecordInput } from '@/api/admin/business'

const api = vi.hoisted(() => ({ record: vi.fn() }))
vi.mock('@/api/admin/business', () => ({ businessAPI: api }))
const Dialog = defineComponent({ template: '<div><slot /><slot name="footer" /></div>' })
function mountDialog(type = 'receipt', target?: BusinessEvent) {
  return mount(BusinessRecordDialog, {
    props: { type, pools: [], target },
    global: { stubs: { BaseDialog: Dialog, BusinessEntitySelect: true, Select: true } },
  })
}
beforeEach(() => { vi.clearAllMocks(); api.record.mockResolvedValue({ id: 1 }) })
describe('经营凭据录入', () => {
  it('失败重试保留相同幂等键与高精度金额，修改内容后换键', async () => {
    api.record.mockRejectedValueOnce(new Error('网络中断')).mockRejectedValueOnce(new Error('网络中断'))
    const wrapper = mountDialog()
    await wrapper.find('input[inputmode="decimal"]').setValue('80.12345678')
    await wrapper.find('textarea').setValue('线下成交凭据')
    await wrapper.find('form').trigger('submit'); await flushPromises()
    const first = api.record.mock.calls[0][0] as BusinessRecordInput
    expect(first.payload.amount_cny).toBe('80.12345678')
    expect(first.payload.apply_balance).toBe(false)
    expect(wrapper.text()).toContain('网络中断')
    expect(wrapper.emitted('saved')).toBeUndefined()
    await wrapper.find('form').trigger('submit'); await flushPromises()
    expect(api.record.mock.calls[1][0]).toEqual(first)
    await wrapper.find('input[inputmode="decimal"]').setValue('90')
    await wrapper.find('form').trigger('submit'); await flushPromises()
    expect(api.record.mock.calls[2][0].idempotency_key).not.toBe(first.idempotency_key)
    expect(wrapper.emitted('saved')).toHaveLength(1)
  })
  it('服务期无效时不提交费用', async () => {
    const wrapper = mountDialog('expense')
    await wrapper.find('input[inputmode="decimal"]').setValue('300')
    await wrapper.find('textarea').setValue('账号月租')
    await wrapper.findAll('input[type="checkbox"]')[0].setValue(true)
    await wrapper.find('form').trigger('submit'); await flushPromises()
    expect(api.record).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('服务期结束必须晚于开始')
  })
  it('终止预付费用仅提交原始凭据，不要求再次录入金额', async () => {
    const target: BusinessEvent = { id: 42, source_key: 'manual:expense', event_type: 'expense', transaction_id: 1, user_id: 0, occurred_at: '2026-08-01T00:00:00Z', recorded_at: '2026-08-01T00:00:00Z', actor_id: 1, reverses_id: 0, payload: {} }
    const wrapper = mountDialog('expense_stop', target)
    await wrapper.find('textarea').setValue('账号提前停用')
    await wrapper.find('form').trigger('submit'); await flushPromises()
    expect(api.record).toHaveBeenCalledOnce()
    expect(api.record.mock.calls[0][0].payload.source_event_id).toBe(42)
    expect(wrapper.emitted('saved')).toHaveLength(1)
  })
})
