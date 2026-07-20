import { readonly, ref } from 'vue'
import { checkinAPI } from '@/api'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'

const checkinReminderVisible = ref(false)
let refreshPromise: Promise<void> | null = null

export function useCheckinReminder() {
  const appStore = useAppStore()
  const authStore = useAuthStore()

  async function refreshCheckinReminder(): Promise<void> {
    if (!authStore.isAuthenticated || appStore.backendModeEnabled) {
      checkinReminderVisible.value = false
      return
    }

    if (!refreshPromise) {
      refreshPromise = (async () => {
        try {
          const status = await checkinAPI.getCheckinStatus()
          checkinReminderVisible.value = authStore.isAuthenticated && !status.already_checked_in
        } catch {
          checkinReminderVisible.value = false
        } finally {
          refreshPromise = null
        }
      })()
    }

    await refreshPromise
  }

  function markCheckinCompleted(): void {
    checkinReminderVisible.value = false
  }

  return {
    checkinReminderVisible: readonly(checkinReminderVisible),
    refreshCheckinReminder,
    markCheckinCompleted,
  }
}
