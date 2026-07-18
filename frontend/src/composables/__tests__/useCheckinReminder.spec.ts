import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useCheckinReminder } from '@/composables/useCheckinReminder'

const mocks = vi.hoisted(() => ({
  getCheckinStatus: vi.fn(),
}))

vi.mock('@/api', () => ({
  authAPI: {},
  checkinAPI: {
    getCheckinStatus: (...args: unknown[]) => mocks.getCheckinStatus(...args),
  },
  isTotp2FARequired: () => false,
}))

const user = {
  id: 1,
  username: 'tester',
  email: 'tester@example.com',
  role: 'user',
  balance: 12.5,
  concurrency: 1,
  status: 'active',
  allowed_groups: null,
  created_at: '2026-01-01',
  updated_at: '2026-01-01',
}

describe('useCheckinReminder', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mocks.getCheckinStatus.mockReset()
    useCheckinReminder().markCheckinCompleted()
  })

  it('deduplicates concurrent status requests and shows the reminder before check-in', async () => {
    const authStore = useAuthStore()
    authStore.token = 'token'
    authStore.user = user as typeof authStore.user
    mocks.getCheckinStatus.mockResolvedValue({ already_checked_in: false })

    const { checkinReminderVisible, refreshCheckinReminder } = useCheckinReminder()
    await Promise.all([refreshCheckinReminder(), refreshCheckinReminder()])

    expect(mocks.getCheckinStatus).toHaveBeenCalledTimes(1)
    expect(checkinReminderVisible.value).toBe(true)
  })

  it('clears the shared reminder immediately after check-in', async () => {
    const authStore = useAuthStore()
    authStore.token = 'token'
    authStore.user = user as typeof authStore.user
    mocks.getCheckinStatus.mockResolvedValue({ already_checked_in: false })

    const { checkinReminderVisible, refreshCheckinReminder, markCheckinCompleted } = useCheckinReminder()
    await refreshCheckinReminder()
    markCheckinCompleted()

    expect(checkinReminderVisible.value).toBe(false)
  })

  it('does not request status for unauthenticated users', async () => {
    useAppStore()
    const { checkinReminderVisible, refreshCheckinReminder } = useCheckinReminder()
    await refreshCheckinReminder()

    expect(mocks.getCheckinStatus).not.toHaveBeenCalled()
    expect(checkinReminderVisible.value).toBe(false)
  })
})
