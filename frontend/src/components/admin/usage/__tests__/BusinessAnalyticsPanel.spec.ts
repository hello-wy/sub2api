import { mount, flushPromises } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import zh from '@/i18n/locales/zh/index'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import BusinessAnalyticsPanel from '../BusinessAnalyticsPanel.vue'
import { businessAPI, type BusinessOverview, type BusinessSettings } from '@/api/admin/business'
vi.mock('@/api/admin/business', () => ({ businessAPI: { overview:vi.fn(), settings:vi.fn(), initialize:vi.fn(), records:vi.fn(), record:vi.fn(), reverse:vi.fn(), terms:vi.fn(), configureTerm:vi.fn() } }))
const settings: BusinessSettings = { enabled_at:'2026-09-01T00:00:00+08:00', timezone:'Asia/Shanghai', opening_revenue:'200', opening_private_revenue:'50', opening_cost:'100' }
const report = (): BusinessOverview => ({ settings:{...settings}, start_date:'2026-09-01', end_date:'2026-09-28', updated_at:'2026-09-28T12:00:00+08:00', summary:{date:'',balance_revenue:'20',subscription_revenue:'10',other_revenue:'8',manual_cost:'10',gift_cost:'25',revenue:'38',cost:'35',known_profit:'3',profit:'3',missing:0}, daily:[],cumulative_profit:'153',cumulative_known_profit:'153', positions:{balance:'1000',frozen:'7',debt:'3',subscription_value:'90',known_subscription_value:'90',suspended_value:'0',suspended_count:0,active_subscriptions:1,missing_subscriptions:0}, capacity:[],rewards:[{source:'usage_rebate',amount:'25'}],pending_terms:0,manual_receipts:'1000' })
const render = () => {
 const operating = Object.fromEntries(Object.entries(zh.admin.businessAnalytics.operating).map(([k,v]) => [k, () => v]))
 const messages = { zh: { admin: { businessAnalytics: { operating } } } }
 return mount(BusinessAnalyticsPanel, { global: { plugins: [createI18n({legacy:false,locale:'zh',messages})], stubs: {BusinessTrendChart:true} } })
}
let wrappers: ReturnType<typeof render>[] = []
const show = () => { const w=render();wrappers.push(w);return w }
beforeEach(() => {vi.resetAllMocks();vi.mocked(businessAPI.settings).mockResolvedValue({...settings});vi.mocked(businessAPI.overview).mockResolvedValue(report());vi.mocked(businessAPI.records).mockResolvedValue([]);vi.mocked(businessAPI.terms).mockResolvedValue([])})
afterEach(() => {wrappers.forEach(w=>w.unmount());wrappers=[]})
describe('经营统计新口径', () => {
 it('分别展示经营利润、存量与仅收款金额',async()=>{const w=show();await flushPromises();expect(w.get('.profit-metric strong').text()).toContain('3.00');expect(w.get('.cumulative strong').text()).toContain('153.00');expect(w.get('.positions').text()).toContain('1,000.00');expect(w.get('.explanation').text()).toContain('仅作收款记录的线下金额：¥1,000.00');expect(w.text()).not.toContain('成本分摊');expect(w.emitted('dates')?.[0]).toEqual(['2026-09-01','2026-09-28'])})
 it('缺失订阅口径时不把已知差额冒充完整利润',async()=>{const r=report();r.summary.profit=null;r.summary.missing=1;r.pending_terms=1;r.cumulative_profit=null;r.positions.subscription_value=null;vi.mocked(businessAPI.overview).mockResolvedValue(r);const w=show();await flushPromises();expect(w.get('.profit-metric strong').text()).toBe('待确认');expect(w.get('.notice').text()).toContain('需要补充');expect(w.get('.positions').text()).toContain('待确认')})
 it('读取期初失败时禁止误启用并提供重试',async()=>{vi.mocked(businessAPI.settings).mockRejectedValue(new Error('网络异常'));const w=show();await flushPromises();expect(w.get('[role=alert]').text()).toContain('网络异常');await w.findAll('.tabs button')[2].trigger('click');expect(w.get('.opening .primary-button').attributes('disabled')).toBeDefined();expect(businessAPI.initialize).not.toHaveBeenCalled()})
 it('初始化需要确认并以十进制字符串保存三个历史总数',async()=>{vi.mocked(businessAPI.settings).mockResolvedValue({...settings,enabled_at:null});const w=show();await flushPromises();await w.get('.welcome button').trigger('click');const inputs=w.findAll('.opening input[type=number]');await inputs[0].setValue('200');await inputs[1].setValue('50');await inputs[2].setValue('100');expect(w.get('.opening .primary-button').attributes('disabled')).toBeDefined();await w.get('.check-label input').setValue(true);await w.get('.opening form').trigger('submit');await flushPromises();expect(businessAPI.initialize).toHaveBeenCalledWith({timezone:'Asia/Shanghai',opening_revenue:'200',opening_private_revenue:'50',opening_cost:'100'})})
 it('保存超时重试复用幂等键，线下收款不会提交为确认收入',async()=>{vi.mocked(businessAPI.record).mockRejectedValueOnce(new Error('网络超时'));const w=show();await flushPromises();await w.get('.actions .primary-button').trigger('click');await w.get('.entry-form select').setValue('receipt');await w.get('[data-test=record-amount]').setValue('100');await w.get('.entry-form input[maxlength="100"]').setValue('线下充值');await w.get('.entry-form').trigger('submit');await flushPromises();const first=vi.mocked(businessAPI.record).mock.calls[0][0];expect(first.kind).toBe('receipt');expect(first.amount).toBe('100');expect(first.source_key).not.toBe('');expect(w.get('.entry-form [role=alert]').text()).toContain('网络超时');await w.get('.entry-form').trigger('submit');await flushPromises();expect(vi.mocked(businessAPI.record).mock.calls[1][0].source_key).toBe(first.source_key)})
 it('快速切换日期时丢弃较早请求的响应',async()=>{let resolveOld:(v:BusinessOverview)=>void=()=>{};vi.mocked(businessAPI.overview).mockReturnValueOnce(new Promise(r=>{resolveOld=r}));const w=show();await flushPromises();const fresh=report();fresh.summary.profit='88';vi.mocked(businessAPI.overview).mockResolvedValueOnce(fresh);await w.setProps({startDate:'2026-09-20',endDate:'2026-09-28'});await flushPromises();resolveOld(report());await flushPromises();expect(w.get('.profit-metric strong').text()).toContain('88.00')})
 it('冲销需要原因且只提交反向记录',async()=>{vi.mocked(businessAPI.records).mockResolvedValue([{id:7,source_key:'cost-7',kind:'cost',amount:'10',date:'2026-09-28',category:'采购',notes:'凭据',reverses_id:null,reversed:false,created_at:'2026-09-28T12:00:00Z'}]);const w=show();await flushPromises();await w.findAll('.tabs button')[1].trigger('click');await w.get('.text-button').trigger('click');await w.get('.reverse-form input').setValue('录入金额有误');await w.get('.reverse-form').trigger('submit');await flushPromises();expect(businessAPI.reverse).toHaveBeenCalledWith(7,'录入金额有误')})
})

describe('经营统计补齐基准和分页', () => {
 it('订阅基准保持已冻结字段并提交补齐依据', async () => {
  vi.mocked(businessAPI.terms).mockResolvedValue([{id:9,user_id:1,group_name:'测试分组',plan_name:'月卡',funding:'paid',amount:'100',quota:null,baseline_units:'20',used_units:'25',starts_at:settings.enabled_at!,ends_at:'2026-12-01T00:00:00Z',opening:true}])
  const w=show();await flushPromises();await w.findAll('.tabs button')[2].trigger('click');await w.get('.term-row > button').trigger('click')
  const numbers=w.findAll('.term-form input[type=number]');expect(numbers[0].attributes('readonly')).toBeDefined();await numbers[1].setValue('1000')
  await w.get('.term-form input[maxlength]').setValue('订单约定额度');await w.get('.term-form').trigger('submit');await flushPromises()
  expect(businessAPI.configureTerm).toHaveBeenCalledWith(9,{amount:'100',quota:'1000',funding:'paid',notes:'订单约定额度'})
 })
 it('账目分页失败后保留已有记录并允许重试',async()=>{
  const records=Array.from({length:50},(_,i)=>({id:100-i,source_key:'record-'+i,kind:'cost' as const,amount:'0.12345678',date:'2026-09-28',category:'测试采购',notes:'',reverses_id:null,reversed:false,created_at:settings.enabled_at!}))
  vi.mocked(businessAPI.records).mockResolvedValueOnce(records).mockRejectedValueOnce(new Error('暂不可用')).mockResolvedValueOnce([])
  const w=show();await flushPromises();await w.findAll('.tabs button')[1].trigger('click');await w.get('.load-more').trigger('click');await flushPromises()
  expect(businessAPI.records).toHaveBeenLastCalledWith(51);expect(w.text()).toContain('暂不可用');expect(w.text()).toContain('¥0.12')
  await w.get('.load-more').trigger('click');await flushPromises();expect(businessAPI.records).toHaveBeenLastCalledWith(51);expect(w.find('.load-more').exists()).toBe(false)
 })
 it('明确区分估算、当前快照与赠送面值',async()=>{const w=show();await flushPromises();expect(w.text()).toContain('经营估算利润');expect(w.text()).toContain('赠送面值不是现金支出');expect(w.text()).toContain('当前快照')})
})
