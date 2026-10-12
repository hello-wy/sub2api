import { mount, flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import OpsErrorDetailsModal from '../OpsErrorDetailsModal.vue'
import { opsAPI } from '@/api/admin/ops'
import * as accountsAPI from '@/api/admin/accounts'
import Select from '@/components/common/Select.vue'

vi.mock('vue-i18n', async (original) => ({ ...await original<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/api/admin/ops', () => ({ opsAPI: { listUpstreamErrors: vi.fn(), listRequestErrors: vi.fn() } }))
vi.mock('@/api/admin/groups', () => ({ list: vi.fn().mockResolvedValue({ items: [{ id: 3, name: 'Group' }] }), getById: vi.fn().mockResolvedValue({ id: 3, name: 'Group' }) }))
vi.mock('@/api/admin/accounts', () => ({ list: vi.fn().mockResolvedValue({ items: [{ id: 839, name: 'cpa_不降智' }] }) }))
vi.mock('@/api/admin/users', () => ({ list: vi.fn().mockResolvedValue({ items: [{ id: 7, email: 'user@example.com' }] }) }))
let wrapper: ReturnType<typeof mount>
beforeEach(() => {
  vi.useFakeTimers()
  vi.mocked(opsAPI.listUpstreamErrors).mockReset().mockResolvedValue({ items: [], total: 0 } as never)
})
afterEach(() => { wrapper?.unmount(); vi.useRealTimers() })
async function settle() { await flushPromises(); await vi.advanceTimersByTimeAsync(400); await flushPromises() }
function setup() {
  wrapper = mount(OpsErrorDetailsModal, { props: { show: true, timeRange: '24h', errorType: 'upstream', groupId: 3 }, global: { stubs: { BaseDialog: { template: '<div><slot /></div>' }, OpsErrorLogTable: true } } })
}
function select(id: string) { return wrapper.findAllComponents(Select).find(s => s.props('id') === id)! }

describe('Ops error detail filters', () => {
  it('combines entity filters with search, resets to inherited group, and preserves filters when resuming', async () => {
    setup(); await settle()
    select('ops-error-account').vm.$emit('update:modelValue', 839)
    select('ops-error-user').vm.$emit('update:modelValue', 7)
    await wrapper.get('input').setValue('request_timeout')
    await settle()
    expect(opsAPI.listUpstreamErrors).toHaveBeenLastCalledWith(expect.objectContaining({ group_id: 3, account_id: 839, user_id: 7, q: 'request_timeout', phase: 'upstream', page: 1 }))
    await wrapper.setProps({ show: false }); await wrapper.setProps({ show: true, resumeState: true }); await settle()
    expect(select('ops-error-account').props('modelValue')).toBe(839)
    await wrapper.findAll('button').find(b => b.text() === 'common.reset')!.trigger('click'); await settle()
    const params = vi.mocked(opsAPI.listUpstreamErrors).mock.calls.at(-1)![0]
    expect(params).toMatchObject({ group_id: 3 })
    expect(params).not.toHaveProperty('account_id')
    expect(params).not.toHaveProperty('user_id')
    expect(params).not.toHaveProperty('q')
    select('ops-error-group').vm.$emit('update:modelValue', null); await settle()
    expect(vi.mocked(opsAPI.listUpstreamErrors).mock.calls.at(-1)![0]).not.toHaveProperty('group_id')
  })

  it('searches accounts remotely and retains the selected label', async () => {
    setup(); await settle()
    const account = select('ops-error-account')
    account.vm.$emit('update:modelValue', 839); await settle()
    vi.mocked(accountsAPI.list).mockResolvedValueOnce({ items: [] } as never)
    account.vm.$emit('search', 'another'); await settle()
    expect(accountsAPI.list).toHaveBeenLastCalledWith(1, 50, expect.objectContaining({ search: 'another' }))
    expect(account.props('options')).toContainEqual({ value: 839, label: 'cpa_不降智 · #839' })
  })

  it('ignores stale results after filters change', async () => {
    let finishOld!: (value: never) => void
    vi.mocked(opsAPI.listUpstreamErrors).mockImplementationOnce(() => new Promise(resolve => { finishOld = resolve }))
    setup(); await settle()
    select('ops-error-account').vm.$emit('update:modelValue', 839)
    vi.mocked(opsAPI.listUpstreamErrors).mockResolvedValueOnce({ items: [], total: 2 } as never)
    await settle()
    finishOld({ items: [], total: 99 } as never); await settle()
    expect(wrapper.findComponent({ name: 'OpsErrorLogTable' }).attributes('total')).toBe('2')
  })
})
