import { computed, onScopeDispose, ref, watch } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { list, type UserMonitorView } from '@/api/channelMonitor'
import { getMatrix, type MonitorMatrixResponse } from '@/api/channelMonitorV2'
import { getChannelMonitorMode, getChannelMonitorRefreshIntervalSeconds, isChannelMonitorRouteEnabled } from '@/utils/featureFlags'
import { plazaV1Status, plazaV2Status } from '@/utils/model-plaza'
import type { ModelPlazaGroup, PlazaModel } from '@/api/modelPlaza'

export function usePlazaStatus() {
  const auth = useAuthStore()
  const app = useAppStore()
  const matrix = ref<MonitorMatrixResponse | null>(null)
  const probes = ref<UserMonitorView[]>([])
  const failed = ref(false)
  const loading = ref(false)
  const mode = computed(() => getChannelMonitorMode())
  let controller: AbortController | undefined
  let timer: ReturnType<typeof setTimeout> | undefined
  let generation = 0
  const enabled = computed(() => auth.isAuthenticated && !!app.cachedPublicSettings && isChannelMonitorRouteEnabled())
  const stop = watch([enabled, mode, () => app.cachedPublicSettings?.channel_monitor_default_interval_seconds, () => auth.user?.id], () => {
    const current = ++generation
    controller?.abort()
    clearTimeout(timer)
    matrix.value = null
    probes.value = []
    failed.value = false
    loading.value = false
    if (!enabled.value) return
    async function refresh() {
      controller = new AbortController()
      loading.value = true
      try {
        if (mode.value === 'v2') {
          const data = await getMatrix({ range: '24h', platforms: [], groupIds: [], models: [] }, 'platform_group_model', false, controller.signal)
          if (current === generation) matrix.value = data
        } else {
          const data = await list({ signal: controller.signal })
          if (current === generation) probes.value = data.items
        }
        if (current === generation) failed.value = false
      } catch {
        if (current === generation) {
          failed.value = true
          matrix.value = null
          probes.value = []
        }
      } finally {
        if (current === generation) {
          loading.value = false
          timer = setTimeout(refresh, Math.max(30, getChannelMonitorRefreshIntervalSeconds()) * 1000)
        }
      }
    }
    void refresh()
  }, { immediate: true })
  onScopeDispose(() => { stop(); generation++; controller?.abort(); clearTimeout(timer) })
  const statusFor = (model: PlazaModel, group: ModelPlazaGroup) => mode.value === 'v2'
    ? plazaV2Status(model, group, matrix.value)
    : plazaV1Status(model, group, probes.value)
  return { statusFor, failed, loading, enabled, mode }
}
