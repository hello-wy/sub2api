import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, reactive, nextTick } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { usePlazaStatus } from '@/composables/usePlazaStatus'
import { model, group, matrixRow } from './fixtures'

const mocks = vi.hoisted(() => ({ getMatrix:vi.fn(), list:vi.fn() }))
vi.mock('@/api/channelMonitorV2', () => ({ getMatrix:mocks.getMatrix }))
vi.mock('@/api/channelMonitor', () => ({ list:mocks.list }))
const state = reactive({ authenticated:false, userId:1, enabled:true, mode:'v2', settings:true })
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ get isAuthenticated() { return state.authenticated }, get user() { return { id:state.userId } } }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ get cachedPublicSettings() { return state.settings ? { channel_monitor_default_interval_seconds:60 } : null } }) }))
vi.mock('@/utils/featureFlags', () => ({
  getChannelMonitorMode: () => state.mode,
  getChannelMonitorRefreshIntervalSeconds: () => 60,
  isChannelMonitorRouteEnabled: () => state.enabled,
}))
let dispose: (() => void) | undefined
function render() {
  let result!: ReturnType<typeof usePlazaStatus>
  const wrapper = mount(defineComponent({ setup() { result=usePlazaStatus();return () => null } }))
  dispose=() => wrapper.unmount()
  return result
}
beforeEach(() => {
  vi.useFakeTimers()
  vi.clearAllMocks()
  state.authenticated=false;state.userId=1;state.enabled=true;state.mode='v2';state.settings=true
  mocks.getMatrix.mockResolvedValue({ coverage:{ data_through:'2026-09-26T00:00:00Z' }, items:[matrixRow()] })
  mocks.list.mockResolvedValue({ items:[] })
})
afterEach(() => { dispose?.();vi.useRealTimers() })
describe('plaza status refresh', () => {
  it('does not request protected status for anonymous users, unloaded settings or a disabled monitor', async () => {
    render();await flushPromises()
    expect(mocks.getMatrix).not.toHaveBeenCalled()
    state.settings=false;state.authenticated=true;await nextTick()
    expect(mocks.getMatrix).not.toHaveBeenCalled()
    state.enabled=false;state.settings=true;await nextTick()
    expect(mocks.getMatrix).not.toHaveBeenCalled()
  })
  it('requests the exact matrix dimensions, refreshes, clears failure data, and cleans up on unmount', async () => {
    state.authenticated=true
    const status=render();await flushPromises()
    expect(mocks.getMatrix).toHaveBeenCalledWith({ range:'24h', platforms:[], groupIds:[], models:[] },'platform_group_model',false,expect.any(AbortSignal))
    expect(status.statusFor(model(),group()).health).toBe('healthy')
    mocks.getMatrix.mockRejectedValueOnce(new Error('offline'))
    await vi.advanceTimersByTimeAsync(60000)
    expect(status.failed.value).toBe(true)
    expect(status.statusFor(model(),group()).health).toBe('unknown')
    await vi.advanceTimersByTimeAsync(60000)
    expect(status.failed.value).toBe(false)
    expect(status.statusFor(model(),group()).health).toBe('healthy')
    dispose?.()
    await vi.advanceTimersByTimeAsync(60000)
    expect(mocks.getMatrix).toHaveBeenCalledTimes(3)
  })
  it('uses only the selected monitor version and clears data on logout', async () => {
    state.authenticated=true;state.mode='v1'
    const status=render();await flushPromises()
    expect(mocks.list).toHaveBeenCalledOnce()
    expect(mocks.getMatrix).not.toHaveBeenCalled()
    state.mode='v2';await nextTick();await flushPromises()
    expect(status.statusFor(model(),group()).health).toBe('healthy')
    state.authenticated=false;await nextTick()
    expect(status.statusFor(model(),group()).health).toBe('unknown')
    await vi.advanceTimersByTimeAsync(60000)
    expect(mocks.getMatrix).toHaveBeenCalledOnce()
  })
  it('ignores a stale in-flight response after the user changes', async () => {
    let resolve!: (data: unknown) => void
    mocks.getMatrix.mockImplementationOnce(() => new Promise(r => { resolve=r }))
    state.authenticated=true
    const status=render()
    state.authenticated=false;await nextTick()
    resolve({ coverage:{ data_through:'2026-09-26T00:00:00Z' },items:[matrixRow()] })
    await flushPromises()
    expect(status.statusFor(model(),group()).health).toBe('unknown')
  })
})
