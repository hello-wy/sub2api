import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import Dashboard from '../PelicanRecordsDashboard.vue'
import { scheduledTestsAPI as api } from '@/api/admin/scheduledTests'
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/api/admin/scheduledTests', () => ({ scheduledTestsAPI: { listByAccount: vi.fn(), listResults: vi.fn(), getResult: vi.fn() } }))
const account = { id: 42, name: 'Manual account' } as any
const manual = { createdAt: '2026-09-23T12:00:00Z', modelId: 'saved-model', reasoningEffort: 'medium', runs: [{ html: '<html><body>MANUAL-ANIMATION</body></html>', output: '', durationMs: 34000, status: 'success' }] }
function render(extra = {}) { return mount(Dashboard, { props: { accounts: [], account, manualRecord: manual, ...extra } }) }
beforeEach(() => { vi.resetAllMocks(); localStorage.clear(); vi.mocked(api.listByAccount).mockResolvedValue([]) })
describe('Pelican record dashboard', () => {
  it('shows and opens a manual result when the current account is outside the list', async () => {
    const wrapper = render(); await flushPromises()
    expect(wrapper.findAll('[data-testid="pelican-record-card"]')).toHaveLength(1)
    expect(wrapper.text()).toContain('saved-model / medium')
    expect(wrapper.text()).toContain('34.0 s')
    expect(wrapper.text()).toContain('sourceManual')
    await wrapper.get('article button').trigger('click')
    expect(wrapper.get('[data-testid="record-detail"] iframe').attributes('srcdoc')).toContain('MANUAL-ANIMATION')
    expect(wrapper.get('[data-testid="record-detail"] iframe').attributes('srcdoc')).toContain('Content-Security-Policy')
    expect(wrapper.get('article iframe').classes()).toContain('pointer-events-none')
    wrapper.unmount()
  })
  it('keeps local records if the server API fails and omits empty accounts', async () => {
    vi.mocked(api.listByAccount).mockRejectedValue(new Error('offline'))
    const wrapper = render({ accounts: [{ id: 99, name: 'Empty account' }] }); await flushPromises()
    expect(wrapper.findAll('article')).toHaveLength(1)
    expect(wrapper.text()).not.toContain('Empty account')
    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
    wrapper.unmount()
  })
  it('loads other accounts manual history and never fabricates duration or time', async () => {
    localStorage.setItem('sub2api-pelican-test:99', JSON.stringify([{ ...manual, createdAt: '', runs: [{ html: '<svg></svg>' }] }]))
    const wrapper = render({ account: null, manualRecord: null, accounts: [{ id: 99, name: 'Other manual account' }] }); await flushPromises()
    expect(wrapper.text()).toContain('Other manual account')
    expect(wrapper.text()).not.toContain('0.0 s')
    expect(wrapper.text()).toContain('—')
    wrapper.unmount()
  })
  it('merges sources into one account card and chooses the most recent result across plans', async () => {
    vi.mocked(api.listByAccount).mockResolvedValue([{ id: 1, pelican_config: {} }, { id: 2, pelican_config: {} }] as any)
    vi.mocked(api.listResults).mockImplementation(async id => [{ id, started_at: id === 1 ? '2026-09-23T11:00:00Z' : '2026-09-23T13:00:00Z' }] as any)
    vi.mocked(api.getResult).mockResolvedValue({ id: 2, started_at: '2026-09-23T13:00:00Z', latency_ms: 1000, response_text: '<svg>NEWEST</svg>', status: 'success', pelican_config: { model_id: 'new-model', reasoning_effort: 'high' } } as any)
    const wrapper = render(); await flushPromises()
    expect(wrapper.findAll('article')).toHaveLength(1)
    expect(wrapper.text()).toContain('new-model / high')
    expect(api.getResult).toHaveBeenCalledTimes(1)
    await wrapper.get('article button').trigger('click')
    expect(wrapper.get('[data-testid="record-detail"] iframe').attributes('srcdoc')).toContain('NEWEST')
    wrapper.unmount()
  })
})
