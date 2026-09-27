import { mount, flushPromises } from '@vue/test-utils'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { defineComponent, ref } from 'vue'
import BusinessAnalyticsPanel from '../BusinessAnalyticsPanel.vue'
import BusinessPanelNavigation, { type BusinessTab } from '../BusinessPanelNavigation.vue'
import type { BusinessOverview } from '@/api/admin/business'

const api = vi.hoisted(() => ({
  overview: vi.fn(), records: vi.fn(), configuration: vi.fn(), pending: vi.fn(), trace: vi.fn(),
}))
vi.mock('@/api/admin/business', () => ({ businessAPI: api }))
vi.mock('@/api/admin/dashboard', () => ({ getBusinessAnalytics: vi.fn() }))
const Dialog = defineComponent({ props: ['show'], template: '<div v-if="show"><slot /></div>' })
function overview(overrides: Partial<BusinessOverview> = {}): BusinessOverview {
  return {
    start_at: '2026-08-01T00:00:00Z', end_at: '2026-09-01T00:00:00Z', enabled_at: '2026-08-01T00:00:00Z', updated_at: '2026-08-31T00:00:00Z', revision: 1, currency: 'CNY', timezone: 'Asia/Shanghai',
    cash_in_cny: '180', cash_refund_cny: '0', cash_out_cny: '6', cash_net_cny: '174',
    recognized_revenue_cny: '18', usage_cost_cny: '6', fixed_cost_cny: '0', operating_cost_cny: '0', known_profit_cny: '12', profit_cny: null, margin: null,
    gift_granted_credits: '100', gift_used_credits: '10', gift_cost_cny: '2', discount_cny: '20',
    wallet_deferred_cny: '162', subscription_deferred_cny: '0', prepaid_supplier_cny: '0', unknown_wallet_credits: '10', prepaid_expense_cny: '0', gift_outstanding_credits: '90',
    quality: { cash: 'confirmed', revenue: 'unknown', cost: 'confirmed', attribution: 'direct', missing_count: 1, estimated_count: 0 },
    entries: [], daily: [], groups: [], models: [], accounts: [], plans: [], ...overrides,
  }
}
function mountPanel() {
  return mount(BusinessAnalyticsPanel, { props: { startDate: '2026-08-01', endDate: '2026-08-31' }, global: { stubs: { BaseDialog: Dialog, BusinessRecordDialog: true, BusinessImportDialog: true, BusinessConfigurationPanel: true, BusinessLedgerCharts: true } } })
}
beforeEach(() => {
  vi.clearAllMocks()
  api.overview.mockResolvedValue(overview())
  api.records.mockResolvedValue([{ id: 1, event_type: 'expense', occurred_at: '2026-08-01T00:00:00Z', payload: { amount_cny: '99', notes: '服务器凭据' } }])
  api.configuration.mockResolvedValue({ pools: [], rules: [], bindings: [], enabled_at: '', timezone: 'Asia/Shanghai' })
  api.pending.mockResolvedValue([])
  api.trace.mockResolvedValue([])
})
describe('人民币经营账', () => {
  it('外置滑块切换面板，面板内跳转同步更新滑块', async () => {
    const Host = defineComponent({
      components: { BusinessAnalyticsPanel, BusinessPanelNavigation },
      setup: () => ({ activeTab: ref<BusinessTab>('overview') }),
      template: '<BusinessPanelNavigation v-model="activeTab" /><BusinessAnalyticsPanel v-model:active-tab="activeTab" start-date="2026-08-01" end-date="2026-08-31" :show-header="false" :show-tabs="false" />',
    })
    const wrapper = mount(Host, { global: { stubs: { BaseDialog: Dialog, BusinessRecordDialog: true, BusinessImportDialog: true, BusinessConfigurationPanel: true, BusinessLedgerCharts: true } } })
    await flushPromises()
    expect(wrapper.findAll('nav[aria-label="经营分析子页"]')).toHaveLength(1)
    const navigation = wrapper.get('nav')
    await navigation.findAll('button').find(b => b.text() === '收支与成本台账')!.trigger('click')
    expect(wrapper.text()).toContain('服务器凭据')
    expect(navigation.get('[aria-current="page"]').text()).toBe('收支与成本台账')
    await navigation.findAll('button').find(b => b.text() === '经营总览')!.trigger('click')
    await wrapper.findAll('button').find(b => b.text() === '查看并补录')!.trigger('click')
    expect(wrapper.text()).toContain('期初与账单调整')
    expect(navigation.get('[aria-current="page"]').text()).toBe('核对与配置')
    wrapper.unmount()
  })
  it('汇总失败时台账与三个录入入口仍可使用', async () => {
    api.overview.mockRejectedValue(new Error('汇总数据库超时'))
    const wrapper = mountPanel(); await flushPromises()
    for (const label of ['登记收款', '登记采购', '录入费用']) expect(wrapper.findAll('button').some(b => b.text() === label && !b.attributes('disabled'))).toBe(true)
    await wrapper.findAll('button').find(b => b.text() === '收支与成本台账')!.trigger('click')
    expect(wrapper.text()).toContain('服务器凭据')
    await wrapper.findAll('button').find(b => b.text() === '录入费用')!.trigger('click')
    expect(wrapper.findComponent({ name: 'BusinessRecordDialog' }).exists()).toBe(true)
  })
  it('缺数据时只显示已知差额，不标完整利润', async () => {
    const wrapper = mountPanel(); await flushPromises()
    expect(wrapper.text()).toContain('利润待核对')
    expect(wrapper.text()).toContain('不能视为完整利润')
    expect(wrapper.text()).not.toContain('已定价利润')
    expect(wrapper.text()).toContain('¥12.00')
  })
  it('现金与利润视图使用各自的服务端金额', async () => {
    const wrapper = mountPanel(); await flushPromises()
    await wrapper.findAll('button').find(b => b.text() === '现金收支')!.trigger('click')
    expect(wrapper.text()).toContain('¥174.00')
    expect(wrapper.text()).toContain('实际退款')
  })
  it('分组行可以过滤到对应收入成本并追溯', async () => {
    api.overview.mockResolvedValue(overview({
      groups: [{ key: '7', name: '订阅分组', revenue_cny: '18', cost_cny: '6', profit_cny: '12', missing_count: 0, allocation: 'allocated' }],
      entries: [
        { id: '1', event_id: 88, occurred_at: '2026-08-01T00:00:00Z', kind: 'wallet_revenue', user_id: 1, account_id: 2, group_id: 7, plan_id: 0, model: 'gpt-test', amount_cny: '18', credits: '30', quality: 'confirmed', detail: {} },
        { id: '2', event_id: 89, occurred_at: '2026-08-01T00:00:00Z', kind: 'wallet_revenue', user_id: 1, account_id: 2, group_id: 8, plan_id: 0, model: 'other-model', amount_cny: '5', credits: '5', quality: 'confirmed', detail: {} },
      ],
    }))
    const wrapper = mountPanel(); await flushPromises()
    await wrapper.findAll('button').find(b => b.text() === '盈利明细')!.trigger('click')
    await wrapper.findAll('button').find(b => b.text() === '订阅分组')!.trigger('click')
    expect(wrapper.text()).toContain('gpt-test'); expect(wrapper.text()).not.toContain('other-model')
    await wrapper.findAll('button').find(b => b.text().includes('#88'))!.trigger('click')
    expect(api.trace).toHaveBeenCalledWith(88)
  })
  it('快速切日期不会被较旧的请求覆盖', async () => {
    let resolveOld!: (value: BusinessOverview) => void
    api.overview.mockReturnValueOnce(new Promise<BusinessOverview>(resolve => { resolveOld = resolve }))
    const wrapper = mountPanel()
    await wrapper.setProps({ endDate: '2026-09-01' }); await flushPromises()
    resolveOld(overview({ known_profit_cny: '9999' })); await flushPromises()
    expect(wrapper.text()).not.toContain('9,999')
  })
  it('未核实成本显示缺失状态，同时保留已入账金额', async () => {
    const report = overview()
    api.overview.mockResolvedValue(overview({ quality: { ...report.quality, cost: 'unknown' } }))
    const wrapper = mountPanel(); await flushPromises()
    const cost = wrapper.get('button[aria-label="上游消耗成本，查看明细"]')
    expect(cost.get('strong').text()).toBe('—')
    expect(cost.text()).toContain('已入账 ¥6.00')

    api.overview.mockResolvedValue(overview({ usage_cost_cny: '0', fixed_cost_cny: '3.10', operating_cost_cny: '2.20' }))
    await wrapper.setProps({ endDate: '2026-09-01' }); await flushPromises()
    expect(cost.get('strong').text()).toBe('¥0.00')
    expect(wrapper.get('button[aria-label="账号及经营费用，查看明细"]').get('strong').text()).toBe('¥5.30')
  })
  it('空趋势提供收款与台账入口，有零金额凭据时仍显示趋势', async () => {
    const wrapper = mountPanel(); await flushPromises()
    expect(wrapper.text()).toContain('本期暂无已入账记录')
    await wrapper.findAll('button').find(b => b.text() === '查看收支台账')!.trigger('click')
    expect(wrapper.text()).toContain('服务器凭据')
    await wrapper.findAll('button').find(b => b.text() === '经营总览')!.trigger('click')
    api.overview.mockResolvedValue(overview({
      entries: [{ id: 'zero', event_id: 8, occurred_at: '2026-08-01T00:00:00Z', kind: 'usage_cost', user_id: 1, account_id: 1, group_id: 1, plan_id: 0, model: 'test', amount_cny: '0', credits: '1', quality: 'confirmed', detail: {} }],
      daily: [{ key: '2026-08-01', name: '08-01', revenue_cny: '0', cost_cny: '0', profit_cny: '0', missing_count: 0, allocation: 'direct' }],
    }))
    await wrapper.setProps({ endDate: '2026-09-01' }); await flushPromises()
    expect(wrapper.text()).not.toContain('本期暂无已入账记录')
    expect(wrapper.findComponent({ name: 'BusinessLedgerCharts' }).exists()).toBe(true)
    await wrapper.findAll('button').find(b => b.text() === '现金收支')!.trigger('click')
    expect(wrapper.text()).toContain('本期暂无已入账记录')
  })
  it('补录指引跳转至来源区，费用指引打开现有录入表单', async () => {
    const scroll = vi.fn()
    const original = HTMLElement.prototype.scrollIntoView
    HTMLElement.prototype.scrollIntoView = scroll
    try {
      const wrapper = mountPanel(); await flushPromises()
      await wrapper.findAll('button').find(b => b.text() === '去补录')!.trigger('click'); await flushPromises()
      expect(wrapper.text()).toContain('待核对与期初来源')
      expect(scroll).toHaveBeenCalledWith({ block: 'start' })
      await wrapper.findAll('button').find(b => b.text() === '经营总览')!.trigger('click')
      await wrapper.findAll('button').find(b => b.text() === '去录入')!.trigger('click')
      expect(wrapper.findComponent({ name: 'BusinessRecordDialog' }).props('type')).toBe('expense')
    } finally {
      HTMLElement.prototype.scrollIntoView = original
    }
  })
  it('余额组成显示未知与赠送额度，不混入付费人民币价值', async () => {
    const wrapper = mountPanel(); await flushPromises()
    await wrapper.findAll('button').find(b => b.text() === '查看组成')!.trigger('click')
    expect(wrapper.text()).toContain('来源未知余额 10 额度')
    expect(wrapper.text()).toContain('未使用赠送余额 90 额度')
    expect(wrapper.text()).toContain('两者均不计为已确认的人民币价值')
    expect(wrapper.text()).toContain('¥162.00')
  })
})
