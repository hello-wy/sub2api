import { mount, flushPromises } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest'
import BusinessIssuesPanel from '../BusinessIssuesPanel.vue'
import type { BusinessIssue } from '@/api/admin/business'
const api = vi.hoisted(() => ({ issues: vi.fn(), repairJobs: vi.fn(), pending: vi.fn() }))
vi.mock('@/api/admin/business', () => ({ businessAPI: api }))
const Dialog = defineComponent({ props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' })
function issue(overrides: Partial<BusinessIssue> = {}): BusinessIssue {
  return { key: 'binding:1', kind: 'cost_binding', account_id: 1, pool_id: 0, user_id: 0, name: '供应商账号', model: '', period: '2026-09', affected_count: 48216, source_count: 0, known_amount_cny: '0', first_at: '2026-09-01T00:00:00Z', last_at: '2026-09-24T00:00:00Z', ...overrides }
}
function result(items: BusinessIssue[], extra = {}) { return { items, total: items.length, processing_count: 0, calculation_error: false, revision: 1, updated_at: '', ...extra } }
function render() { return mount(BusinessIssuesPanel, { props: { startDate: '2026-09-01', endDate: '2026-09-27' }, global: { stubs: { BaseDialog: Dialog, BusinessCostRepairDialog: true } } }) }
beforeEach(() => { vi.clearAllMocks(); api.issues.mockResolvedValue(result([issue()])); api.repairJobs.mockResolvedValue([]); api.pending.mockResolvedValue([]) })
afterEach(() => vi.useRealTimers())
describe('按原因处理经营账问题', () => {
  it('类型列区分 OAuth、API Key、用户余额和订阅', async () => {
    api.issues.mockResolvedValue(result([issue({ object_types: ['oauth'] }), issue({ key: 'api', object_types: ['apikey'] }), issue({ key: 'funds', kind: 'funding_source', object_types: ['user_balance', 'user_subscription'] })]))
    const wrapper = render(); await flushPromises()
    expect(wrapper.findAll('th').map(h => h.text())).toContain('类型')
    for (const label of ['OAuth 账号', 'API Key', '用户余额', '用户订阅']) expect(wrapper.text()).toContain(label)
    wrapper.unmount()
  })
  it('大量调用只展示一个问题，并把账号与生效时间带入配置', async () => {
    const wrapper = render(); await flushPromises()
    expect(wrapper.findAll('tbody tr')).toHaveLength(1)
    expect(wrapper.text()).toContain('48,216 条记录')
    expect(wrapper.text()).toContain('1 项待处理问题')
    expect(wrapper.text()).not.toContain('补录凭据')
    await wrapper.findAll('button').find(b => b.text() === '绑定成本池')!.trigger('click')
    expect(wrapper.emitted('configure')?.[0][0]).toMatchObject({ mode: 'binding', accountIDs: [1], effectiveAt: '2026-09-01T00:00:00Z' })
    wrapper.unmount()
  })
  it('多月重复的账号选择去重后批量绑定', async () => {
    api.issues.mockResolvedValue(result([issue(), issue({ key: 'next-month', period: '2026-10' }), issue({ key: 'another', account_id: 2 })]))
    const wrapper = render(); await flushPromises()
    for (const checkbox of wrapper.findAll('input[type="checkbox"]')) await checkbox.setValue(true)
    await wrapper.findAll('button').find(b => b.text() === '批量绑定成本池')!.trigger('click')
    expect(wrapper.emitted('configure')?.[0][0]).toMatchObject({ accountIDs: [1, 2] })
    wrapper.unmount()
  })
  it('资金来源只读取对应用户原始凭据，不打开调用成本表单', async () => {
    api.issues.mockResolvedValue(result([issue({ kind: 'funding_source', user_id: 7, account_id: 0, source_count: 1 })]))
    api.pending.mockResolvedValue([{ id: 21, user_id: 7, event_type: 'opening_unknown', payload: { credits: '100' } }])
    const wrapper = render(); await flushPromises()
    await wrapper.findAll('button').find(b => b.text() === '确认资金来源')!.trigger('click'); await flushPromises()
    expect(api.pending).toHaveBeenCalledWith(0, 7)
    await wrapper.findAll('button').find(b => b.text() === '确认此来源')!.trigger('click')
    expect(wrapper.emitted('annotate')?.[0][0]).toMatchObject({ id: 21, event_type: 'opening_unknown' })
    wrapper.unmount()
  })
  it('账单任务直接打开整期核对，保留成本池信息', async () => {
    api.issues.mockResolvedValue(result([issue({ kind: 'invoice_needed', pool_id: 5, period_start_at: '2026-09-01T00:00:00+08:00', period_end_at: '2026-10-01T00:00:00+08:00' })]))
    const wrapper = render(); await flushPromises()
    await wrapper.findAll('button').find(b => b.text() === '核对整期账单')!.trigger('click')
    expect(wrapper.emitted('record')?.[0]).toEqual(['reconciliation', expect.objectContaining({ pool_id: 5, bill_mode: true, starts_at: '2026-09-01T00:00:00+08:00', ends_at: '2026-10-01T00:00:00+08:00' })])
    wrapper.unmount()
  })
  it('计算中不是人工缺口，完成后自动更新状态并通知报表刷新', async () => {
    vi.useFakeTimers()
    api.issues.mockResolvedValueOnce(result([], { processing_count: 200 })).mockResolvedValue(result([], { revision: 2 }))
    const wrapper = render(); await flushPromises()
    expect(wrapper.text()).toContain('0 项待处理问题')
    expect(wrapper.text()).toContain('后台正在计算 200 条记录')
    await vi.advanceTimersByTimeAsync(5000); await flushPromises()
    expect(wrapper.text()).not.toContain('后台正在计算 200 条记录')
    expect(wrapper.emitted('settled')).toHaveLength(1)
    wrapper.unmount()
  })
  it('快速切换日期时只展示最后一次问题查询', async () => {
    let old!: (value: ReturnType<typeof result>) => void
    api.issues.mockReturnValueOnce(new Promise(resolve => { old = resolve })).mockResolvedValue(result([issue({ name: '新期间' })]))
    const wrapper = render(); await wrapper.setProps({ endDate: '2026-09-28' }); await flushPromises()
    old(result([issue({ name: '旧期间' })])); await flushPromises()
    expect(wrapper.text()).toContain('新期间'); expect(wrapper.text()).not.toContain('旧期间')
    wrapper.unmount()
  })
  it('问题查询失败仍提供重试与补算入口', async () => {
    api.issues.mockRejectedValueOnce(new Error('问题读取超时'))
    const wrapper = render(); await flushPromises()
    expect(wrapper.text()).toContain('问题读取超时')
    await wrapper.findAll('button').find(b => b.text() === '刷新状态')!.trigger('click'); await flushPromises()
    expect(wrapper.text()).toContain('48,216')
    wrapper.unmount()
  })
})
