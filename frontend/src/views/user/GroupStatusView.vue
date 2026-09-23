<template>
  <AppLayout>
    <div class="space-y-6 pb-10">
      <header class="space-y-5 border-b border-gray-200 pb-6 dark:border-dark-700">
        <div class="flex flex-wrap items-start justify-between gap-4">
          <div>
            <p class="mb-2 flex items-center gap-2 text-xs font-medium tracking-wide text-gray-500 dark:text-gray-400">
              <span class="h-2 w-2 rounded-full" :class="response && !error && !disabled ? 'bg-emerald-500' : 'bg-gray-400'" aria-hidden="true" />
              {{ t('groupStatus.serviceTelemetry') }}
            </p>
            <h1 class="text-2xl font-bold tracking-tight text-gray-950 dark:text-white sm:text-3xl">{{ t('groupStatus.title') }}</h1>
            <p class="mt-3 max-w-3xl text-sm leading-relaxed text-gray-500 dark:text-gray-400">{{ t('groupStatus.description') }}</p>
          </div>
          <router-link to="/monitor" class="btn btn-secondary btn-sm">{{ t('groupStatus.deepMonitor') }}</router-link>
        </div>
        <div v-if="!disabled" class="flex flex-wrap items-center justify-between gap-3">
          <div class="flex max-w-full items-center gap-2">
            <div class="flex flex-wrap gap-1 rounded-2xl bg-gray-100 p-1 dark:bg-dark-800" role="group" :aria-label="t('groupStatus.timeRange')">
              <button v-for="option in ranges" :key="option.value" type="button" class="rounded-xl px-3 py-2 text-sm font-medium transition-colors focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary-500" :class="range === option.value ? 'bg-white text-primary-700 shadow-sm dark:bg-dark-600 dark:text-primary-300' : 'text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-white'" :aria-pressed="range === option.value" :data-testid="`range-${option.value}`" @click="range = option.value">
                {{ option.label }}
              </button>
            </div>
            <button type="button" class="btn btn-secondary btn-icon h-10 w-10 shrink-0 rounded-full" :disabled="loading" :aria-label="t('groupStatus.refresh')" :title="t('groupStatus.refresh')" @click="reload">
              <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
            </button>
          </div>
          <div class="flex w-full flex-wrap items-center gap-2 sm:w-auto">
            <label class="min-w-0 flex-1 sm:w-52">
              <span class="sr-only">{{ t('groupStatus.model') }}</span>
              <select v-model="model" class="input w-full" data-testid="model-filter">
                <option value="">{{ t('groupStatus.allModels') }}</option>
                <option v-for="name in modelOptions" :key="name" :value="name">{{ name }}</option>
              </select>
            </label>
            <label class="min-w-0 flex-1 sm:w-44">
              <span class="sr-only">{{ t('groupStatus.search') }}</span>
              <input v-model="search" type="search" class="input w-full" :placeholder="t('groupStatus.search')" data-testid="group-search" />
            </label>
          </div>
        </div>
      </header>

      <div v-if="!disabled" class="flex flex-wrap items-center justify-between gap-2 text-xs text-gray-500 dark:text-gray-400" aria-live="polite">
        <span>{{ t('groupStatus.groupCount', { count: visibleGroups.length }) }}</span>
        <span v-if="loading">{{ t('groupStatus.loading') }}</span>
        <span v-else-if="response?.updated_at">{{ t('groupStatus.updatedAt', { time: formatGroupTime(response.updated_at, locale) }) }}</span>
      </div>
      <p v-if="stale" class="rounded-xl bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:bg-amber-950/30 dark:text-amber-300" role="status">{{ t('groupStatus.stale') }}</p>

      <section v-if="disabled" class="rounded-2xl border border-gray-200 bg-gray-50 p-8 text-center dark:border-dark-700 dark:bg-dark-800" role="status" data-testid="group-status-disabled">
        <h2 class="font-semibold text-gray-900 dark:text-white">{{ t('groupStatus.disabledTitle') }}</h2>
        <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">{{ t(isAdmin ? 'groupStatus.disabledAdmin' : 'groupStatus.disabledUser') }}</p>
        <router-link v-if="isAdmin" to="/admin/channels/monitor" class="btn btn-secondary mt-4">{{ t('groupStatus.enableSettings') }}</router-link>
      </section>
      <section v-else-if="error" class="rounded-2xl border border-rose-200 bg-rose-50 p-8 text-center dark:border-rose-900/50 dark:bg-rose-950/20" role="alert">
        <p class="text-sm text-rose-700 dark:text-rose-300">{{ t('groupStatus.loadFailed') }}</p>
        <button type="button" class="btn btn-secondary mt-4" @click="reload">{{ t('groupStatus.retry') }}</button>
      </section>
      <div v-else-if="loading && !response" class="grid gap-4 md:grid-cols-2 2xl:grid-cols-3" aria-busy="true">
        <div v-for="index in 6" :key="index" class="h-72 animate-pulse rounded-3xl bg-gray-100 dark:bg-dark-800" />
      </div>
      <section v-else-if="!visibleGroups.length" class="rounded-3xl border border-dashed border-gray-300 px-6 py-16 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400">
        {{ search || model ? t('groupStatus.noMatches') : t('groupStatus.empty') }}
      </section>
      <div v-else class="grid gap-4 md:grid-cols-2 2xl:grid-cols-3" :aria-busy="loading">
        <GroupStatusCard v-for="group in visibleGroups" :key="group.group_id" :group="group" :admin="isAdmin" @manage="selectedGroup = $event" />
      </div>
      <div v-if="!disabled" class="flex flex-wrap items-center justify-center gap-x-5 gap-y-2 text-xs text-gray-500 dark:text-gray-400" :aria-label="t('groupStatus.legend')">
        <span v-for="state in serviceStates" :key="state" class="inline-flex items-center gap-1.5"><span class="h-2.5 w-2.5 rounded-full" :class="stateColor(state)" aria-hidden="true" />{{ t(`groupStatus.status.${state}`) }}</span>
      </div>
    </div>
    <GroupProbeDialog v-if="isAdmin && selectedGroup && !disabled" :key="selectedGroup.group_id" :group="selectedGroup" :models="modelOptions" :selected-model="model" @close="selectedGroup = null" @changed="reload" />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAuthStore } from '@/stores/auth'
import { getGroupStatus, isGroupStatusDisabledError } from '@/api/groupStatus'
import { isChannelMonitorV2Mode } from '@/utils/featureFlags'
import type { GroupServiceStatus, GroupStatusRange, GroupStatusResponse } from '@/api/groupStatus'
import GroupStatusCard from '@/features/group-status/GroupStatusCard.vue'
import GroupProbeDialog from '@/features/group-status/GroupProbeDialog.vue'
import { formatGroupTime, serviceStates, stateColor } from '@/features/group-status/format'

const { t, locale } = useI18n()
const authStore = useAuthStore()
const isAdmin = computed(() => authStore.isAdmin)
const range = ref<GroupStatusRange>('24h')
const model = ref('')
const search = ref('')
const response = ref<GroupStatusResponse | null>(null)
const loading = ref(false)
const error = ref(false)
const serverDisabled = ref(false)
const modeEnabled = computed(() => isChannelMonitorV2Mode())
const disabled = computed(() => !modeEnabled.value || serverDisabled.value)
const now = ref(Date.now())
const selectedGroup = ref<GroupServiceStatus | null>(null)
const ranges = computed<Array<{ value: GroupStatusRange; label: string }>>(() => [
  { value: '24h', label: t('groupStatus.range24h') },
  { value: '7d', label: t('groupStatus.range7d') },
  { value: '15d', label: t('groupStatus.range15d') },
  { value: '30d', label: t('groupStatus.range30d') },
])
const modelOptions = computed(() => [...new Set([...(response.value?.models || []), ...(model.value ? [model.value] : [])])].sort())
const visibleGroups = computed(() => {
  const needle = search.value.trim().toLocaleLowerCase()
  return (response.value?.groups || []).filter(group => !needle || group.group_name.toLocaleLowerCase().includes(needle))
})
const stale = computed(() => {
  if (disabled.value || !response.value?.updated_at) return false
  return now.value - new Date(response.value.updated_at).getTime() > 10 * 60 * 1000
})
let controller: AbortController | undefined
let refreshTimer: ReturnType<typeof setTimeout> | undefined
let generation = 0
let disposed = false

async function reload() {
  if (disposed) return
  controller?.abort()
  controller = new AbortController()
  const current = ++generation
  if (!modeEnabled.value) {
    response.value = null
    selectedGroup.value = null
    loading.value = false
    error.value = false
    return
  }
  loading.value = true
  error.value = false
  try {
    const data = await getGroupStatus(range.value, model.value, isAdmin.value, controller.signal)
    if (disposed || current !== generation) return
    response.value = data
    serverDisabled.value = data.enabled === false
    now.value = Date.now()
    if (selectedGroup.value && !data.groups.some(group => group.group_id === selectedGroup.value?.group_id)) selectedGroup.value = null
  } catch (cause) {
    if (disposed || current !== generation) return
    // Discard previous permissions' data instead of keeping cards after an authorization failure.
    response.value = null
    selectedGroup.value = null
    serverDisabled.value = isGroupStatusDisabledError(cause)
    error.value = !serverDisabled.value
  } finally {
    if (!disposed && current === generation) loading.value = false
  }
}
function scheduleRefresh() {
  refreshTimer = setTimeout(async () => {
    now.value = Date.now()
    if (!document.hidden) await reload()
    if (!disposed) scheduleRefresh()
  }, 60000)
}
function onVisibilityChange() { if (!document.hidden) void reload() }
watch([range, model, isAdmin, modeEnabled], () => {
  response.value = null
  selectedGroup.value = null
  serverDisabled.value = false
  void reload()
}, { immediate: true })
onMounted(() => {
  scheduleRefresh()
  document.addEventListener('visibilitychange', onVisibilityChange)
})
onUnmounted(() => {
  disposed = true
  controller?.abort()
  if (refreshTimer) clearTimeout(refreshTimer)
  document.removeEventListener('visibilitychange', onVisibilityChange)
})
</script>
