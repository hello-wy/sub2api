import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import MonitorSortDialog from '../MonitorSortDialog.vue'

const { list, updateSortOrder, showSuccess, showError } = vi.hoisted(() => ({
  list: vi.fn(), updateSortOrder: vi.fn(), showSuccess: vi.fn(), showError: vi.fn(),
}))
vi.mock('@/api/admin', () => ({ adminAPI: { channelMonitor: { list, updateSortOrder } } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess, showError }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('vue-draggable-plus', () => ({
  VueDraggable: defineComponent({
    name: 'VueDraggable', props: ['modelValue', 'disabled'], emits: ['update:modelValue'],
    template: '<div><slot /></div>',
  }),
}))
const BaseDialog = defineComponent({
  props: ['show', 'showCloseButton', 'closeOnEscape'],
  template: '<section v-if="show"><slot /><slot name="footer" /></section>',
})
const monitors = [
  { id: 2, name: 'Plus', provider: 'openai', sort_order: 10, enabled: true },
  { id: 4, name: 'Pro', provider: 'openai', sort_order: 20, enabled: true },
  { id: 15, name: 'Ultra', provider: 'openai', sort_order: 30, enabled: false },
]
function mountDialog() {
  return mount(MonitorSortDialog, { props: { show: true }, global: { stubs: { BaseDialog } } })
}
function rowIDs(wrapper: ReturnType<typeof mountDialog>) {
  return wrapper.findAll('[data-monitor-id]').map(row => Number(row.attributes('data-monitor-id')))
}

beforeEach(() => {
  vi.clearAllMocks()
  list.mockReset().mockResolvedValue({ items: monitors, total: 3 })
  updateSortOrder.mockReset().mockResolvedValue(undefined)
})

describe('MonitorSortDialog', () => {
  it('loads all pages and includes disabled monitors without table filters', async () => {
    list.mockReset()
      .mockResolvedValueOnce({ items: monitors.slice(0, 2), total: 3 })
      .mockResolvedValueOnce({ items: monitors.slice(2), total: 3 })
    const wrapper = mountDialog()
    await flushPromises()
    expect(list).toHaveBeenNthCalledWith(1, { page: 1, page_size: 100 }, { signal: expect.any(AbortSignal) })
    expect(list).toHaveBeenNthCalledWith(2, { page: 2, page_size: 100 }, { signal: expect.any(AbortSignal) })
    expect(rowIDs(wrapper)).toEqual([2, 4, 15])
    wrapper.unmount()
  })

  it('saves the dragged order in one request using the original IDs', async () => {
    const wrapper = mountDialog()
    await flushPromises()
    wrapper.findComponent({ name: 'VueDraggable' }).vm.$emit('update:modelValue', [monitors[2], monitors[0], monitors[1]])
    await wrapper.get('[data-testid="save-monitor-order"]').trigger('click')
    await flushPromises()
    expect(updateSortOrder).toHaveBeenCalledTimes(1)
    expect(updateSortOrder).toHaveBeenCalledWith([15, 2, 4])
    expect(wrapper.emitted('saved')).toHaveLength(1)
    expect(wrapper.emitted('close')).toHaveLength(1)
    wrapper.unmount()
  })

  it('supports arrow buttons and cancellation without saving', async () => {
    const wrapper = mountDialog()
    await flushPromises()
    await wrapper.get('[data-monitor-id="4"] [aria-label="admin.channelMonitor.sortOrder.moveUp"]').trigger('click')
    expect(rowIDs(wrapper)).toEqual([4, 2, 15])
    const cancel = wrapper.findAll('button').find(button => button.text() === 'common.cancel')!
    await cancel.trigger('click')
    expect(wrapper.emitted('close')).toHaveLength(1)
    expect(updateSortOrder).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('keeps the draft order after a save failure so it can be retried', async () => {
    updateSortOrder.mockRejectedValueOnce(new Error('temporary failure'))
    const wrapper = mountDialog()
    await flushPromises()
    wrapper.findComponent({ name: 'VueDraggable' }).vm.$emit('update:modelValue', [monitors[2], monitors[0], monitors[1]])
    await wrapper.get('[data-testid="save-monitor-order"]').trigger('click')
    await flushPromises()
    expect(rowIDs(wrapper)).toEqual([15, 2, 4])
    expect(wrapper.emitted('close')).toBeUndefined()
    expect(wrapper.emitted('saved')).toBeUndefined()
    expect(showError).toHaveBeenCalledOnce()
    await wrapper.get('[data-testid="save-monitor-order"]').trigger('click')
    await flushPromises()
    expect(updateSortOrder).toHaveBeenLastCalledWith([15, 2, 4])
    expect(wrapper.emitted('saved')).toHaveLength(1)
    wrapper.unmount()
  })

  it('blocks saving an incomplete list and allows retrying its load', async () => {
    list.mockRejectedValueOnce(new Error('load failed'))
    const wrapper = mountDialog()
    await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="save-monitor-order"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[role="alert"] button').trigger('click')
    await flushPromises()
    expect(rowIDs(wrapper)).toEqual([2, 4, 15])
    wrapper.unmount()
  })

  it('prevents duplicate saves and locks the draft while saving', async () => {
    let finish!: () => void
    updateSortOrder.mockReturnValueOnce(new Promise<void>(resolve => { finish = resolve }))
    const wrapper = mountDialog()
    await flushPromises()
    await wrapper.get('[data-testid="save-monitor-order"]').trigger('click')
    await wrapper.get('[data-testid="save-monitor-order"]').trigger('click')
    expect(updateSortOrder).toHaveBeenCalledOnce()
    expect(wrapper.findComponent({ name: 'VueDraggable' }).props('disabled')).toBe(true)
    expect(wrapper.findComponent(BaseDialog).props('closeOnEscape')).toBe(false)
    finish()
    await flushPromises()
    wrapper.unmount()
  })
})
