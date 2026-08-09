<template>
  <AppLayout>
    <TablePageLayout>
      <template #actions>
        <WelfareSummaryDashboard :summary="summary" />
      </template>

      <template #filters>
        <WelfareRecordsFilters
          v-model:search="searchQuery"
          v-model:type="benefitType"
          v-model:status="statusFilter"
          v-model:start-date="startDate"
          v-model:end-date="endDate"
          :loading="loading"
          @search-change="handleSearch"
          @type-change="onBenefitTypeChange"
          @status-change="onStatusChange"
          @date-change="onDateRangeChange"
          @refresh="loadRecords"
        />
      </template>

      <template #table>
        <DataTable
          :columns="columns"
          :data="records"
          :loading="loading"
          :server-side-sort="false"
        >
          <template #cell-user_email="{ value }">
            <span class="text-sm font-medium text-gray-900 dark:text-white">{{ value }}</span>
          </template>

          <template #cell-amount="{ value }">
            <span class="text-sm font-semibold text-primary-600 dark:text-primary-400">
              ${{ value.toFixed(2) }}
            </span>
          </template>

          <template #cell-remarks="{ value }">
            <span class="text-sm text-gray-600 dark:text-gray-300 font-mono">{{ value }}</span>
          </template>

          <template #cell-type="{ value }">
            <span
              :class="[
                'badge',
                value === 'checkin'
                  ? 'bg-blue-50 text-blue-700 dark:bg-blue-900/20 dark:text-blue-400'
                  : value === 'lottery'
                    ? 'bg-amber-50 text-amber-700 dark:bg-amber-900/20 dark:text-amber-300'
                    : 'bg-purple-50 text-purple-700 dark:bg-purple-900/20 dark:text-purple-400'
              ]"
            >
              {{ formatBenefitType(value) }}
            </span>
          </template>

          <template #cell-status="{ value }">
            <span
              :class="[
                'badge',
                value === 'revoked'
                  ? 'bg-red-50 text-red-700 dark:bg-red-900/20 dark:text-red-400'
                  : 'bg-green-50 text-green-700 dark:bg-green-900/20 dark:text-green-400'
              ]"
            >
              {{ value === 'revoked' ? t('admin.welfare.status.revoked') : t('admin.welfare.status.success') }}
            </span>
          </template>

          <template #cell-created_at="{ value }">
            <span class="text-sm text-gray-500 dark:text-dark-400">
              {{ formatDateTime(value) }}
            </span>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex items-center space-x-1">
              <button
                v-if="row.status !== 'revoked' && row.type !== 'lottery'"
                @click="confirmRevoke(row)"
                class="btn btn-sm btn-danger"
                :title="t('admin.welfare.action.revoke')"
              >
                {{ t('admin.welfare.action.revokeButton') }}
              </button>
              <span v-else-if="row.status === 'revoked'" class="text-xs text-gray-400 dark:text-gray-500 px-1.5 py-1">
                {{ t('admin.welfare.status.revoked') }}
              </span>
            </div>
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :page-size="pagination.pageSize"
          :total="pagination.total"
          @update:page="handlePageChange"
          @update:page-size="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>

    <!-- Revoke Confirmation Dialog -->
    <ConfirmDialog
      :show="showRevokeConfirm"
      :title="t('admin.welfare.action.revokeConfirmTitle')"
      :message="t('admin.welfare.action.revokeConfirmMessage', { amount: selectedRecord ? selectedRecord.amount.toFixed(2) : '0.00' })"
      :danger="true"
      @confirm="executeRevoke"
      @cancel="showRevokeConfirm = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { WelfareBenefitType, WelfareRecord, WelfareRecordStatus, WelfareSummary } from '@/api/admin/welfare'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import WelfareSummaryDashboard from '@/components/admin/welfare/WelfareSummaryDashboard.vue'
import WelfareRecordsFilters from '@/components/admin/welfare/WelfareRecordsFilters.vue'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
const appStore = useAppStore()
const DAY_MS = 24 * 60 * 60 * 1000

const loading = ref(false)
const records = ref<WelfareRecord[]>([])
const searchQuery = ref('')
const benefitType = ref<'' | WelfareBenefitType>('')
const statusFilter = ref<'' | WelfareRecordStatus>('')
const selectedRecord = ref<WelfareRecord | null>(null)
const showRevokeConfirm = ref(false)
const defaultRange = getLast24HoursRangeDates()
const startDate = ref(defaultRange.start)
const endDate = ref(defaultRange.end)

const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0
})

const summary = reactive<WelfareSummary>({
  total_count: 0,
  total_amount: 0,
  checkin_amount: 0,
  leaderboard_amount: 0,
  lottery_amount: 0
})

const columns = [
  { key: 'user_email', label: t('admin.welfare.table.email'), sortable: false },
  { key: 'amount', label: t('admin.welfare.table.amount'), sortable: false },
  { key: 'type', label: t('admin.welfare.table.type'), sortable: false },
  { key: 'remarks', label: t('admin.welfare.table.remarks'), sortable: false },
  { key: 'status', label: t('admin.welfare.table.status'), sortable: false },
  { key: 'created_at', label: t('admin.welfare.table.createdAt'), sortable: false },
  { key: 'actions', label: t('admin.welfare.table.actions'), sortable: false }
]

async function loadRecords() {
  loading.value = true
  try {
    const res = await adminAPI.welfare.list(
      pagination.page,
      pagination.pageSize,
      searchQuery.value.trim() || undefined,
      {
        startDate: startDate.value,
        endDate: endDate.value,
        type: benefitType.value || undefined,
        status: statusFilter.value || undefined
      }
    )
    records.value = res.items || []
    pagination.total = res.total || 0
    applySummary(res.summary)
  } catch (err: unknown) {
    appStore.showError(t('common.unknownError'))
  } finally {
    loading.value = false
  }
}

function applySummary(nextSummary?: WelfareSummary) {
  summary.total_count = nextSummary?.total_count || 0
  summary.total_amount = nextSummary?.total_amount || 0
  summary.checkin_amount = nextSummary?.checkin_amount || 0
  summary.leaderboard_amount = nextSummary?.leaderboard_amount || 0
  summary.lottery_amount = nextSummary?.lottery_amount || 0
}

let searchTimeout: ReturnType<typeof setTimeout>
function handleSearch() {
  clearTimeout(searchTimeout)
  searchTimeout = setTimeout(() => {
    pagination.page = 1
    loadRecords()
  }, 300)
}

function handlePageChange(newPage: number) {
  pagination.page = newPage
  loadRecords()
}

function handlePageSizeChange(newSize: number) {
  pagination.pageSize = newSize
  pagination.page = 1
  loadRecords()
}

function onDateRangeChange() {
  pagination.page = 1
  loadRecords()
}

function onBenefitTypeChange() {
  pagination.page = 1
  loadRecords()
}

function onStatusChange() {
  pagination.page = 1
  loadRecords()
}

function confirmRevoke(record: WelfareRecord) {
  selectedRecord.value = record
  showRevokeConfirm.value = true
}

async function executeRevoke() {
  if (!selectedRecord.value) return
  showRevokeConfirm.value = false
  const recordId = selectedRecord.value.id
  const recordType = selectedRecord.value.type
  try {
    await adminAPI.welfare.revoke(recordId, recordType)
    appStore.showSuccess(t('admin.welfare.action.revokeSuccess'))
    loadRecords()
  } catch (err: unknown) {
    appStore.showError(t('common.unknownError'))
  }
}

function formatBenefitType(value: string) {
  if (value === 'checkin') return t('admin.welfare.type.checkin')
  if (value === 'lottery') return t('admin.welfare.type.lottery')
  return t('admin.welfare.type.leaderboard')
}

function formatDateTime(val: string) {
  if (!val) return ''
  const date = new Date(val)
  return date.toLocaleString()
}

function formatLocalDate(date: Date): string {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
}

function getLast24HoursRangeDates(): { start: string; end: string } {
  const end = new Date()
  const start = new Date(end.getTime() - DAY_MS)
  return {
    start: formatLocalDate(start),
    end: formatLocalDate(end)
  }
}

onMounted(() => {
  loadRecords()
})
</script>
