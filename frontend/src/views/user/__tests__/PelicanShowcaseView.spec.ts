import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import PelicanShowcaseView from '../PelicanShowcaseView.vue'
import type { PelicanShowcaseItem, PelicanShowcaseView as ShowcaseData } from '@/api/pelicanShowcase'

const { getShowcase, getShowcaseItem, removeShowcaseItem, showError, showSuccess, auth } = vi.hoisted(() => ({
  getShowcase: vi.fn(),
  getShowcaseItem: vi.fn(),
  removeShowcaseItem: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  auth: { isAdmin: false },
}))
vi.mock('@/api/pelicanShowcase', () => ({ getShowcase, getShowcaseItem, removeShowcaseItem }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess }) }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => auth }))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string, named?: Record<string, unknown>) => (named ? `${key} ${JSON.stringify(named)}` : key) }),
}))

const item = (id: number, groupId: number): PelicanShowcaseItem => ({
  id, group_id: groupId, model_id: 'gpt-6-astra', reasoning_effort: 'high', latency_ms: 42300,
  generated_at: '2026-09-24T08:30:00Z',
})
const showcase = (overrides: Partial<ShowcaseData> = {}): ShowcaseData => ({
  enabled: true,
  max_items: 20,
  retention_days: 7,
  groups: [
    { id: 1, name: 'Claude Max', platform: 'anthropic', items: Array.from({ length: 12 }, (_, i) => item(100 + i, 1)) },
    { id: 2, name: 'GPT Plus', platform: 'openai', items: [item(200, 2)] },
    { id: 3, name: 'Empty', platform: 'gemini', items: [] },
  ],
  ...overrides,
})

const mountView = () => mount(PelicanShowcaseView, {
  global: {
    stubs: {
      AppLayout: { template: '<div><slot /></div>' },
      Icon: true,
      PlatformIcon: true,
      EmptyState: { props: ['title', 'description'], template: '<div class="empty-state">{{ title }}</div>' },
      ConfirmDialog: {
        props: ['show'], emits: ['confirm', 'cancel'],
        template: '<div v-if="show" class="confirm"><button class="confirm-yes" @click="$emit(\'confirm\')" /></div>',
      },
      BaseDialog: {
        props: ['show', 'title'], emits: ['close'],
        template: '<div v-if="show" class="dialog"><h3>{{ title }}</h3><slot /><slot name="footer" /></div>',
      },
    },
  },
})

// The shared test setup installs an observer that never fires; cards here are on screen.
class OnScreenObserver {
  constructor(private readonly callback: IntersectionObserverCallback) {}
  observe(target: Element) {
    this.callback([{ isIntersecting: true, target } as IntersectionObserverEntry], this as unknown as IntersectionObserver)
  }
  disconnect() {}
  unobserve() {}
}

let wrapper: ReturnType<typeof mountView>
beforeEach(() => {
  vi.stubGlobal('IntersectionObserver', OnScreenObserver)
  getShowcase.mockReset()
  getShowcaseItem.mockReset().mockImplementation(async (id: number) => ({
    ...item(id, 0),
    response_text: id === 201 ? '21' : `<svg data-item="${id}"></svg>`,
  }))
  removeShowcaseItem.mockReset().mockResolvedValue(undefined)
  showError.mockReset()
  showSuccess.mockReset()
  auth.isAdmin = false
})
afterEach(() => {
  wrapper?.unmount()
  vi.unstubAllGlobals()
})

describe('PelicanShowcaseView', () => {
  it('explains that the gallery is closed without asking for items', async () => {
    getShowcase.mockResolvedValue(showcase({ enabled: false, groups: [] }))
    wrapper = mountView()
    await flushPromises()
    expect(wrapper.get('.empty-state').text()).toBe('pelicanShowcase.disabled.title')
    expect(wrapper.findAll('[data-testid="pelican-showcase-card"]')).toHaveLength(0)
    expect(getShowcaseItem).not.toHaveBeenCalled()
  })

  it('lists every group with the gallery rules and loads visible cards in a sandbox', async () => {
    getShowcase.mockResolvedValue(showcase())
    wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-testid="showcase-keep-rule"]').text()).toContain('"count":20')
    expect(wrapper.get('[data-testid="showcase-retention-rule"]').text()).toContain('"days":7')
    expect(wrapper.findAll('[role="tab"]').map((tab) => tab.text())).toEqual([
      'pelicanShowcase.allGroups', 'Claude Max12', 'GPT Plus1', 'Empty0',
    ])
    expect(wrapper.get('[data-testid="showcase-group-3"]').text()).toContain('pelicanShowcase.groupEmpty')

    const cards = wrapper.findAll('[data-testid="pelican-showcase-card"]')
    expect(cards).toHaveLength(11) // 10 of the first group, then the second group's only item
    expect(getShowcaseItem).toHaveBeenCalledTimes(11)
    const frame = cards[0].get('iframe')
    expect(frame.attributes('sandbox')).toBe('allow-scripts')
    expect(frame.attributes('referrerpolicy')).toBe('no-referrer')
    expect(frame.attributes('srcdoc')).toContain('Content-Security-Policy')
    expect(frame.attributes('srcdoc')).toContain('data-item="100"')
    expect(cards[0].text()).toContain('gpt-6-astra')
    expect(cards[0].text()).toContain('"seconds":"42.3"')
    expect(cards[0].text()).toContain('pelicanShowcase.efforts.high')

    await wrapper.get('[data-testid="showcase-more-1"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="showcase-group-1"]').findAll('[data-testid="pelican-showcase-card"]')).toHaveLength(12)
    expect(wrapper.find('[data-testid="showcase-more-1"]').exists()).toBe(false)

    await wrapper.get('[data-testid="showcase-tab-2"]').trigger('click')
    expect(wrapper.find('[data-testid="showcase-group-1"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="showcase-group-2"]').exists()).toBe(true)
  })

  it('fetches HTML only for cards that reach the viewport', async () => {
    vi.unstubAllGlobals() // back to the setup observer, which never reports a card as visible
    getShowcase.mockResolvedValue(showcase())
    wrapper = mountView()
    await flushPromises()
    expect(wrapper.findAll('[data-testid="pelican-showcase-card"]')).toHaveLength(11)
    expect(getShowcaseItem).not.toHaveBeenCalled()
  })

  it('distinguishes real group tests from legacy account samples', async () => {
    getShowcase.mockResolvedValue(showcase({
      groups: [{ id: 2, name: 'GPT Plus', platform: 'openai', items: [
        { ...item(200, 2), source_scope: 'group' },
        { ...item(201, 2), source_scope: 'account' },
        item(202, 2),
      ] }],
    }))
    wrapper = mountView()
    await flushPromises()
    expect(wrapper.findAll('[data-testid="showcase-source"]').map((badge) => badge.text())).toEqual([
      'pelicanShowcase.sourceGroup', 'pelicanShowcase.sourceAccount', 'pelicanShowcase.sourceAccount',
    ])
  })

  it('scales the full result to the thumbnail width while keeping the enlarged preview unscaled', async () => {
    let resizeThumbnail: (width: number) => void = () => {}
    vi.stubGlobal('ResizeObserver', class {
      constructor(private readonly callback: ResizeObserverCallback) {}
      observe(target: Element) {
        resizeThumbnail = (width) => this.callback(
          [{ target, contentRect: { width } } as ResizeObserverEntry],
          this as unknown as ResizeObserver,
        )
        resizeThumbnail(120)
      }
      disconnect() {}
      unobserve() {}
    })
    getShowcase.mockResolvedValue(showcase({
      groups: [{ id: 2, name: 'GPT Plus', platform: 'openai', items: [item(200, 2)] }],
    }))
    wrapper = mountView()
    await flushPromises()

    const card = wrapper.get('[data-testid="pelican-showcase-card"]')
    const thumbnail = card.get('iframe').element as HTMLIFrameElement
    expect(thumbnail.style.width).toBe('800px')
    expect(thumbnail.style.height).toBe('600px')
    expect(thumbnail.style.transform).toBe('scale(0.15)')

    resizeThumbnail(160)
    await flushPromises()
    expect(thumbnail.style.transform).toBe('scale(0.2)')

    await card.get('button').trigger('click')
    const preview = wrapper.get('[data-testid="showcase-preview"] iframe').element as HTMLIFrameElement
    expect(preview.style.transform).toBe('')
    expect(preview.srcdoc).toBe(thumbnail.srcdoc)
  })

  it('shows a readable state for output without HTML and for failed loads', async () => {
    getShowcase.mockResolvedValue(showcase({
      groups: [{ id: 2, name: 'GPT Plus', platform: 'openai', items: [item(201, 2), item(202, 2)] }],
    }))
    getShowcaseItem.mockImplementation(async (id: number) => {
      if (id === 202) throw new Error('boom')
      return { ...item(id, 2), response_text: '21' }
    })
    wrapper = mountView()
    await flushPromises()
    const cards = wrapper.findAll('[data-testid="pelican-showcase-card"]')
    expect(cards[0].find('iframe').exists()).toBe(false)
    expect(cards[0].text()).toContain('pelicanShowcase.invalidHtml')
    expect(cards[1].text()).toContain('pelicanShowcase.itemLoadError')

    // Refresh retries a failed card even though it already reported being on screen.
    getShowcaseItem.mockImplementation(async (id: number) => ({ ...item(id, 2), response_text: `<svg data-item="${id}"></svg>` }))
    await wrapper.get('button[aria-label="common.refresh"]').trigger('click')
    await flushPromises()
    expect(getShowcaseItem).toHaveBeenCalledTimes(3)
    expect(wrapper.findAll('[data-testid="pelican-showcase-card"]')[1].get('iframe').attributes('srcdoc')).toContain('data-item="202"')
  })

  it('previews a card, and only admins can take it down', async () => {
    getShowcase.mockResolvedValue(showcase())
    wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="showcase-group-2"] [data-testid="pelican-showcase-card"] button').trigger('click')
    await flushPromises()
    const dialog = wrapper.get('[data-testid="showcase-preview"]')
    expect(dialog.get('iframe').attributes('srcdoc')).toContain('data-item="200"')
    expect(dialog.get('iframe').attributes('sandbox')).toBe('allow-scripts')
    expect(wrapper.find('[data-testid="showcase-remove"]').exists()).toBe(false)
    wrapper.unmount()

    auth.isAdmin = true
    wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="showcase-group-2"] [data-testid="pelican-showcase-card"] button').trigger('click')
    await wrapper.get('[data-testid="showcase-remove"]').trigger('click')
    await wrapper.get('.confirm-yes').trigger('click')
    await flushPromises()
    expect(removeShowcaseItem).toHaveBeenCalledWith(200)
    expect(showSuccess).toHaveBeenCalledWith('pelicanShowcase.removed')
    expect(wrapper.find('[data-testid="showcase-preview"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="showcase-group-2"]').text()).toContain('pelicanShowcase.groupEmpty')
  })
})
