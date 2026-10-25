import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { reactive } from 'vue'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import CanvasView from '../CanvasView.vue'
import { CANVAS_PROTOCOL } from '@/utils/canvas-bridge'

const mocks = vi.hoisted(() => ({
  auth: { user: { id: 7, role: 'admin' }, token: 'panel-token-must-not-leak' },
  current: vi.fn(), list: vi.fn(), key: vi.fn(), push: vi.fn(), session: vi.fn(), settings: { enabled: true, url: "http://127.0.0.1:3010/", api_base_url: '', effective_api_base_url: 'https://configured.example' }
}))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => reactive(mocks.auth) }))
vi.mock('@/stores/canvas', () => ({ useCanvasStore: () => ({ settings: mocks.settings, fetchSettings: async () => mocks.settings }) }))
vi.mock('@/api/auth', () => ({ getCurrentUser: mocks.current }))
vi.mock('@/api/keys', () => ({ keysAPI: { list: mocks.list, getById: mocks.key } }))
vi.mock('@/api/client', () => ({ buildGatewayUrl: () => 'https://api.example/', apiClient: { get: mocks.session } }))
vi.mock('vue-router', () => ({ useRouter: () => ({ push: mocks.push }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ locale: { value: 'zh-CN' } }) }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<div><slot /></div>' } }))

let wrapper: VueWrapper
const key = { id: 12, user_id: 7, name: '我的画布', key: 'user-gateway-key', status: 'active', group_id: 1, expires_at: null, group: { name: '创作分组' } }
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
  mocks.settings.effective_api_base_url = 'https://configured.example'
  mocks.session.mockResolvedValue({ data: { user: { id: 7, role: 'user' }, apiKey: 'user-gateway-key', keyId: 12, keyName: 'test', canvasUrl: mocks.settings.url, baseUrl: mocks.settings.effective_api_base_url } })
  localStorage.setItem('sub2api:canvas-key:7', '12')
  mocks.current.mockResolvedValue({ data: { id: 7, role: 'user', username: '测试用户' } })
  mocks.list.mockResolvedValue({ items: [key], total: 1, page_size: 10 })
  mocks.key.mockResolvedValue(key)
})
afterEach(() => { wrapper?.unmount(); document.body.innerHTML = ''; localStorage.clear() })

describe('Canvas host session', () => {
  it('uses fresh backend role and sends only the selected user gateway key', async () => {
    const { frame, send } = await open()
    await request(frame, { type: 'session', role: 'admin' })
    const payload = send.mock.calls[0][0]
    expect(payload.session.user.role).toBe('user')
    expect(payload.session.apiKey).toBe('user-gateway-key')
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
  it('does not embed a disabled canvas even with a remembered key', async () => {
    mocks.settings.enabled = false
    wrapper = mount(CanvasView, { global: { stubs: { RouterLink: true } } })
    await flushPromises()
    expect(wrapper.find('iframe').exists()).toBe(false)
    expect(mocks.list).not.toHaveBeenCalled()
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
  it('refuses keys from another user', async () => {
    mocks.session.mockRejectedValue(new Error('API key not found'))
    const { frame, send } = await open()
    await request(frame, { type: 'session' })
    expect(send.mock.calls[0][0].error).toBeTruthy()
    expect(send.mock.calls[0][0].session).toBeUndefined()
  })
})

function button(label: string) {
  return wrapper.findAll('button').find(item => item.text() === label)!
}
const secondKey = { ...key, id: 18, name: '另一把密钥', key: 'second-gateway-key' }
function nextSession(id = 18) {
  return { data: { user: { id: 7, role: 'user' }, apiKey: 'second-gateway-key', keyId: id, keyName: '另一把密钥', canvasUrl: mocks.settings.url, baseUrl: 'https://new-api.example' } }
}
async function chooseSecondKey() {
  mocks.list.mockResolvedValue({ items: [key, secondKey], total: 2, page_size: 10 })
  await request(wrapper.get('iframe').element as HTMLIFrameElement, { type: 'select-key' })
  await flushPromises()
  await wrapper.get('input[value="18"]').setValue()
}

describe('Canvas key switching and API site', () => {
  for (const purpose of ['text', 'image'] as const) {
    it(`recreates the iframe and uses the selected ${purpose} key after switching and refresh`, async () => {
      mocks.list.mockResolvedValue({ items: [key, secondKey], total: 2, page_size: 10 })
      mocks.session.mockImplementation(async (path: string) => ({ data: {
        user: { id: 7, role: 'user' }, apiKey: path.endsWith('/18') ? secondKey.key : key.key,
        keyId: path.endsWith('/18') ? 18 : 12, keyName: path.endsWith('/18') ? secondKey.name : key.name,
        canvasUrl: mocks.settings.url, baseUrl: mocks.settings.effective_api_base_url
      } }))
      const { frame } = await open()
      await request(frame, { type: 'select-key', purpose })
      expect((wrapper.get('input[value="12"]').element as HTMLInputElement).checked).toBe(true)
      await wrapper.get('input[value="18"]').setValue()
      await button('确认切换').trigger('click')
      await flushPromises()
      expect(wrapper.get('iframe').element).not.toBe(frame)
      expect(localStorage.getItem(`sub2api:canvas-${purpose}-key:7`)).toBe('18')
      const verify = async (current: HTMLIFrameElement, send: Awaited<ReturnType<typeof open>>['send']) => {
        await request(current, { type: 'session' })
        const payload = send.mock.calls.find(([data]) => data.type === 'session')![0]
        expect(payload.session[purpose + 'Key'].apiKey).toBe(secondKey.key)
        expect(payload.session[(purpose === 'text' ? 'image' : 'text') + 'Key'].apiKey).toBe(key.key)
        expect(payload.session.apiKey).toBe(key.key)
      }
      const current = wrapper.get('iframe').element as HTMLIFrameElement
      await verify(current, vi.spyOn(current.contentWindow!, 'postMessage'))
      wrapper.unmount()
      const restored = await open()
      await verify(restored.frame, restored.send)
    })
  }
  it('recreates the frame only for a reconnect request from its trusted window', async () => {
    const { frame } = await open()
    await request(frame, { type: 'reconnect' }, 'https://evil.example')
    expect(wrapper.get('iframe').element).toBe(frame)
    await request(frame, { type: 'reconnect' })
    expect(wrapper.get('iframe').element).not.toBe(frame)
    expect(localStorage.getItem('sub2api:canvas-key:7')).toBe('12')
  })
  it('never falls back to the default key if a selected text key fails server validation', async () => {
    localStorage.setItem('sub2api:canvas-text-key:7', '18')
    mocks.list.mockResolvedValue({ items: [key, secondKey], total: 2, page_size: 10 })
    const session = { user: { id: 7, role: 'user' }, apiKey: key.key, keyId: 12, keyName: key.name, canvasUrl: mocks.settings.url, baseUrl: mocks.settings.effective_api_base_url }
    mocks.session.mockImplementation(async (path: string) => {
      if (path.endsWith('/18')) throw new Error('text key revoked')
      return { data: session }
    })
    const { frame, send } = await open()
    await request(frame, { type: 'session' })
    expect(send.mock.calls[0][0].error).toBeTruthy()
    expect(send.mock.calls[0][0].session).toBeUndefined()
  })
  it('keeps connection information out of the canvas header and opens selection only for the trusted frame', async () => {
    mocks.auth.user.role = 'user'
    const { frame } = await open()
    expect(wrapper.text()).not.toContain('https://configured.example')
    expect(wrapper.text()).not.toContain('切换密钥')
    expect(wrapper.text()).not.toContain('由管理员配置')
    await request(frame, { type: 'select-key' }, 'https://evil.example')
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
    await request(frame, { type: 'select-key' })
    expect(wrapper.find('[role="dialog"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('我的密钥 · 仅列出当前账号的可用密钥')
    expect(wrapper.text()).toContain('https://configured.example')
    expect(wrapper.text()).not.toContain('user-gateway-key')
  })
  it('uses the server-provided URL for the frame', async () => {
    const { frame, send } = await open()
    mocks.session.mockResolvedValue(nextSession(12))
    await request(frame, { type: 'session', baseUrl: 'https://forged.example' })
    expect(send.mock.calls[0][0].session.baseUrl).toBe('https://new-api.example')
  })
  it('falls back to the current gateway when no site is configured', async () => {
    mocks.settings.effective_api_base_url = ''
    const { frame, send } = await open()
    mocks.session.mockResolvedValue({ data: { ...nextSession(12).data, baseUrl: '' } })
    await request(frame, { type: 'session' })
    expect(send.mock.calls[0][0].session.baseUrl).toBe('https://api.example')
  })
  it('forwards only the server retry policy to the trusted canvas', async () => {
    const { frame, send } = await open()
    for (const upstreamRetryCount of [0, 5]) {
      mocks.session.mockResolvedValue({ data: { ...nextSession(12).data, upstreamRetryCount } })
      await request(frame, { type: 'session', upstreamRetryCount: 99 })
      expect(send.mock.calls.at(-1)![0].session.upstreamRetryCount).toBe(upstreamRetryCount)
    }
  })
  it('keeps the iframe and stored key unchanged when opening and cancelling', async () => {
    const { frame, send } = await open()
    await chooseSecondKey()
    expect(wrapper.get('iframe').element).toBe(frame)
    await button('取消').trigger('click')
    expect(wrapper.get('iframe').element).toBe(frame)
    expect(localStorage.getItem('sub2api:canvas-key:7')).toBe('12')
    expect(send).not.toHaveBeenCalled()
  })
  it('validates, persists and reconnects with the newly selected key', async () => {
    const { frame, send } = await open()
    await chooseSecondKey()
    mocks.session.mockResolvedValue(nextSession())
    await button('确认切换').trigger('click')
    await flushPromises()
    expect(mocks.session).toHaveBeenCalledWith('/canvas/session/18')
    expect(localStorage.getItem('sub2api:canvas-key:7')).toBe('18')
    expect(send.mock.calls.some(([payload]) => payload.type === 'invalidate')).toBe(true)
    const newFrame = wrapper.get('iframe').element as HTMLIFrameElement
    expect(newFrame).not.toBe(frame)
    const newSend = vi.spyOn(newFrame.contentWindow!, 'postMessage')
    await request(newFrame, { type: 'session' })
    expect(newSend.mock.calls[0][0].session.apiKey).toBe('second-gateway-key')
  })
  it('retains the old session when the new key is rejected', async () => {
    const { frame } = await open()
    await chooseSecondKey()
    mocks.session.mockRejectedValue(new Error('quota exhausted'))
    await button('确认切换').trigger('click')
    await flushPromises()
    expect(wrapper.get('iframe').element).toBe(frame)
    expect(localStorage.getItem('sub2api:canvas-key:7')).toBe('12')
    expect(wrapper.text()).toContain('当前选择未改变')
  })
  it('discards a switch response that arrives after cancellation', async () => {
    const { frame } = await open()
    await chooseSecondKey()
    let resolve!: (value: ReturnType<typeof nextSession>) => void
    mocks.session.mockReturnValue(new Promise(done => { resolve = done }))
    await button('确认切换').trigger('click')
    await button('取消').trigger('click')
    resolve(nextSession())
    await flushPromises()
    expect(wrapper.get('iframe').element).toBe(frame)
    expect(localStorage.getItem('sub2api:canvas-key:7')).toBe('12')
  })
  it('drops an old session response after switching keys', async () => {
    const { frame, send } = await open()
    let resolve!: (value: ReturnType<typeof nextSession>) => void
    mocks.session.mockReturnValueOnce(new Promise(done => { resolve = done }))
    await request(frame, { type: 'session' })
    await chooseSecondKey()
    mocks.session.mockResolvedValue(nextSession())
    await button('确认切换').trigger('click')
    await flushPromises()
    resolve(nextSession(12))
    await flushPromises()
    expect(send.mock.calls.filter(([payload]) => payload.type === 'session')).toHaveLength(0)
    expect(localStorage.getItem('sub2api:canvas-key:7')).toBe('18')
  })
  it('refreshes all pages and excludes invalid keys from the picker', async () => {
    await open()
    mocks.list.mockResolvedValueOnce({ items: [key, { ...key, id: 20, user_id: 99 }, { ...key, id: 21, group_id: null }], total: 6, page_size: 3 })
      .mockResolvedValueOnce({ items: [secondKey, { ...key, id: 22, expires_at: '2020-01-01' }, { ...key, id: 23, quota: 1, quota_used: 1 }], total: 6, page_size: 3 })
    await request(wrapper.get('iframe').element as HTMLIFrameElement, { type: 'select-key' })
    await flushPromises()
    expect(wrapper.findAll('input[type="radio"]').map(input => input.attributes('value'))).toEqual(['12', '18'])
    expect(mocks.list).toHaveBeenLastCalledWith(2, 10, { status: 'active' })
  })
  it('can reopen the picker after an initial empty key list', async () => {
    localStorage.clear()
    mocks.list.mockResolvedValue({ items: [], total: 0, page_size: 10 })
    wrapper = mount(CanvasView, { attachTo: document.body, global: { stubs: { teleport: true, RouterLink: true } } })
    await flushPromises()
    expect(wrapper.text()).toContain('暂无已分组的有效 API Key')
    await button('取消').trigger('click')
    mocks.list.mockResolvedValue({ items: [key], total: 1, page_size: 10 })
    await button('重新选择密钥').trigger('click')
    await flushPromises()
    await button('进入画布').trigger('click')
    await flushPromises()
    expect(wrapper.find('iframe').exists()).toBe(true)
  })
})
