import { mount, flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ConsumptionRevenuePanel from '../ConsumptionRevenuePanel.vue'
import { businessAPI, type ConsumptionRevenueReport } from '@/api/admin/business'
vi.mock('@/api/admin/business', () => ({businessAPI: {consumptionRevenue: vi.fn()}}))
const report = (): ConsumptionRevenueReport => ({start_date:'2026-11-16',end_date:'2026-11-16',timezone:'Asia/Shanghai',updated_at:'2026-11-16T12:00:00+08:00',summary:{balance_cny:'30',subscription_cny:'7.5',total_cny:'37.5',unpriced_credits:'1.49'},actual_credits:'51.49',requests:10,components:[{source:'balance',group_id:null,group_name:'',reason:'priced',requests:5,credits:'40',revenue_cny:'30',unpriced_credits:'0'},{source:'subscription',group_id:33,group_name:'Pro 月卡 A',reason:'priced',requests:3,credits:'10',revenue_cny:'7.5',unpriced_credits:'0'},{source:'subscription',group_id:49,group_name:'不降智 A',reason:'missing_period_quota',requests:2,credits:'1.49',revenue_cny:'0',unpriced_credits:'1.49'}]})
beforeEach(()=>{vi.resetAllMocks();vi.mocked(businessAPI.consumptionRevenue).mockResolvedValue(report())})
describe('人民币消费组成',()=>{
 it('展示原始扣费、各组折算及未计入原因，无需初始化经营账',async()=>{
  const w=mount(ConsumptionRevenuePanel,{props:{startDate:'2026-11-16',endDate:'2026-11-16'}});await flushPromises()
  expect(businessAPI.consumptionRevenue).toHaveBeenCalledWith({start_date:'2026-11-16',end_date:'2026-11-16'})
  expect(w.get('.total strong').text()).toBe('¥37.50')
  expect(w.text()).toContain('$51.49');expect(w.text()).toContain('Pro 月卡 A');expect(w.text()).toContain('缺少明确的整期额度')
  expect(w.get('.pending-note').text()).toContain('$1.49')
  expect(w.get('.pending-row td:last-child').text()).toBe('待折算')
  expect(w.text()).toContain('不依赖下方经营记账是否启用')
  w.unmount()
 })
 it('切换日期后忽略过期请求，失败不保留旧区间金额',async()=>{
  let resolveOld: (v:ConsumptionRevenueReport)=>void=()=>{}
  vi.mocked(businessAPI.consumptionRevenue).mockReturnValueOnce(new Promise(r=>{resolveOld=r}))
  const w=mount(ConsumptionRevenuePanel)
  const fresh=report();fresh.summary.total_cny='99'
  vi.mocked(businessAPI.consumptionRevenue).mockResolvedValueOnce(fresh)
  await w.setProps({startDate:'2026-11-15',endDate:'2026-11-15'});await flushPromises();resolveOld(report());await flushPromises()
  expect(w.get('.total strong').text()).toBe('¥99.00')
  vi.mocked(businessAPI.consumptionRevenue).mockRejectedValueOnce(new Error('统计暂不可用'))
  await w.get('.refresh').trigger('click');await flushPromises()
  expect(w.get('[role=alert]').text()).toContain('统计暂不可用');expect(w.find('.total').exists()).toBe(false)
  await w.get('[role=alert] button').trigger('click');await flushPromises()
  expect(w.get('.total strong').text()).toBe('¥37.50')
  w.unmount()
 })
 it('无消费时显示零元和空态，不显示未折算提示',async()=>{
  const empty=report();empty.summary={balance_cny:'0',subscription_cny:'0',total_cny:'0',unpriced_credits:'0'};empty.actual_credits='0';empty.requests=0;empty.components=[]
  vi.mocked(businessAPI.consumptionRevenue).mockResolvedValueOnce(empty)
  const w=mount(ConsumptionRevenuePanel);await flushPromises()
  expect(w.get('.total strong').text()).toBe('¥0.00');expect(w.get('.empty').text()).toContain('暂无');expect(w.find('.pending-note').exists()).toBe(false)
  w.unmount()
 })
})
