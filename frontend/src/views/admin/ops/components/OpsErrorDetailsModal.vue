<script setup lang="ts">
import { computed, ref, watch, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import * as groupsAPI from '@/api/admin/groups'
import * as accountsAPI from '@/api/admin/accounts'
import * as usersAPI from '@/api/admin/users'
import Select from '@/components/common/Select.vue'
import OpsErrorLogTable from './OpsErrorLogTable.vue'
import { opsAPI, type OpsErrorLog } from '@/api/admin/ops'
import { buildOpsErrorTimeParams } from '../utils/opsErrorParams'

interface Props {
  show: boolean
  timeRange: string
  customStartTime?: string | null
  customEndTime?: string | null
  platform?: string
  groupId?: number | null
  errorType: 'request' | 'upstream'
  resumeState?: boolean
}

const props = defineProps<Props>()
const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
  (e: 'openErrorDetail', errorId: number): void
}>()

const { t } = useI18n()


const loading = ref(false)
const rows = ref<OpsErrorLog[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(10)

const q = ref('')
const statusCode = ref<number | 'other' | null>(null)
const phase = ref<string>('')
const errorOwner = ref<string>('')
const viewMode = ref<'errors' | 'excluded' | 'all'>('errors')


type EntityKind = 'group' | 'account' | 'user'
type EntityOption = { value: number; label: string }
const groupId = ref<number | null>(props.groupId ?? null)
const accountId = ref<number | null>(null)
const userId = ref<number | null>(null)
const entityIds = { group: groupId, account: accountId, user: userId }
const entityOptions = ref<Record<EntityKind, EntityOption[]>>({ group: [], account: [], user: [] })
const entityLoading = ref({ group: false, account: false, user: false })
const entityErrors = ref({ group: false, account: false, user: false })
const entityKinds: EntityKind[] = ['group', 'account', 'user']
const entityRequests = { group: 0, account: 0, user: 0 }
const loadError = ref(false)
let requestVersion = 0
let searchTimeout: ReturnType<typeof setTimeout> | undefined

async function searchEntity(kind: EntityKind, search = '') {
  const version = ++entityRequests[kind]
  entityLoading.value[kind] = true
  entityErrors.value[kind] = false
  try {
    // Do not restrict by current membership: historical errors retain their original group.
    const result = kind === 'group'
      ? await groupsAPI.list(1, 50, { search })
      : kind === 'account'
        ? await accountsAPI.list(1, 50, { search, lite: 'true' })
        : await usersAPI.list(1, 50, { search, include_subscriptions: false })
    if (version !== entityRequests[kind]) return
    const selected = entityOptions.value[kind].find(option => option.value === entityIds[kind].value)
    const options = result.items.map(item => ({
      value: item.id,
      label: `${'email' in item ? item.email : item.name} · #${item.id}`
    }))
    if (selected && !options.some(option => option.value === selected.value)) options.unshift(selected)
    entityOptions.value[kind] = options
  } catch {
    if (version === entityRequests[kind]) entityErrors.value[kind] = true
  } finally {
    if (version === entityRequests[kind]) entityLoading.value[kind] = false
  }
}

const modalTitle = computed(() => {
  return props.errorType === 'upstream' ? t('admin.ops.errorDetails.upstreamErrors') : t('admin.ops.errorDetails.requestErrors')
})

const statusCodeSelectOptions = computed(() => {
  const codes = [400, 401, 403, 404, 409, 422, 429, 500, 502, 503, 504, 529]
  return [
    { value: null, label: t('common.all') },
    ...codes.map((c) => ({ value: c, label: String(c) })),
    { value: 'other', label: t('admin.ops.errorDetails.statusCodeOther') || 'Other' }
  ]
})

const ownerSelectOptions = computed(() => {
  return [
    { value: '', label: t('common.all') },
    { value: 'provider', label: t('admin.ops.errorDetails.owner.provider') || 'provider' },
    ...(props.errorType === 'upstream' ? [] : [
    { value: 'client', label: t('admin.ops.errorDetails.owner.client') || 'client' },
    { value: 'platform', label: t('admin.ops.errorDetails.owner.platform') || 'platform' }
    ])
  ]
})


const viewModeSelectOptions = computed(() => {
  return [
    { value: 'errors', label: t('admin.ops.errorDetails.viewErrors') || 'errors' },
    { value: 'excluded', label: t('admin.ops.errorDetails.viewExcluded') || 'excluded' },
    { value: 'all', label: t('common.all') }
  ]
})

const phaseSelectOptions = computed(() => {
  const options = [
    { value: '', label: t('common.all') },
    { value: 'request', label: t('admin.ops.errorDetails.phase.request') || 'request' },
    { value: 'auth', label: t('admin.ops.errorDetails.phase.auth') || 'auth' },
    { value: 'account_auth', label: t('admin.ops.errorDetails.phase.account_auth') || 'account_auth' },
    { value: 'routing', label: t('admin.ops.errorDetails.phase.routing') || 'routing' },
    { value: 'upstream', label: t('admin.ops.errorDetails.phase.upstream') || 'upstream' },
    { value: 'network', label: t('admin.ops.errorDetails.phase.network') || 'network' },
    { value: 'internal', label: t('admin.ops.errorDetails.phase.internal') || 'internal' }
  ]
  return props.errorType === 'upstream'
    ? options.filter(option => ['', 'upstream', 'account_auth'].includes(option.value))
    : options
})

function close() {
  emit('update:show', false)
}

const sortBy = ref('created_at')
const sortOrder = ref<'asc' | 'desc'>('desc')

function onSort(nextSortBy: string, nextSortOrder: 'asc' | 'desc') {
  sortBy.value = nextSortBy
  sortOrder.value = nextSortOrder
  page.value = 1
}

async function fetchErrorLogs() {
  if (!props.show) return

  const version = ++requestVersion
  loading.value = true
  loadError.value = false
  try {
    const params: Record<string, any> = {
      page: page.value,
      page_size: pageSize.value,
      view: viewMode.value,
      sort_by: sortBy.value,
      sort_order: sortOrder.value
    }
    Object.assign(params, buildOpsErrorTimeParams(props.timeRange, props.customStartTime, props.customEndTime))

    if (props.timeRange === 'custom') {
      if (props.customStartTime && props.customEndTime) {
        params.start_time = props.customStartTime
        params.end_time = props.customEndTime
        delete params.time_range
      } else {
        // Safety fallback: avoid sending time_range=custom (backend doesn't support it)
        params.time_range = '1h'
      }
    }

    const platform = String(props.platform || '').trim()
    if (platform) params.platform = platform
    if (groupId.value) params.group_id = groupId.value
    if (accountId.value) params.account_id = accountId.value
    if (userId.value) params.user_id = userId.value

    if (q.value.trim()) params.q = q.value.trim()
    if (statusCode.value === 'other') params.status_codes_other = '1'
    else if (typeof statusCode.value === 'number') params.status_codes = String(statusCode.value)

    const phaseVal = String(phase.value || '').trim()
    if (phaseVal) params.phase = phaseVal

    const ownerVal = String(errorOwner.value || '').trim()
    if (ownerVal) params.error_owner = ownerVal


    const res = props.errorType === 'upstream'
      ? await opsAPI.listUpstreamErrors(params)
      : await opsAPI.listRequestErrors(params)
    if (version !== requestVersion) return
    rows.value = res.items || []
    total.value = res.total || 0
  } catch (err) {
    if (version !== requestVersion) return
    console.error('[OpsErrorDetailsModal] Failed to fetch error logs', err)
    loadError.value = true
    rows.value = []
    total.value = 0
  } finally {
    if (version === requestVersion) loading.value = false
  }
}

function resetFilters() {
  q.value = ''
  groupId.value = props.groupId ?? null
  accountId.value = null
  userId.value = null
  statusCode.value = null
  phase.value = ''
  errorOwner.value = ''
  viewMode.value = 'errors'
  page.value = 1
}

watch(() => props.show, (open) => {
  if (!open) {
    ++requestVersion
    clearTimeout(searchTimeout)
    return
  }
  if (!props.resumeState) {
    pageSize.value = 10
    resetFilters()
    if (groupId.value && !entityOptions.value.group.some(option => option.value === groupId.value)) {
      entityOptions.value.group.push({ value: groupId.value, label: `#${groupId.value}` })
      const id = groupId.value
      void groupsAPI.getById(id).then(group => {
        const option = entityOptions.value.group.find(option => option.value === id)
        if (option) option.label = `${group.name} · #${id}`
      }).catch(() => {})
    }
    entityKinds.forEach(kind => void searchEntity(kind))
  }
  scheduleFetch()
}, { immediate: true })

watch(() => props.groupId, value => { groupId.value = value ?? null })

function scheduleFetch(delay = 0) {
  clearTimeout(searchTimeout)
  ++requestVersion
  if (!props.show) return
  loading.value = true
  searchTimeout = setTimeout(() => void fetchErrorLogs(), delay)
}

watch([q, groupId, accountId, userId, statusCode, phase, errorOwner, viewMode,
  () => props.timeRange, () => props.customStartTime, () => props.customEndTime,
  () => props.platform, () => props.errorType], (values, previous) => {
  page.value = 1
  scheduleFetch(values[0] !== previous[0] ? 350 : 0)
})
watch([page, sortBy, sortOrder], () => scheduleFetch())
watch(pageSize, () => {
  page.value = 1
  scheduleFetch()
})

onUnmounted(() => {
  ++requestVersion
  clearTimeout(searchTimeout)
  entityKinds.forEach(kind => ++entityRequests[kind])
})
</script>

<template>
  <BaseDialog :show="show" :title="modalTitle" width="full" @close="close">
    <div class="flex h-full min-h-0 flex-col">
      <!-- Filters -->
      <div class="mb-4 flex-shrink-0 border-b border-gray-200 pb-4 dark:border-dark-700">
        <div class="grid grid-cols-2 items-end gap-3 lg:grid-cols-4">
          <div class="compact-select lg:col-span-3">
            <label class="mb-1 block text-xs text-gray-500" for="ops-error-search">{{ t('common.search') }}</label>
            <div class="relative group">
              <div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3">
                <svg
                  class="h-3.5 w-3.5 text-gray-400 transition-colors group-focus-within:text-blue-500"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                </svg>
              </div>
              <input
                id="ops-error-search"
                v-model="q"
                type="text"
                class="w-full rounded-lg border-gray-200 bg-gray-50/50 py-1.5 pl-9 pr-3 text-xs font-medium text-gray-700 transition-all focus:border-blue-500 focus:bg-white focus:ring-2 focus:ring-blue-500/10 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-300 dark:focus:bg-dark-800"
                :placeholder="t('admin.ops.errorDetails.searchPlaceholder')"
              />
            </div>
          </div>

          <div class="flex items-center justify-end">
            <button type="button" class="rounded-lg bg-gray-100 px-3 py-1.5 text-xs font-semibold text-gray-700 transition-colors hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-300 dark:hover:bg-dark-600" @click="resetFilters">
              {{ t('common.reset') }}
            </button>
          </div>

          <div v-for="kind in entityKinds" :key="kind" class="compact-select">
            <label :for="`ops-error-${kind}`" class="mb-1 block text-xs text-gray-500">{{ t(`admin.ops.errorDetail.${kind}`) }}</label>
            <Select :id="`ops-error-${kind}`" :model-value="entityIds[kind].value"
              :aria-label="t(`admin.ops.errorDetail.${kind}`)"
              :options="entityOptions[kind]" remote clearable :loading="entityLoading[kind]"
              :placeholder="t('common.all')" :search-placeholder="t(`admin.ops.errorDetails.search${kind}`)"
              :empty-text="entityErrors[kind] ? t('admin.ops.errorDetails.loadFailed') : undefined"
              @search="searchEntity(kind, $event)" @update:model-value="entityIds[kind].value = $event as number | null" />
            <button v-if="entityErrors[kind]" type="button" class="mt-1 text-xs text-red-600" @click="searchEntity(kind)">{{ t('admin.ops.errorDetails.loadFailed') }}</button>
          </div>

          <div class="compact-select">
            <label for="ops-statusCode" class="mb-1 block text-xs text-gray-500">{{ t('admin.ops.errorDetail.status') }}</label>
            <Select id="ops-statusCode" :aria-label="t('admin.ops.errorDetail.status')" :model-value="statusCode" :options="statusCodeSelectOptions" @update:model-value="statusCode = $event as any" />
          </div>

          <div class="compact-select">
            <label for="ops-phase" class="mb-1 block text-xs text-gray-500">{{ t('admin.ops.errorDetail.phase') }}</label>
            <Select id="ops-phase" :aria-label="t('admin.ops.errorDetail.phase')" :model-value="phase" :options="phaseSelectOptions" @update:model-value="phase = String($event ?? '')" />
          </div>

          <div class="compact-select">
            <label for="ops-errorOwner" class="mb-1 block text-xs text-gray-500">{{ t('admin.ops.errorDetails.ownerLabel') }}</label>
            <Select id="ops-errorOwner" :aria-label="t('admin.ops.errorDetails.ownerLabel')" :model-value="errorOwner" :options="ownerSelectOptions" @update:model-value="errorOwner = String($event ?? '')" />
          </div>

          <div class="compact-select">
            <label for="ops-viewMode" class="mb-1 block text-xs text-gray-500">{{ t('admin.ops.errorDetails.viewLabel') }}</label>
            <Select id="ops-viewMode" :aria-label="t('admin.ops.errorDetails.viewLabel')" :model-value="viewMode" :options="viewModeSelectOptions" @update:model-value="viewMode = $event as any" />
          </div>


        </div>
      </div>

      <button v-if="loadError" type="button" role="alert" class="mb-3 text-sm text-red-600" @click="fetchErrorLogs">{{ t('admin.ops.errorDetails.loadFailed') }}</button>
      <!-- Body -->
      <div class="flex min-h-0 flex-1 flex-col">
        <div class="mb-2 flex-shrink-0 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.ops.errorDetails.total') }} {{ total }}
        </div>

          <OpsErrorLogTable
            class="min-h-0 flex-1"
            summary-first
            :rows="rows"
            :total="total"
            :loading="loading"
            :page="page"
            :page-size="pageSize"
            @openErrorDetail="emit('openErrorDetail', $event)"
            @sort="onSort"

            @update:page="page = $event"
            @update:pageSize="pageSize = $event"
          />

      </div>
    </div>
  </BaseDialog>
</template>

<style scoped>
.compact-select :deep(.select-trigger) {
  @apply py-1.5 px-3 text-xs rounded-lg;
}
</style>
