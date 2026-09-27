import { mount, flushPromises } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import BusinessCostRepairDialog from '../BusinessCostRepairDialog.vue'
const api = vi.hoisted(() => ({ previewRepair: vi.fn(), repair: vi.fn() }))
vi.mock('@/api/admin/business', () => ({ businessAPI: api }))
const Dialog = defineComponent({ template: '<div><slot /><slot name="footer" /></div>' })
function render() { return mount(BusinessCostRepairDialog, { props: { startDate: '2026-09-01', endDate: '2026-09-27' }, global: { stubs: { BaseDialog: Dialog } } }) }
const preview = { through_event_id: 200, fingerprint: 'preview-fingerprint', repairable_count: 100, missing_binding_count: 3, missing_rule_count: 2, protected_count: 10 }
beforeEach(() => { vi.clearAllMocks(); api.previewRepair.mockResolvedValue(preview); api.repair.mockResolvedValue({ id: 9, status: 'queued', count: 100 }) })
describe('成本补算预览', () => {
  it('预览和说明齐备才能执行，明确保留缺失和受保护数量', async () => {
    const wrapper = render()
    const confirm = () => wrapper.findAll('button').find(b => b.text() === '确认并后台补算')!
    expect(confirm().attributes('disabled')).toBeDefined()
    await wrapper.findAll('button').find(b => b.text() === '预览影响范围')!.trigger('click'); await flushPromises()
    expect(wrapper.text()).toContain('可补算 100 次请求')
    expect(wrapper.text()).toContain('仍缺价格 2 次')
    expect(wrapper.text()).toContain('受实际成本或账单保护 10 次')
    expect(confirm().attributes('disabled')).toBeDefined()
    await wrapper.get('textarea').setValue('合同凭据')
    await confirm().trigger('click'); await flushPromises()
    expect(api.repair).toHaveBeenCalledWith(expect.objectContaining({ through_event_id: 200, fingerprint: preview.fingerprint, notes: '合同凭据' }))
    expect(wrapper.emitted('queued')).toHaveLength(1)
  })
  it('改变日期会使旧预览失效', async () => {
    const wrapper = render()
    await wrapper.findAll('button').find(b => b.text() === '预览影响范围')!.trigger('click'); await flushPromises()
    await wrapper.findAll('input')[0].setValue('2026-09-02T00:00')
    expect(wrapper.find('[aria-label="补算预览"]').exists()).toBe(false)
    expect(wrapper.findAll('button').find(b => b.text() === '确认并后台补算')!.attributes('disabled')).toBeDefined()
  })
  it('网络失败重试复用同一批次幂等键', async () => {
    api.repair.mockRejectedValueOnce(new Error('连接中断'))
    const wrapper = render()
    await wrapper.findAll('button').find(b => b.text() === '预览影响范围')!.trigger('click'); await flushPromises()
    await wrapper.get('textarea').setValue('凭据')
    await wrapper.findAll('button').find(b => b.text() === '确认并后台补算')!.trigger('click'); await flushPromises()
    const first = api.repair.mock.calls[0][0]
    await wrapper.findAll('button').find(b => b.text() === '确认并后台补算')!.trigger('click'); await flushPromises()
    expect(api.repair.mock.calls[1][0]).toEqual(first)
    expect(wrapper.emitted('queued')).toHaveLength(1)
  })
})
