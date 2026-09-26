import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import ScheduledTestsPanel from '../ScheduledTestsPanel.vue'
import { adminAPI } from '@/api/admin'

vi.mock('vue-i18n', async () => ({ ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'), useI18n: () => ({ t: (key: string, named?: Record<string, unknown>) => named ? key + ' ' + JSON.stringify(named) : key }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn() }) }))
vi.mock('@/api/admin', () => ({ adminAPI: { scheduledTests: { listByAccount: vi.fn(), listByGroup: vi.fn(), listGroupTestKeys: vi.fn(), triggerGroupPlan: vi.fn(), cancelGroupPlan: vi.fn(), listResults: vi.fn(), getResult: vi.fn(), create: vi.fn(), update: vi.fn(), delete: vi.fn() } } }))

const config = { question_kind: 'pelican' as const, prompt: 'draw a pelican', reasoning_effort: 'medium', parallel_count: 1 }
const plan = { id: 8, account_id: 0, group_id: 17, api_key_id: 23, model_id: 'public-model', cron_expression: '0 * * * *', enabled: true, max_results: 20, auto_recover: false, pelican_config: config }
function mountPanel() {
  return mount(ScheduledTestsPanel, { props: { show: true, embedded: true, accountId: null, groupId: 17, modelOptions: [], pelicanConfig: config }, global: { stubs: { BaseDialog: { template: '<div><slot /></div>' }, ConfirmDialog: true, Select: true, Input: true, Toggle: true, Icon: true, HelpTooltip: true, PelicanTestFields: true } } })
}
describe('group scheduled Pelican plans', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.mocked(adminAPI.scheduledTests.listByGroup).mockResolvedValue([])
    vi.mocked(adminAPI.scheduledTests.listGroupTestKeys).mockResolvedValue([{ id: 23, name: 'test', user_email: 'owner@example.com' }])
    vi.mocked(adminAPI.scheduledTests.listResults).mockResolvedValue([])
  })
  afterEach(() => { vi.clearAllMocks(); vi.useRealTimers() })

  it('requires an explicit billing key and creates a group target without account recovery', async () => {
    const wrapper = mountPanel(); await flushPromises()
    const vm = wrapper.vm as any
    expect(adminAPI.scheduledTests.listByAccount).not.toHaveBeenCalled()
    expect(adminAPI.scheduledTests.listByGroup).toHaveBeenCalledWith(17)
    expect(vm.newPlan.api_key_id).toBe(0)
    vm.newPlan.model_id = 'public-model'
    await vm.handleCreate()
    expect(adminAPI.scheduledTests.create).not.toHaveBeenCalled()
    vm.newPlan.api_key_id = 23
    vm.newPlan.auto_recover = true
    await vm.handleCreate()
    expect(adminAPI.scheduledTests.create).toHaveBeenCalledWith(expect.objectContaining({ group_id: 17, api_key_id: 23, model_id: 'public-model', auto_recover: false, pelican_config: config }))
    expect(adminAPI.scheduledTests.create).toHaveBeenCalledWith(expect.not.objectContaining({ account_id: expect.anything() }))
    wrapper.unmount()
  })

  it('edits credentials, pauses, starts execution and loads preview content', async () => {
    vi.mocked(adminAPI.scheduledTests.listByGroup).mockResolvedValue([plan] as any)
    vi.mocked(adminAPI.scheduledTests.update).mockResolvedValue(plan as any)
    const result = { id: 19, plan_id: 8, response_text: '<svg></svg>' }
    vi.mocked(adminAPI.scheduledTests.getResult).mockResolvedValue(result as any)
    const wrapper = mountPanel(); await flushPromises()
    const vm = wrapper.vm as any
    vm.startEdit(plan); vm.editForm.api_key_id = 24
    await vm.handleEdit()
    expect(adminAPI.scheduledTests.update).toHaveBeenCalledWith(8, expect.objectContaining({ api_key_id: 24, auto_recover: false }))
    await vm.handleToggleEnabled(plan, false)
    expect(adminAPI.scheduledTests.update).toHaveBeenLastCalledWith(8, { enabled: false })
    await wrapper.get('[data-testid="trigger-group-test"]').trigger('click'); await flushPromises()
    expect(adminAPI.scheduledTests.triggerGroupPlan).toHaveBeenCalledWith(8)
    await vm.previewResult(result)
    expect(wrapper.emitted('preview')?.[0]).toEqual([result])
    wrapper.unmount()
  })

  it('shows a missing-key explanation and hides account recovery and candy selection', async () => {
    vi.mocked(adminAPI.scheduledTests.listGroupTestKeys).mockResolvedValue([])
    const wrapper = mountPanel(); await flushPromises()
    expect(wrapper.find('[data-testid="group-test-no-keys"]').exists()).toBe(true)
    const vm = wrapper.vm as any; vm.showAddForm = true; await flushPromises()
    expect(wrapper.text()).not.toContain('admin.scheduledTests.autoRecover')
    expect(wrapper.find('pelican-test-fields-stub').attributes('drawingonly')).toBe('true')
    expect(wrapper.findAll('input-stub')).toHaveLength(3)
    wrapper.unmount()
  })

  it('does not let an old group response replace the selected group', async () => {
    let resolveOld!: (keys: any[]) => void
    vi.mocked(adminAPI.scheduledTests.listGroupTestKeys).mockImplementationOnce(() => new Promise(resolve => { resolveOld = resolve }))
    const wrapper = mountPanel()
    expect(wrapper.find('[data-testid="group-test-no-keys"]').exists()).toBe(false)
    await wrapper.setProps({ groupId: 18 }); await flushPromises()
    resolveOld([{ id: 99, name: 'stale', user_email: 'old@example.com' }]); await flushPromises()
    expect((wrapper.vm as any).testKeys.map((key: any) => key.id)).toEqual([23])
    expect(adminAPI.scheduledTests.listByGroup).toHaveBeenCalledWith(18)
    expect(adminAPI.scheduledTests.listByGroup).not.toHaveBeenCalledWith(17)
    wrapper.unmount()
  })

  it('starts immediately and refreshes running, retrying and completed states every second', async () => {
    vi.mocked(adminAPI.scheduledTests.listByGroup).mockResolvedValue([plan] as any)
    let start!: () => void
    vi.mocked(adminAPI.scheduledTests.triggerGroupPlan).mockImplementationOnce(() => new Promise<void>(resolve => { start = resolve }))
    const wrapper = mountPanel(); await flushPromises()
    await wrapper.get('[data-testid="trigger-group-test"]').trigger('click')
    expect(wrapper.get('[data-testid="group-test-status-8"]').text()).toContain('groupStatusStarting')
    expect(wrapper.get('[data-testid="trigger-group-test"]').attributes('disabled')).toBeDefined()
    const execution = { status: 'running', attempt: 1, max_attempts: 3, total: 1, completed: 0, succeeded: 0, failed: 0, started_at: new Date().toISOString() }
    const running = { ...plan, running_until: new Date(Date.now() + 900000).toISOString(), execution }
    vi.mocked(adminAPI.scheduledTests.listByGroup).mockResolvedValue([running] as any)
    start(); await flushPromises()
    expect(wrapper.get('[data-testid="group-test-status-8"]').text()).toContain('groupStatusRunning')

    vi.mocked(adminAPI.scheduledTests.listByGroup).mockResolvedValue([{ ...running, execution: { ...execution, status: 'retrying', completed: 1, failed: 1, last_error: 'upstream unavailable', retry_at: new Date(Date.now() + 5000).toISOString() } }] as any)
    await vi.advanceTimersByTimeAsync(1000); await flushPromises()
    const status = wrapper.get('[data-testid="group-test-status-8"]').text()
    expect(status).toContain('groupStatusRetrying')
    expect(status).toContain('upstream unavailable')
    expect(status).toContain('"seconds":4')
    expect(wrapper.get('[data-testid="trigger-group-test"]').attributes('disabled')).toBeDefined()
    expect((wrapper.vm as any).loading).toBe(false)

    vi.mocked(adminAPI.scheduledTests.listByGroup).mockResolvedValue([{ ...plan, running_until: null, execution: { ...execution, status: 'success', attempt: 2, completed: 1, succeeded: 1, finished_at: new Date().toISOString() } }] as any)
    const result = { id: 41, plan_id: 8, status: 'success', started_at: new Date().toISOString(), latency_ms: 25 }
    vi.mocked(adminAPI.scheduledTests.listResults).mockResolvedValue([result] as any)
    await vi.advanceTimersByTimeAsync(1000); await flushPromises()
    expect(wrapper.get('[data-testid="group-test-status-8"]').text()).toContain('groupStatusSuccess')
    expect(wrapper.get('[data-testid="trigger-group-test"]').attributes('disabled')).toBeUndefined()
    expect((wrapper.vm as any).results[0].id).toBe(41)
    wrapper.unmount()
    const requests = vi.mocked(adminAPI.scheduledTests.listByGroup).mock.calls.length
    await vi.advanceTimersByTimeAsync(5000)
    expect(adminAPI.scheduledTests.listByGroup).toHaveBeenCalledTimes(requests)
  })

  it('shows stream phases while the gateway response is still in progress', async () => {
    const execution = { status: 'running', phase: 'waiting', attempt: 1, max_attempts: 3, total: 1, completed: 0, succeeded: 0, failed: 0, started_at: new Date().toISOString() }
    const running = { ...plan, running_until: new Date(Date.now() + 900000).toISOString(), execution }
    vi.mocked(adminAPI.scheduledTests.listByGroup).mockResolvedValue([running] as any)
    const wrapper = mountPanel(); await flushPromises()
    expect(wrapper.get('[data-testid="group-test-status-8"]').text()).toContain('groupPhaseWaiting')
    for (const [phase, label] of [['receiving', 'groupPhaseReceiving'], ['thinking', 'groupPhaseThinking'], ['generating', 'groupPhaseGenerating'], ['saving', 'groupPhaseSaving']]) {
      vi.mocked(adminAPI.scheduledTests.listByGroup).mockResolvedValue([{ ...running, execution: { ...execution, phase } }] as any)
      await vi.advanceTimersByTimeAsync(1000); await flushPromises()
      const status = wrapper.get('[data-testid="group-test-status-8"]').text()
      expect(status).toContain(label)
      expect(status).not.toContain('groupStatusRunning')
      expect(status).not.toContain('groupStatusSuccess')
      expect(wrapper.get('[data-testid="trigger-group-test"]').attributes('disabled')).toBeDefined()
    }
    vi.mocked(adminAPI.scheduledTests.listByGroup).mockResolvedValue([{ ...running, execution: { ...execution, phase: 'generating', status: 'retrying' } }] as any)
    await vi.advanceTimersByTimeAsync(1000); await flushPromises()
    expect(wrapper.get('[data-testid="group-test-status-8"]').text()).toContain('groupStatusRetrying')
    wrapper.unmount()
  })

  it('interrupts a running execution and keeps restart disabled until it stops', async () => {
    const runningUntil = new Date(Date.now() + 900000).toISOString()
    const execution = { status: 'running', phase: 'generating', attempt: 1, max_attempts: 3, total: 1, completed: 0, succeeded: 0, failed: 0, started_at: new Date().toISOString() }
    const running = { ...plan, running_until: runningUntil, execution }
    vi.mocked(adminAPI.scheduledTests.listByGroup).mockResolvedValue([running] as any)
    let acknowledge!: () => void
    vi.mocked(adminAPI.scheduledTests.cancelGroupPlan).mockImplementationOnce(() => new Promise<void>(resolve => { acknowledge = resolve }))
    const wrapper = mountPanel(); await flushPromises()
    await wrapper.get('[data-testid="cancel-group-test"]').trigger('click')
    expect(adminAPI.scheduledTests.cancelGroupPlan).toHaveBeenCalledWith(8, runningUntil)
    expect(wrapper.get('[data-testid="group-test-status-8"]').text()).toContain('groupStatusCancelling')
    expect(wrapper.get('[data-testid="cancel-group-test"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-testid="trigger-group-test"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="cancel-group-test"]').trigger('click')
    expect(adminAPI.scheduledTests.cancelGroupPlan).toHaveBeenCalledTimes(1)
    vi.mocked(adminAPI.scheduledTests.listByGroup).mockResolvedValue([{ ...running, execution: { ...execution, status: 'cancelling', cancel_requested: true } }] as any)
    acknowledge(); await flushPromises()
    expect(wrapper.get('[data-testid="group-test-status-8"]').text()).toContain('groupStatusCancelling')
    vi.mocked(adminAPI.scheduledTests.listByGroup).mockResolvedValue([{ ...plan, running_until: null, execution: { ...execution, status: 'interrupted', cancel_requested: true, finished_at: new Date().toISOString() } }] as any)
    await vi.advanceTimersByTimeAsync(1000); await flushPromises()
    expect(wrapper.get('[data-testid="group-test-status-8"]').text()).toContain('groupStatusInterrupted')
    expect(wrapper.find('[data-testid="cancel-group-test"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="trigger-group-test"]').attributes('disabled')).toBeUndefined()
    expect(adminAPI.scheduledTests.update).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('allows interruption during retry and recovers the button after a failed request', async () => {
    const running = { ...plan, running_until: new Date(Date.now() + 900000).toISOString(), execution: { status: 'retrying', attempt: 1, max_attempts: 3, total: 1, completed: 1, succeeded: 0, failed: 1, started_at: new Date().toISOString() } }
    vi.mocked(adminAPI.scheduledTests.listByGroup).mockResolvedValue([running] as any)
    vi.mocked(adminAPI.scheduledTests.cancelGroupPlan).mockRejectedValueOnce(new Error('offline'))
    const wrapper = mountPanel(); await flushPromises()
    await wrapper.get('[data-testid="cancel-group-test"]').trigger('click'); await flushPromises()
    expect(wrapper.get('[data-testid="cancel-group-test"]').attributes('disabled')).toBeUndefined()
    expect(wrapper.get('[data-testid="group-test-status-8"]').text()).toContain('groupStatusRetrying')
    wrapper.unmount()
  })

  it('does not overlap background polls or restore stale results after switching groups', async () => {
    vi.mocked(adminAPI.scheduledTests.listByGroup).mockResolvedValue([plan] as any)
    const wrapper = mountPanel(); await flushPromises()
    let resolveOld!: (plans: any[]) => void
    vi.mocked(adminAPI.scheduledTests.listByGroup).mockImplementationOnce(() => new Promise(resolve => { resolveOld = resolve }))
    await vi.advanceTimersByTimeAsync(1000)
    const calls = vi.mocked(adminAPI.scheduledTests.listByGroup).mock.calls.length
    await vi.advanceTimersByTimeAsync(4000)
    expect(adminAPI.scheduledTests.listByGroup).toHaveBeenCalledTimes(calls)
    vi.mocked(adminAPI.scheduledTests.listByGroup).mockResolvedValue([])
    await wrapper.setProps({ groupId: 18 }); await flushPromises()
    resolveOld([plan]); await flushPromises()
    expect((wrapper.vm as any).plans).toEqual([])
    expect((wrapper.vm as any).results).toEqual([])
    wrapper.unmount()
  })

  it('keeps a collapsed plan collapsed during polling and recovers from failed status reads', async () => {
    vi.mocked(adminAPI.scheduledTests.listByGroup).mockResolvedValue([plan] as any)
    const wrapper = mountPanel(); await flushPromises()
    await (wrapper.vm as any).toggleExpand(8)
    vi.mocked(adminAPI.scheduledTests.listByGroup).mockRejectedValueOnce(new Error('offline'))
    await vi.advanceTimersByTimeAsync(1000); await flushPromises()
    expect(wrapper.find('[data-testid="group-status-reconnect"]').exists()).toBe(true)
    expect((wrapper.vm as any).plans).toHaveLength(1)
    await vi.advanceTimersByTimeAsync(1000); await flushPromises()
    expect(wrapper.find('[data-testid="group-status-reconnect"]').exists()).toBe(false)
    expect((wrapper.vm as any).expandedPlanId).toBeNull()
    wrapper.unmount()
  })

})
