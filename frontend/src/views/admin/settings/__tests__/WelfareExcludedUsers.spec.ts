import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import WelfareExcludedUsers from '../WelfareExcludedUsers.vue'

const { listUsers, getUser } = vi.hoisted(() => ({ listUsers: vi.fn(), getUser: vi.fn() }))
vi.mock('@/api', () => ({ adminAPI: { users: { list: listUsers, getById: getUser } } }))

const user = (id: number) => ({ id, email: `user${id}@example.com`, username: `User ${id}`, role: 'user' })
const mountEditor = (ids: number[] = []) => mount(WelfareExcludedUsers, {
  props: { modelValue: ids },
  global: {
    plugins: [createI18n({ legacy: false, locale: 'zh-CN', messages: {} })],
    stubs: { BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' } }
  }
})

describe('WelfareExcludedUsers', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getUser.mockImplementation(async (id: number) => user(id))
    listUsers.mockResolvedValue({ items: [user(7), user(9)], total: 2 })
  })

  it('loads stored users, confirms additions, prevents duplicates and removes users', async () => {
    const wrapper = mountEditor([7])
    await flushPromises()
    expect(getUser).toHaveBeenCalledWith(7, true)
    expect(wrapper.get('[data-testid="welfare-excluded-7"]').text()).toContain('user7@example.com')
    await wrapper.get('[data-testid="welfare-exclusions-open"]').trigger('click')
    await wrapper.get('[data-testid="welfare-exclusions-query"]').setValue('user')
    await wrapper.get('[data-testid="welfare-exclusions-search"]').trigger('click')
    await flushPromises()
    expect(listUsers).toHaveBeenCalledWith(1, 20, { search: 'user', role: 'user' }, { signal: expect.any(AbortSignal) })
    expect(wrapper.get('[data-testid="welfare-select-7"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="welfare-select-9"]').setValue(true)
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    await wrapper.get('[data-testid="welfare-exclusions-confirm"]').trigger('click')
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([[7, 9]])
    await wrapper.setProps({ modelValue: [7, 9] })
    await wrapper.get('[data-testid="welfare-excluded-7"] button').trigger('click')
    expect(wrapper.emitted('update:modelValue')?.[1]).toEqual([[9]])
  })

  it('shows API failures and retries loading stored users', async () => {
    getUser.mockRejectedValueOnce(new Error('Unable to load user'))
    const wrapper = mountEditor([7])
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('Unable to load user')
    await wrapper.get('button.btn-secondary.mt-2').trigger('click')
    await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    listUsers.mockRejectedValueOnce(new Error('Search failed'))
    await wrapper.get('[data-testid="welfare-exclusions-open"]').trigger('click')
    await wrapper.get('[data-testid="welfare-exclusions-search"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('Search failed')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })

  it('paginates search results and keeps selections across pages', async () => {
    listUsers.mockResolvedValueOnce({ items: [user(7)], total: 21 })
    const wrapper = mountEditor()
    await wrapper.get('[data-testid="welfare-exclusions-open"]').trigger('click')
    await wrapper.get('[data-testid="welfare-exclusions-search"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="welfare-select-7"]').setValue(true)
    listUsers.mockResolvedValueOnce({ items: [user(9)], total: 21 })
    await wrapper.findAll('button').find(button => button.text() === '下一页')!.trigger('click')
    await flushPromises()
    expect(listUsers.mock.calls[1][0]).toBe(2)
    await wrapper.get('[data-testid="welfare-select-9"]').setValue(true)
    await wrapper.get('[data-testid="welfare-exclusions-confirm"]').trigger('click')
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([[7, 9]])
  })
})
