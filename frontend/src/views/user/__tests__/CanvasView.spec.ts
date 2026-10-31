import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { reactive } from 'vue'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import CanvasView from '../CanvasView.vue'
import { CANVAS_PROTOCOL } from '@/utils/canvas-bridge'

const mocks = vi.hoisted(() => ({
  auth: { user: { id: 7, role: 'admin' }, token: 'panel-token-must-not-leak' },
  current: vi.fn(), push: vi.fn(), fetchSettings: vi.fn(), settings: { enabled: true, url: 'http://127.0.0.1:3010/', upstream_retry_count: 2 },
  listKeys: vi.fn(), getKey: vi.fn()
}))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => reactive(mocks.auth) }))
vi.mock('@/stores/canvas', () => ({ useCanvasStore: () => ({ settings: mocks.settings, fetchSettings: mocks.fetchSettings }) }))
vi.mock('@/api/auth', () => ({ getCurrentUser: mocks.current }))
vi.mock('@/api/keys', () => ({ keysAPI: { list: mocks.listKeys, getById: mocks.getKey } }))
vi.mock('@/api/client', () => ({ apiClient: { get: () => { throw new Error('No custom canvas session endpoint') } } }))
vi.mock('vue-router', () => ({ useRouter: () => ({ push: mocks.push }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ locale: { value: 'zh-CN' } }) }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<div><slot /></div>' } }))

let wrapper: VueWrapper
async function open() {
  wrapper = mount(CanvasView, { attachTo: document.body, global: { stubs: { teleport: true, RouterLink: { template: '<a><slot /></a>' } } } })
  await flushPromises()
  const frame = wrapper.get('iframe').element as HTMLIFrameElement
  const send = vi.spyOn(frame.contentWindow!, 'postMessage')
  return { frame, send }
}
async function request(frame: HTMLIFrameElement, data: object, origin = 'http://127.0.0.1:3010') {
  window.dispatchEvent(new MessageEvent('message', { source: frame.contentWindow, origin, data: { protocol: CANVAS_PROTOCOL, requestId: 'test', ...data } }))
  await flushPromises()
}
beforeEach(() => {
  vi.clearAllMocks()
  mocks.auth.user = { id: 7, role: 'admin' }
  mocks.settings.enabled = true
  mocks.fetchSettings.mockResolvedValue(mocks.settings)
  mocks.current.mockResolvedValue({ data: { id: 7, role: 'user', username: '测试用户' } })
  mocks.listKeys.mockResolvedValue({ items: [], page: 1, pages: 1 })
})

const accountKey = { id: 21, user_id: 7, key: 'personal-key-secret', name: '图片专用', status: 'active', expires_at: null, quota: 0, quota_used: 0, group: { name: '图片分组' } }
describe('Canvas account key selection', () => {
  it('lists every page of owned key metadata without disclosing credentials', async () => {
    mocks.listKeys.mockResolvedValueOnce({ items: [accountKey, { ...accountKey, id: 22, user_id: 8 }], pages: 2 })
      .mockResolvedValueOnce({ items: [{ ...accountKey, id: 23, status: 'inactive' }], pages: 2 })
    const { frame, send } = await open()
    await request(frame, { type: 'keys' })
    expect(mocks.listKeys.mock.calls).toEqual([[1], [2]])
    const reply = send.mock.calls.at(-1)![0]
    expect(reply.type).toBe('keys')
    expect(reply.result).toEqual([{ id: 21, name: '图片专用', groupName: '图片分组', available: true }, { id: 23, name: '图片专用', groupName: '图片分组', available: false }])
    expect(JSON.stringify(reply)).not.toContain('personal-key-secret')
    expect(JSON.stringify(reply)).not.toContain(mocks.auth.token)
  })
  it('returns only the explicitly selected owned key', async () => {
    mocks.getKey.mockResolvedValue(accountKey)
    const { frame, send } = await open()
    await request(frame, { type: 'key', id: 21 })
    expect(mocks.getKey).toHaveBeenCalledWith(21)
    expect(send.mock.calls.at(-1)![0]).toEqual({ protocol: CANVAS_PROTOCOL, type: 'key', requestId: 'test', result: { id: 21, key: 'personal-key-secret' } })
    expect(JSON.stringify(send.mock.calls)).not.toContain(mocks.auth.token)
  })
  it.each([
    { ...accountKey, user_id: 8 }, { ...accountKey, id: 99 }, { ...accountKey, status: 'inactive' },
    { ...accountKey, expires_at: '2000-01-01T00:00:00Z' }, { ...accountKey, quota: 1, quota_used: 1 }
  ])('rejects foreign or unavailable keys', async key => {
    mocks.getKey.mockResolvedValue(key)
    const { frame, send } = await open()
    await request(frame, { type: 'key', id: 21 })
    const reply = send.mock.calls.at(-1)![0]
    expect(reply.type).toBe('key')
    expect(reply.error).toBeTruthy()
    expect(reply.result).toBeUndefined()
  })
  it('ignores forged requests and rejects invalid key IDs before querying keys', async () => {
    const { frame, send } = await open()
    await request(frame, { type: 'keys' }, 'https://evil.example')
    expect(mocks.listKeys).not.toHaveBeenCalled()
    await request(frame, { type: 'key', id: -1 })
    expect(mocks.getKey).not.toHaveBeenCalled()
    expect(send.mock.calls.at(-1)![0].error).toBeTruthy()
  })
  it('sends list failures on the matching request without replacing the identity session', async () => {
    mocks.listKeys.mockRejectedValue({ status: 503 })
    const { frame, send } = await open()
    await request(frame, { type: 'keys' })
    expect(send.mock.calls.at(-1)![0]).toMatchObject({ type: 'keys', requestId: 'test', error: expect.stringContaining('HTTP 503') })
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
  })
  it('drops a key arriving after reconnect', async () => {
    let finish!: (value: typeof accountKey) => void
    mocks.getKey.mockReturnValueOnce(new Promise(resolve => { finish = resolve }))
    const { frame, send } = await open()
    await request(frame, { type: 'key', id: 21 })
    await request(frame, { type: 'reconnect' })
    finish(accountKey)
    await flushPromises()
    expect(send.mock.calls.filter(([payload]) => payload.type === 'key')).toHaveLength(0)
  })
})
afterEach(() => { wrapper?.unmount(); document.body.innerHTML = ''; localStorage.clear() })

describe('Canvas host session', () => {
  it('delegates clipboard access to the embedded canvas', async () => {
    const { frame } = await open()
    expect(frame.getAttribute('allow')).toBe('clipboard-read; clipboard-write')
  })
  it('uses fresh backend role and sends identity without credentials', async () => {
    const { frame, send } = await open()
    await request(frame, { type: 'session', role: 'admin' })
    const payload = send.mock.calls[0][0]
    expect(payload.session.user.role).toBe('user')
    expect(payload.session).toEqual({ user: { id: 7, role: 'user' }, upstreamRetryCount: 2 })
    expect(mocks.current).toHaveBeenCalled()
    expect(mocks.fetchSettings).toHaveBeenCalledTimes(2)
    expect(JSON.stringify(payload)).not.toContain(mocks.auth.token)
    expect(frame.src).not.toContain('token')
  })
  it('ignores a forged origin and denies admin navigation even when cached role says admin', async () => {
    const { frame, send } = await open()
    await request(frame, { type: 'session' }, 'https://evil.example')
    expect(mocks.current).not.toHaveBeenCalled()
    expect(send).not.toHaveBeenCalled()
    await request(frame, { type: 'navigate', path: '/admin/settings' })
    expect(mocks.push).not.toHaveBeenCalled()
  })
  it('allows backend-verified admins to open existing platform management', async () => {
    mocks.current.mockResolvedValue({ data: { id: 7, role: 'admin' } })
    const { frame } = await open()
    await request(frame, { type: 'navigate', path: '/admin/settings' })
    expect(mocks.push).toHaveBeenCalledWith('/admin/settings')
  })
  it('does not embed a disabled canvas', async () => {
    mocks.settings.enabled = false
    wrapper = mount(CanvasView, { global: { stubs: { RouterLink: true } } })
    await flushPromises()
    expect(wrapper.find('iframe').exists()).toBe(false)
  })
  it('keeps iframe stable when the parent theme changes', async () => {
    const { frame, send } = await open()
    const src = frame.src
    document.documentElement.classList.toggle('dark')
    await flushPromises()
    expect(wrapper.get('iframe').element).toBe(frame)
    expect(frame.src).toBe(src)
    expect(send).toHaveBeenCalledWith({ protocol: CANVAS_PROTOCOL, type: 'theme', theme: document.documentElement.classList.contains('dark') ? 'dark' : 'light' }, 'http://127.0.0.1:3010')
    send.mockClear()
    frame.dispatchEvent(new Event('load'))
    await flushPromises()
    expect(send.mock.calls[0][0].theme).toBe(document.documentElement.classList.contains('dark') ? 'dark' : 'light')
  })
  it('invalidates the old session on role downgrade', async () => {
    const { send } = await open()
    reactive(mocks.auth).user.role = 'user'
    await flushPromises()
    expect(send.mock.calls.some(([payload]) => payload.type === 'invalidate')).toBe(true)
  })
  it('removes iframe on logout', async () => {
    await open()
    reactive(mocks.auth).user = undefined as never
    await flushPromises()
    expect(wrapper.find('iframe').exists()).toBe(false)
  })
  it('refuses an invalid platform session', async () => {
    mocks.current.mockRejectedValue(new Error('登录已失效'))
    const { frame, send } = await open()
    await request(frame, { type: 'session' })
    expect(send.mock.calls[0][0].error).toBeTruthy()
    expect(send.mock.calls[0][0].session).toBeUndefined()
  })
})

describe('Canvas personal configuration boundary', () => {
  it('opens without selecting or storing any platform key', async () => {
    const { frame, send } = await open()
    expect(wrapper.find('input[type="radio"]').exists()).toBe(false)
    expect(localStorage.length).toBe(0)
    await request(frame, { type: 'session' })
    expect(send.mock.calls.at(-1)![0].session).toEqual({ user: { id: 7, role: 'user' } })
  })
  it('recreates the frame only for a trusted reconnect request', async () => {
    const { frame } = await open()
    await request(frame, { type: 'reconnect' }, 'https://evil.example')
    expect(wrapper.get('iframe').element).toBe(frame)
    await request(frame, { type: 'reconnect' })
    expect(wrapper.get('iframe').element).not.toBe(frame)
  })
  it('does not forward keys or panel tokens even if the server adds extra fields', async () => {
    const { frame, send } = await open()
    mocks.current.mockResolvedValue({ data: { id: 7, role: 'user', email: 'private@example.com', apiKey: 'secret-key', token: mocks.auth.token } })
    await request(frame, { type: 'session' })
    expect(send.mock.calls.at(-1)![0].session).toEqual({ user: { id: 7, role: 'user' } })
    expect(JSON.stringify(send.mock.calls)).not.toContain('secret-key')
    expect(JSON.stringify(send.mock.calls)).not.toContain('private@example.com')
  })
  it('rejects a session belonging to a different user', async () => {
    const { frame, send } = await open()
    mocks.current.mockResolvedValue({ data: { id: 8, role: 'user' } })
    await request(frame, { type: 'session' })
    expect(send.mock.calls.at(-1)![0].error).toBeTruthy()
  })
  it('rejects the handshake when the refreshed canvas switch is disabled', async () => {
    const { frame, send } = await open()
    mocks.fetchSettings.mockResolvedValue({ enabled: false, url: mocks.settings.url })
    await request(frame, { type: 'session' })
    expect(send.mock.calls.at(-1)![0].error).toContain('已关闭')
    expect(send.mock.calls.at(-1)![0].session).toBeUndefined()
  })
  it('preserves a settings request failure instead of reporting every failure as a login issue', async () => {
    const { frame, send } = await open()
    mocks.fetchSettings.mockRejectedValue({ status: 503 })
    await request(frame, { type: 'session' })
    expect(send.mock.calls.at(-1)![0].error).toContain('HTTP 503')
    expect(send.mock.calls.at(-1)![0].session).toBeUndefined()
  })
  it('drops an old response after reconnecting', async () => {
    const { frame, send } = await open()
    let resolve!: (value: unknown) => void
    mocks.current.mockReturnValueOnce(new Promise(done => { resolve = done }))
    await request(frame, { type: 'session' })
    await request(frame, { type: 'reconnect' })
    resolve({ data: { id: 7, role: 'user' } })
    await flushPromises()
    expect(send.mock.calls.filter(([data]) => data.type === 'session')).toHaveLength(0)
  })
})
