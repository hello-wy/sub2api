import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import ScheduledTestsPanel from '../ScheduledTestsPanel.vue'
import { adminAPI } from '@/api/admin'

vi.mock('vue-i18n', async () => ({ ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn() }) }))
vi.mock('@/api/admin', () => ({ adminAPI: { scheduledTests: { listByAccount: vi.fn(), listByGroup: vi.fn(), listGroupTestKeys: vi.fn(), triggerGroupPlan: vi.fn(), listResults: vi.fn(), getResult: vi.fn(), create: vi.fn(), update: vi.fn(), delete: vi.fn() } } }))

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

  it('edits credentials, pauses, queues execution and loads preview content', async () => {
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
})
