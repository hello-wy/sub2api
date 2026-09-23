import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import PelicanSchedulePanel from '../PelicanSchedulePanel.vue'
import { scheduledTestsAPI } from '@/api/admin/scheduledTests'
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/api/admin/scheduledTests', () => ({ scheduledTestsAPI: { getResult: vi.fn(), listByAccount: vi.fn(), listResults: vi.fn(), create: vi.fn(), update: vi.fn() } }))
const cfg = { run_for_hours: 24, prompt: 'original prompt', reasoning_effort: 'medium', parallel_count: 2, interval_minutes: 30 }
const plan = { id: 4, account_id: 42, model_id: 'gpt-6-astra', enabled: true, pelican_config: cfg }
function mountPanel() {
  return mount(PelicanSchedulePanel, { props: { accountId: 42, modelId: 'gpt-6-astra', prompt: 'draw a pelican', reasoningEffort: 'medium', parallelCount: 1, disabled: false }, global: { stubs: { Input: true } } })
}
describe('PelicanSchedulePanel', () => {
  beforeEach(() => { vi.useFakeTimers(); vi.mocked(scheduledTestsAPI.listByAccount).mockResolvedValue([]); vi.mocked(scheduledTestsAPI.listResults).mockResolvedValue([]) })
  afterEach(() => { vi.clearAllMocks(); vi.useRealTimers() })
  it('creates a server schedule with the exact prompt and no auto-recovery', async () => {
    vi.mocked(scheduledTestsAPI.create).mockResolvedValue(plan as any)
    const wrapper = mountPanel(); await flushPromises()
    await wrapper.find('form').trigger('submit'); await flushPromises()
    expect(scheduledTestsAPI.create).toHaveBeenCalledWith(expect.objectContaining({ account_id: 42, enabled: true, max_results: 50, pelican_config: { ...cfg, prompt: 'draw a pelican', parallel_count: 1 } }))
    wrapper.unmount()
  })
  it('filters connection plans, pauses, edits and previews saved results', async () => {
    vi.mocked(scheduledTestsAPI.listByAccount).mockResolvedValue([{ id: 1 }, plan] as any)
    const result = { id: 9, status: 'success', response_text: '<html></html>', latency_ms: 1200, pelican_config: cfg }
    vi.mocked(scheduledTestsAPI.listResults).mockResolvedValue([result] as any)
    vi.mocked(scheduledTestsAPI.getResult).mockResolvedValue(result as any)
    vi.mocked(scheduledTestsAPI.update).mockResolvedValue({ ...plan, enabled: false } as any)
    const wrapper = mountPanel(); await flushPromises()
    expect(scheduledTestsAPI.listResults).toHaveBeenCalledWith(4, 50, false)
    await wrapper.findAll('button').find(b => b.text().includes('pelicanTest.pause'))!.trigger('click'); await flushPromises()
    expect(scheduledTestsAPI.update).toHaveBeenCalledWith(4, { enabled: false })
    await wrapper.findAll('button').find(b => b.text() === 'common.edit')!.trigger('click')
    expect(wrapper.emitted('edit')?.[0]).toEqual([cfg, 'gpt-6-astra'])
    await wrapper.findAll('button').find(b => b.text().includes('pelicanTest.preview'))!.trigger('click')
    await flushPromises()
    expect(scheduledTestsAPI.getResult).toHaveBeenCalledWith(4, 9)
    expect(wrapper.emitted('preview')?.[0]).toEqual([result])
    wrapper.unmount()
    await vi.advanceTimersByTimeAsync(30000)
    expect(scheduledTestsAPI.listByAccount).toHaveBeenCalledTimes(1)
  })
  it('shows API errors instead of reporting a schedule as enabled', async () => {
    vi.mocked(scheduledTestsAPI.create).mockRejectedValue(new Error('storage unavailable'))
    const wrapper = mountPanel(); await flushPromises()
    await wrapper.find('form').trigger('submit'); await flushPromises()
    expect(wrapper.find('[role="alert"]').text()).toBe('storage unavailable')
    expect(wrapper.find('form').exists()).toBe(true)
    wrapper.unmount()
  })
})
