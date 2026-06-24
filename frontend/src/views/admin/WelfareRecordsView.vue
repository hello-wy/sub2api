<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <!-- Left: Search by Email -->
          <div class="flex-1 sm:max-w-64">
            <input
              v-model="searchQuery"
              type="text"
              :placeholder="t('admin.welfare.searchPlaceholder')"
              class="input"
              @input="handleSearch"
            />
          </div>

          <!-- Right: Refresh button -->
          <div class="flex flex-1 flex-wrap items-center justify-end gap-2">
            <button
              @click="loadRecords"
              :disabled="loading"
              class="btn btn-secondary"
              :title="t('common.refresh')"
            >
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
          </div>
        </div>
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
                v-if="row.status !== 'revoked'"
                @click="confirmRevoke(row)"
                class="flex flex-col items-center gap-0.5 rounded-lg p-1.5 text-gray-500 transition-colors hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20 dark:hover:text-red-400"
                :title="t('admin.welfare.action.revoke')"
              >
                <Icon name="trash" size="sm" />
              </button>
              <span v-else class="text-xs text-gray-400 dark:text-gray-500 px-1.5 py-1">
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
import type { WelfareRecord } from '@/api/admin/welfare'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const records = ref<WelfareRecord[]>([])
const searchQuery = ref('')
const selectedRecord = ref<WelfareRecord | null>(null)
const showRevokeConfirm = ref(false)

const pagination = reactive({
  page: 1,
  pageSize: 20,
  total: 0
})

const columns = [
  { key: 'user_email', label: t('admin.welfare.table.email'), sortable: false },
  { key: 'amount', label: t('admin.welfare.table.amount'), sortable: false },
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
      searchQuery.value.trim() || undefined
    )
    records.value = res.items || []
    pagination.total = res.total || 0
  } catch (err: unknown) {
    appStore.showError(t('common.unknownError'))
  } finally {
    loading.value = false
  }
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

function confirmRevoke(record: WelfareRecord) {
  selectedRecord.value = record
  showRevokeConfirm.value = true
}

async function executeRevoke() {
  if (!selectedRecord.value) return
  showRevokeConfirm.value = false
  const recordId = selectedRecord.value.id
  try {
    await adminAPI.welfare.revoke(recordId)
    appStore.showSuccess(t('admin.welfare.action.revokeSuccess'))
    loadRecords()
  } catch (err: unknown) {
    appStore.showError(t('common.unknownError'))
  }
}

function formatDateTime(val: string) {
  if (!val) return ''
  const date = new Date(val)
  return date.toLocaleString()
}

onMounted(() => {
  loadRecords()
})
</script>
