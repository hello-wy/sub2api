import { defineStore } from 'pinia'
import { ref } from 'vue'
import { apiClient } from '@/api/client'

export type CanvasSettings = { enabled: boolean; url: string; api_base_url?: string; effective_api_base_url?: string }
export const useCanvasStore = defineStore('canvas', () => {
  const settings = ref<CanvasSettings>({ enabled: false, url: '' })
  let pending: Promise<CanvasSettings> | undefined
  async function fetchSettings(): Promise<CanvasSettings> {
    if (pending) return pending
    pending = apiClient.get<CanvasSettings>('/canvas/settings').then(({ data }) => {
      settings.value = data
      return data
    }).catch(error => {
      settings.value = { enabled: false, url: '' }
      throw error
    }).finally(() => { pending = undefined })
    return pending
  }
  async function saveSettings(value: CanvasSettings) {
    const { data } = await apiClient.put<CanvasSettings>('/admin/settings/canvas', value)
    settings.value = data
  }
  return { settings, fetchSettings, saveSettings }
})
