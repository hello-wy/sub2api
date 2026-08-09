<template>
  <div class="flex flex-wrap items-center gap-3">
    <div class="flex-1 sm:max-w-64">
      <input
        :value="search"
        type="text"
        :placeholder="t('admin.welfare.searchPlaceholder')"
        class="input"
        @input="emitSearch"
      />
    </div>
    <Select
      :model-value="type"
      :options="typeOptions"
      class="w-40"
      @update:model-value="emitType"
      @change="$emit('type-change')"
    />
    <Select
      :model-value="status"
      :options="statusOptions"
      class="w-36"
      @update:model-value="emitStatus"
      @change="$emit('status-change')"
    />
    <div class="flex items-center gap-2">
      <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
        {{ t('admin.dashboard.timeRange') }}:
      </span>
      <DateRangePicker
        :start-date="startDate"
        :end-date="endDate"
        @update:start-date="$emit('update:startDate', $event)"
        @update:end-date="$emit('update:endDate', $event)"
        @change="$emit('date-change')"
      />
    </div>
    <div class="flex flex-1 flex-wrap items-center justify-end gap-2">
      <button
        @click="$emit('refresh')"
        :disabled="loading"
        class="btn btn-secondary"
        :title="t('common.refresh')"
      >
        <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import type { WelfareBenefitType, WelfareRecordStatus } from '@/api/admin/welfare'

defineProps<{
  search: string
  type: '' | WelfareBenefitType
  status: '' | WelfareRecordStatus
  startDate: string
  endDate: string
  loading: boolean
}>()

const emit = defineEmits<{
  'update:search': [value: string]
  'update:type': [value: '' | WelfareBenefitType]
  'update:status': [value: '' | WelfareRecordStatus]
  'update:startDate': [value: string]
  'update:endDate': [value: string]
  'search-change': []
  'type-change': []
  'status-change': []
  'date-change': []
  refresh: []
}>()

const { t } = useI18n()

const typeOptions = computed(() => [
  { value: '', label: t('admin.welfare.type.all') },
  { value: 'leaderboard', label: t('admin.welfare.type.leaderboard') },
  { value: 'checkin', label: t('admin.welfare.type.checkin') },
  { value: 'lottery', label: t('admin.welfare.type.lottery') }
])

const statusOptions = computed(() => [
  { value: '', label: t('admin.welfare.statusFilter.all') },
  { value: 'success', label: t('admin.welfare.status.success') },
  { value: 'revoked', label: t('admin.welfare.status.revoked') }
])

function emitSearch(event: Event) {
  emit('update:search', (event.target as HTMLInputElement).value)
  emit('search-change')
}

function emitType(value: string | number | boolean | null) {
  emit('update:type', String(value ?? '') as '' | WelfareBenefitType)
}

function emitStatus(value: string | number | boolean | null) {
  emit('update:status', String(value ?? '') as '' | WelfareRecordStatus)
}
</script>
