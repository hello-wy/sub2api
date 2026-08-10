import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const apiMocks = vi.hoisted(() => ({
  adjustLotteryTickets: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: {
      adjustLotteryTickets: apiMocks.adjustLotteryTickets,
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: apiMocks.showError,
    showSuccess: apiMocks.showSuccess,
  }),
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

vi.mock('@/components/common/BaseDialog.vue', () => ({
  default: {
    name: 'BaseDialog',
    props: ['show', 'title', 'width'],
    template: '<div v-if="show"><slot /><slot name="footer" /></div>',
  },
}))

import UserLotteryTicketsModal from '../UserLotteryTicketsModal.vue'

const user = { id: 99, email: 'tickets@example.com' } as any

async function mountModal(availableTickets = 10) {
  const wrapper = mount(UserLotteryTicketsModal, {
    props: { show: false, user, availableTickets },
  })
  await wrapper.setProps({ show: true })
  await flushPromises()
  return wrapper
}

async function setTarget(wrapper: ReturnType<typeof mount>, value: number) {
  await wrapper.find('input[type="number"]').setValue(value)
}

beforeEach(() => {
  vi.clearAllMocks()
  apiMocks.adjustLotteryTickets.mockResolvedValue({ available_tickets: 0 })
})

describe('UserLotteryTicketsModal', () => {
  it('maps a higher target to an add adjustment', async () => {
    const wrapper = await mountModal()
    await setTarget(wrapper, 13)
    await wrapper.find('textarea').setValue('manual correction')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(apiMocks.adjustLotteryTickets).toHaveBeenCalledWith(
      99,
      3,
      'add',
      'manual correction',
      expect.any(String),
    )
    expect(wrapper.emitted('success')).toHaveLength(1)
    expect(wrapper.emitted('close')).toHaveLength(1)
  })

  it('maps a lower target to a subtract adjustment', async () => {
    const wrapper = await mountModal()
    await setTarget(wrapper, 4)
    await wrapper.find('textarea').setValue('manual correction')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(apiMocks.adjustLotteryTickets).toHaveBeenCalledWith(
      99,
      6,
      'subtract',
      'manual correction',
      expect.any(String),
    )
  })

  it('closes without a request when the target is unchanged', async () => {
    const wrapper = await mountModal()
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(apiMocks.adjustLotteryTickets).not.toHaveBeenCalled()
    expect(wrapper.emitted('success')).toBeUndefined()
    expect(wrapper.emitted('close')).toHaveLength(1)
  })

  it('rejects an invalid target or missing reason without a request', async () => {
    const wrapper = await mountModal()
    await setTarget(wrapper, -1)
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(apiMocks.adjustLotteryTickets).not.toHaveBeenCalled()
    expect(apiMocks.showError).toHaveBeenCalledWith('admin.users.lotteryTicketTargetRequired')

    await setTarget(wrapper, 11)
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(apiMocks.adjustLotteryTickets).not.toHaveBeenCalled()
    expect(apiMocks.showError).toHaveBeenCalledWith('admin.users.lotteryTicketReasonRequired')
  })
})
